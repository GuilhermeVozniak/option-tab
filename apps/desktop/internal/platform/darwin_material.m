//go:build darwin

#import "darwin_material.h"
#import "darwin_dock_panel.h"
#import "darwin_material_effect.h"
#import <QuartzCore/QuartzCore.h>
#include <math.h>
#ifndef OT_MATERIAL_CURRENT
extern int goMaterialAdmission(uintptr_t);
#define OT_MATERIAL_CURRENT(token) goMaterialAdmission(token)
#endif
#ifndef OT_MATERIAL_BEFORE_APPLY
#define OT_MATERIAL_BEFORE_APPLY() ((void)0)
#endif
#ifndef OT_MATERIAL_FADE
#define OT_MATERIAL_FADE(effect, ...)                                          \
  [NSAnimationContext                                                          \
      runAnimationGroup:^(NSAnimationContext * context) {                      \
        context.duration = .18;                                                \
        [[effect animator] setAlphaValue:0];                                   \
      }                                                                        \
      completionHandler:(__VA_ARGS__)]
#endif
@interface OTMaterialEffect : NSVisualEffectView
@end
@implementation OTMaterialEffect
- (NSView *)hitTest:(NSPoint)point {
  return nil;
}
@end
@interface OTMaterialRecord : NSObject
@property uint64_t token, preview, session, revision;
@property BOOL retired;
@property NSWindow *window, *host;
@property NSView *content;
@property OTMaterialEffect *effect;
@property NSAppearance *originalAppearance;
@property NSMutableArray *observers;
@end
@implementation OTMaterialRecord
@end
static NSMutableDictionary<NSNumber *, OTMaterialRecord *> *
materialRecords(void) {
  static NSMutableDictionary *records;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    records = [NSMutableDictionary new];
  });
  return records;
}
static void materialMain(void (^work)(void)) {
  if (NSThread.isMainThread) {
    @autoreleasepool {
      work();
    }
  } else
    dispatch_sync(dispatch_get_main_queue(), ^{
      @autoreleasepool {
        work();
      }
    });
}
static void materialRemove(uint64_t token) {
  OTMaterialRecord *r = materialRecords()[@(token)];
  if (!r)
    return;
  [materialRecords() removeObjectForKey:@(token)];
  for (id observer in r.observers)
    [NSNotificationCenter.defaultCenter removeObserver:observer];
  [r.observers removeAllObjects];
  [r.effect.layer removeAllAnimations];
  [r.effect removeFromSuperview];
  r.effect = nil;
  if (r.window.contentView == r.content)
    r.window.appearance = r.originalAppearance;
}
static BOOL materialHostCurrent(OTMaterialRecord *r) {
  if (!r || materialRecords()[@(r.token)] != r ||
      r.window.contentView != r.content)
    return NO;
  if (r.preview) {
    void *w = NULL, *c = NULL, *h = NULL;
    if (!ot_preview_material_target(r.preview, &w, &c, &h) ||
        w != (__bridge void *)r.window || c != (__bridge void *)r.content ||
        h != (__bridge void *)r.host)
      return NO;
  }
  return YES;
}
static uint64_t materialAttach(NSWindow *window, NSView *content,
                               NSWindow *host, uint64_t preview) {
  if (!window || !content || !host || window.contentView != content ||
      materialRecords().count >= 64)
    return 0;
  for (OTMaterialRecord *other in materialRecords().allValues) {
    if (other.window == window || other.host == host)
      return 0;
  }
  static uint64_t next = 0;
  if (next == UINT64_MAX)
    return 0;
  OTMaterialRecord *r = [OTMaterialRecord new];
  r.token = ++next;
  r.preview = preview;
  r.window = window;
  r.content = content;
  r.host = host;
  r.originalAppearance = window.appearance;
  r.observers = [NSMutableArray new];
  materialRecords()[@(r.token)] = r;
  uint64_t token = r.token;
  for (NSWindow *observed in (window == host ? @[ window ]
                                             : @[ window, host ])) {
    id observer = [NSNotificationCenter.defaultCenter
        addObserverForName:NSWindowWillCloseNotification
                    object:observed
                     queue:nil
                usingBlock:^(NSNotification *note) {
                  materialMain(^{
                    materialRemove(token);
                  });
                }];
    [r.observers addObject:observer];
  }
  return token;
}
uint64_t ot_material_preview(uint64_t preview) {
  __block uint64_t token = 0;
  materialMain(^{
    void *w = NULL, *c = NULL, *h = NULL;
    if (ot_preview_material_target(preview, &w, &c, &h))
      token = materialAttach((__bridge NSWindow *)w, (__bridge NSView *)c,
                             (__bridge NSWindow *)h, preview);
  });
  return token;
}
uint64_t ot_material_overlay(void *pointer) {
  if (!pointer)
    return 0;
  __block uint64_t token = 0;
  materialMain(^{
    if (ot_panel_material_host_owned(pointer))
      return;
    for (NSWindow *window in NSApp.windows) {
      if ((__bridge void *)window == pointer) {
        token = materialAttach(window, window.contentView, window, 0);
        return;
      }
    }
  });
  return token;
}
static BOOL materialScopeCurrent(OTMaterialRecord *r, uint64_t session,
                                 uint64_t revision, BOOL retire) {
  return r && session && revision && session >= r.session &&
         revision >= r.revision &&
         (retire ||
          (revision > r.revision && !(r.retired && session == r.session)));
}
static BOOL materialRectangle(NSView *content, double x, double y, double width,
                              double height, NSRect *frame) {
  double values[] = {x, y, width, height};
  for (int i = 0; i < 4; i++)
    if (!isfinite(values[i]) || values[i] < 0 || values[i] > 65536)
      return NO;
  NSRect b = content.bounds;
  if (!isfinite(b.size.width) || !isfinite(b.size.height) || width <= 0 ||
      height <= 0 || x + width > b.size.width || y + height > b.size.height ||
      x + width > 65536 || y + height > 65536)
    return NO;
  *frame = NSMakeRect(b.origin.x + x,
                      b.origin.y +
                          (content.isFlipped ? y : b.size.height - y - height),
                      width, height);
  return YES;
}
int ot_material_apply(uint64_t token, uint64_t session, uint64_t revision,
                      int enabled, const char *rawTheme, int radius, double x,
                      double y, double width, double height,
                      uintptr_t admission) {
  if (!rawTheme || radius < 0 || radius > 64 ||
      (enabled != 0 && enabled != 1) || !admission)
    return 0;
  NSString *theme = [NSString stringWithUTF8String:rawTheme];
  if (![@[ @"system", @"light", @"dark" ] containsObject:theme])
    return 0;
  __block int ok = 0;
  materialMain(^{
    OTMaterialRecord *r = materialRecords()[@(token)];
    NSRect frame = NSZeroRect;
    if (!materialHostCurrent(r) ||
        !materialScopeCurrent(r, session, revision, NO) ||
        !OT_MATERIAL_CURRENT(admission) ||
        (enabled && !materialRectangle(r.content, x, y, width, height, &frame)))
      return;
    OTMaterialEffect *effect = r.effect;
    if (enabled && !effect) {
      effect = [[OTMaterialEffect alloc] initWithFrame:frame];
      if (!effect)
        return;
      OTConfigureMaterialEffect(effect);
      effect.autoresizingMask = NSViewNotSizable;
      effect.wantsLayer = YES;
      effect.layer.masksToBounds = YES;
    }
    OT_MATERIAL_BEFORE_APPLY();
    if (!materialHostCurrent(r) ||
        !materialScopeCurrent(r, session, revision, NO) ||
        !OT_MATERIAL_CURRENT(admission) ||
        (enabled && !materialRectangle(r.content, x, y, width, height, &frame)))
      return;
    r.session = session;
    r.revision = revision;
    r.retired = NO;
    r.window.appearance =
        [theme isEqual:@"system"]
            ? nil
            : [NSAppearance appearanceNamed:[theme isEqual:@"dark"]
                                                ? NSAppearanceNameDarkAqua
                                                : NSAppearanceNameAqua];
    if (enabled) {
      [effect.layer removeAllAnimations];
      effect.alphaValue = 1;
      effect.frame = frame;
      effect.layer.cornerRadius = MIN(radius, MIN(width, height) / 2);
      if (effect.superview != r.content)
        [r.content addSubview:effect positioned:NSWindowBelow relativeTo:nil];
      r.effect = effect;
    } else {
      [r.effect.layer removeAllAnimations];
      [r.effect removeFromSuperview];
      r.effect = nil;
    }
    ok = 1;
  });
  return ok;
}
int ot_material_retire(uint64_t token, uint64_t session, uint64_t revision,
                       int fade, uintptr_t admission) {
  if ((fade != 0 && fade != 180) || !admission)
    return 0;
  __block int ok = 0;
  materialMain(^{
    OTMaterialRecord *r = materialRecords()[@(token)];
    if (!materialHostCurrent(r) ||
        !materialScopeCurrent(r, session, revision, YES) ||
        !OT_MATERIAL_CURRENT(admission))
      return;
    r.session = session;
    r.revision = revision;
    r.retired = YES;
    ok = 1;
    if (!r.effect)
      return;
    if (fade == 0) {
      [r.effect.layer removeAllAnimations];
      [r.effect removeFromSuperview];
      r.effect = nil;
      return;
    }
    __weak OTMaterialRecord *weak = r;
    OT_MATERIAL_FADE(r.effect, ^{
      OTMaterialRecord *current = weak;
      if (current && materialRecords()[@(token)] == current &&
          current.session == session && current.revision == revision &&
          current.retired) {
        [current.effect removeFromSuperview];
        current.effect = nil;
      }
    });
  });
  return ok;
}
void ot_material_close(uint64_t token) {
  materialMain(^{
    materialRemove(token);
  });
}
