//go:build darwin
#import "darwin_dock_panel.h"
#import <Cocoa/Cocoa.h>

@interface OTDockPanel : NSPanel
@end
@implementation OTDockPanel
- (BOOL)canBecomeMainWindow {
  return NO;
}
- (BOOL)canBecomeKeyWindow {
  return NO;
}
@end
@interface OTDockPanelRecord : NSObject
@property NSWindow *host;
@property id hostDelegate;
@property NSView *content;
@property OTDockPanel *panel;
@property id closeObserver;
@property NSRect originalFrame;
@property NSAutoresizingMaskOptions originalAutoresizingMask;
@property NSMapTable<NSView *, NSValue *> *originalSubviewFrames;
@end
@implementation OTDockPanelRecord
@end
// All state below is accessed on AppKit's main thread. Tokens never repeat, so
// a delayed notification cannot touch a replacement host or panel.
static NSMutableDictionary<NSNumber *, OTDockPanelRecord *> *
panelRecords(void) {
  static NSMutableDictionary *records;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    records = [NSMutableDictionary new];
  });
  return records;
}
static void panelMain(void (^work)(void)) {
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
static NSHashTable<NSWindow *> *retiredPanelHosts(void) {
  static NSHashTable *hosts;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    hosts = [NSHashTable weakObjectsHashTable];
  });
  return hosts;
}

static void destroyPanel(uint64_t token, BOOL restore) {
  OTDockPanelRecord *r = panelRecords()[@(token)];
  if (!r)
    return;
  if (!restore)
    [retiredPanelHosts() addObject:r.host];
  [panelRecords() removeObjectForKey:@(token)];
  if (r.closeObserver) {
    [NSNotificationCenter.defaultCenter removeObserver:r.closeObserver];
    r.closeObserver = nil;
  }
  [r.panel orderOut:nil];
  r.panel.contentView = nil;
  // The host is strongly retained here, including during its WillClose
  // notification. Wails stores WKWebView in an assign property and accesses it
  // in -dealloc, so its content must remain owned by the host through teardown.
  r.host.contentView = r.content;
  r.content.frame = r.originalFrame;
  r.content.autoresizingMask = r.originalAutoresizingMask;
  for (NSView *view in r.originalSubviewFrames) {
    view.frame = [r.originalSubviewFrames objectForKey:view].rectValue;
  }
  if (restore)
    [r.host orderOut:nil];
  [r.panel close];
}
uint64_t ot_dock_panel_create(void *pointer) {
  __block uint64_t result = 0;
  panelMain(^{
    // Pointer comparison against live NSApp windows avoids dereferencing an
    // arbitrary or already-retired caller pointer.
    NSWindow *host = nil;
    for (NSWindow *candidate in NSApp.windows) {
      if ((__bridge void *)candidate == pointer) {
        host = candidate;
        break;
      }
    }
    if (!host || host.visible || !host.contentView ||
        [retiredPanelHosts() containsObject:host])
      return;
    for (OTDockPanelRecord *other in panelRecords().allValues) {
      if (other.host == host)
        return;
    }
    static uint64_t nextToken = 0;
    uint64_t token = ++nextToken;
    OTDockPanelRecord *r = [OTDockPanelRecord new];
    r.host = host;
    r.hostDelegate = host.delegate;
    r.content = host.contentView;
    r.originalFrame = r.content.frame;
    r.originalAutoresizingMask = r.content.autoresizingMask;
    r.originalSubviewFrames = [NSMapTable weakToStrongObjectsMapTable];
    for (NSView *view in r.content.subviews) {
      [r.originalSubviewFrames setObject:[NSValue valueWithRect:view.frame]
                                  forKey:view];
    }
    r.panel = [[OTDockPanel alloc]
        initWithContentRect:NSMakeRect(0, 0, 1, 1)
                  styleMask:NSWindowStyleMaskBorderless |
                            NSWindowStyleMaskNonactivatingPanel
                    backing:NSBackingStoreBuffered
                      defer:NO];
    r.panel.releasedWhenClosed = NO;
    r.panel.level = NSFloatingWindowLevel;
    r.panel.floatingPanel = YES;
    r.panel.hidesOnDeactivate = NO;
    r.panel.becomesKeyOnlyIfNeeded = YES;
    r.panel.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces |
                                 NSWindowCollectionBehaviorFullScreenAuxiliary;
    r.panel.movableByWindowBackground = NO;
    r.panel.opaque = NO;
    r.panel.backgroundColor = NSColor.clearColor;
    r.panel.hasShadow = NO;
    host.contentView = [[NSView alloc] initWithFrame:r.originalFrame];
    r.panel.contentView = r.content;
    r.content.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
    r.closeObserver = [NSNotificationCenter.defaultCenter
        addObserverForName:NSWindowWillCloseNotification
                    object:host
                     queue:nil
                usingBlock:^(NSNotification *note) {
                  destroyPanel(token, NO);
                }];
    panelRecords()[@(token)] = r;
    result = token;
  });
  return result;
}
int ot_dock_panel_show(uint64_t token, double x, double y, double w, double h) {
  __block int ok = 0;
  panelMain(^{
    OTDockPanelRecord *r = panelRecords()[@(token)];
    if (!r)
      return;
    CGFloat top = NSMaxY(NSScreen.screens.firstObject.frame);
    NSRect frame = NSMakeRect(x, top - y - h, w, h);
    [r.panel setFrame:frame display:YES];
    NSRect contentFrame = NSMakeRect(0, 0, w, h);
    r.content.frame = contentFrame;
    for (NSView *view in r.content.subviews) {
      view.frame = r.content.bounds;
    }
    [r.host orderOut:nil];
    [r.panel orderFrontRegardless];
    ok = 1;
  });
  return ok;
}
int ot_dock_panel_hide(uint64_t token) {
  __block int ok = 0;
  panelMain(^{
    OTDockPanelRecord *r = panelRecords()[@(token)];
    if (r) {
      [r.panel orderOut:nil];
      ok = 1;
    }
  });
  return ok;
}
int ot_dock_panel_close(uint64_t token) {
  panelMain(^{
    destroyPanel(token, YES);
  });
  return 1;
}
