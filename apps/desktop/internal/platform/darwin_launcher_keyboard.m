//go:build darwin

#import "darwin_launcher_keyboard.h"
#include <string.h>

@interface OTLauncherKeyboardOwner : NSObject {
@public
  OTLauncherKeyboardPolicy policy;
  __weak NSWindow *panel;
  id spaceObserver;
}
@end
@implementation OTLauncherKeyboardOwner
@end

static NSMutableDictionary<NSNumber *, OTLauncherKeyboardOwner *> *keyboardOwners(void) {
  static NSMutableDictionary *owners;
  static dispatch_once_t once;
  dispatch_once(&once, ^{ owners = [NSMutableDictionary new]; });
  return owners;
}
static void keyboardMain(void (^work)(void)) {
  if (NSThread.isMainThread)
    work();
  else
    dispatch_sync(dispatch_get_main_queue(), work);
}
static BOOL keyboardScope(OTLauncherKeyboardPolicy a, OTLauncherKeyboardPolicy b) {
  return a.epoch == b.epoch && a.session == b.session && a.revision == b.revision &&
         a.admission == b.admission && strcmp(a.display, b.display) == 0;
}
BOOL OTLauncherKeyboardCanKey(uint64_t token) {
  OTLauncherKeyboardOwner *o = keyboardOwners()[@(token)];
  return o && o->policy.enabled;
}
void OTLauncherKeyboardResigned(uint64_t token) {
  OTLauncherKeyboardOwner *o = keyboardOwners()[@(token)];
  if (!o)
    return;
  o->policy.enabled = 0;
  if (o->spaceObserver) {
    [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:o->spaceObserver];
    o->spaceObserver = nil;
  }
}
void OTLauncherKeyboardRetire(uint64_t token, BOOL closed) {
  OTLauncherKeyboardOwner *o = keyboardOwners()[@(token)];
  if (!o)
    return;
  BOOL enabled = o->policy.enabled;
  NSWindow *panel = o->panel;
  // Clear permission before resignKeyWindow's override reenters retirement.
  OTLauncherKeyboardResigned(token);
  if (closed)
    [keyboardOwners() removeObjectForKey:@(token)];
  if (enabled && panel.isKeyWindow)
    [panel resignKeyWindow];
}
BOOL OTLauncherKeyboardEvent(uint64_t token, NSEvent *event) {
  if (!OTLauncherKeyboardCanKey(token))
    return NO;
  if (event.type == NSEventTypeKeyDown && event.keyCode == 53) {
    OTLauncherKeyboardRetire(token, NO);
    return YES;
  }
  return NO;
}
int ot_launcher_keyboard_set(uint64_t token, OTLauncherKeyboardPolicy p, uintptr_t guard) {
  __block int ok = 0;
  keyboardMain(^{
    if (!token || !p.epoch || !p.session || !p.revision || !p.admission ||
        !memchr(p.display, 0, sizeof(p.display)) || !p.display[0])
      return;
    OTLauncherKeyboardOwner *o = keyboardOwners()[@(token)];
    if (o && (p.admission < o->policy.admission || p.epoch < o->policy.epoch ||
              (p.epoch == o->policy.epoch && p.session < o->policy.session) ||
              (p.epoch == o->policy.epoch && p.session == o->policy.session &&
               p.revision < o->policy.revision)))
      return;
    NSWindow *panel = nil;
    NSView *content = nil;
    if (p.enabled) {
      panel = OTLauncherKeyboardPanel(token, p.display);
      content = OTLauncherKeyboardContent(token);
      if (!panel || !content)
        return;
      if (o && p.admission == o->policy.admission) {
        if (!keyboardScope(o->policy, p) || !o->policy.enabled || !panel.isKeyWindow)
          return;
        ok = guard && ot_go_launcher_keyboard_current(guard);
        return;
      }
    } else if (o && p.admission == o->policy.admission && !keyboardScope(o->policy, p)) {
      return;
    }
    // Final bounded Go-only lifetime check after all native preparation.
    if (!guard || !ot_go_launcher_keyboard_current(guard))
      return;
    OTLauncherKeyboardRetire(token, NO);
    if (!o) {
      o = [OTLauncherKeyboardOwner new];
      keyboardOwners()[@(token)] = o;
    }
    o->policy = p;
    o->panel = panel;
    if (!p.enabled) {
      ok = 1;
      return;
    }
    [panel makeKeyWindow];
    if (!panel.isKeyWindow || ![panel makeFirstResponder:content]) {
      OTLauncherKeyboardRetire(token, NO);
      return;
    }
    o->spaceObserver = [NSWorkspace.sharedWorkspace.notificationCenter
        addObserverForName:NSWorkspaceActiveSpaceDidChangeNotification
                    object:nil
                     queue:NSOperationQueue.mainQueue
                usingBlock:^(NSNotification *note) { OTLauncherKeyboardRetire(token, NO); }];
    ok = 1;
  });
  return ok;
}
int ot_launcher_keyboard_valid(uint64_t token, OTLauncherKeyboardPolicy p) {
  __block int ok = 0;
  keyboardMain(^{
    OTLauncherKeyboardOwner *o = keyboardOwners()[@(token)];
    if (!o || !o->policy.enabled || !keyboardScope(o->policy, p))
      return;
    NSWindow *panel = OTLauncherKeyboardPanel(token, p.display);
    ok = panel && panel == o->panel && panel.isKeyWindow;
    if (!ok)
      OTLauncherKeyboardRetire(token, NO);
  });
  return ok;
}
