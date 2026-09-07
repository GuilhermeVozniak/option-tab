//go:build darwin

#import "darwin_dock_panel.h"
#import "darwin_media_panel.h"
#import <Cocoa/Cocoa.h>

@class OTDockPanel;
static BOOL handlePanelWheel(OTDockPanel *, NSEvent *);
@interface OTDockPanel : NSPanel
@property uint64_t panelToken;
@end
@implementation OTDockPanel
- (void)sendEvent:(NSEvent *)event {
  if (OTMediaHeaderEvent(self.panelToken,event)) return;
  if (event.type == NSEventTypeScrollWheel && handlePanelWheel(self, event))
    return;
  [super sendEvent:event];
}
- (BOOL)canBecomeMainWindow {
  return NO;
}
- (BOOL)canBecomeKeyWindow {
  return NO;
}
@end
@interface OTPanelWheelMailbox : NSObject {
@public
  OTDockPanelWheelEvent events[129];
  NSUInteger head, count;
}
@end
@implementation OTPanelWheelMailbox
@end

@interface OTDockPanelRecord : NSObject
@property OTPanelWheelMailbox *wheelMailbox;
@property NSArray<NSDictionary *> *wheelRegions;
@property uint64_t wheelSession, wheelRevision, wheelGesture, wheelValidGesture,
    wheelWindow;
@property int wheelApp;
@property BOOL wheelEnabled, wheelVisible, wheelEnded, wheelBypass;
@property double wheelLastTime;
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

// The AppKit thread is the sole owner of gesture state and these bounded
// queues. Draining marshals here briefly; no Go callback, AX or Wails runs in
// sendEvent.
typedef struct {
  double x, y, dx, dy, timestamp, unixTime;
  int precise, inverted, phase, momentum;
} OTPanelWheelInput;
static uint64_t wheelSequence, wheelGestureSequence;
static NSMutableDictionary<NSNumber *, OTPanelWheelMailbox *> *
retiredWheelMailboxes(void) {
  static NSMutableDictionary *mailboxes;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    mailboxes = [NSMutableDictionary new];
  });
  return mailboxes;
}
static OTDockPanelWheelEvent wheelEvent(OTDockPanelRecord *r,
                                        OTPanelWheelInput i) {
  double direction = i.inverted ? -1 : 1;
  return (OTDockPanelWheelEvent){.session = r.wheelSession,
                                 .revision = r.wheelRevision,
                                 .sequence = ++wheelSequence,
                                 .gesture =
                                     r.wheelGesture ?: r.wheelValidGesture,
                                 .window = r.wheelWindow,
                                 .app = r.wheelApp,
                                 .owned = 1,
                                 .precise = i.precise,
                                 .inverted = i.inverted,
                                 .phase = i.phase,
                                 .momentum = i.momentum,
                                 .unixTime = i.unixTime,
                                 .x = i.x,
                                 .y = i.y,
                                 .dx = i.dx * direction,
                                 .dy = i.dy * direction};
}
static void queueWheel(OTDockPanelRecord *r, OTDockPanelWheelEvent event) {
  OTPanelWheelMailbox *m = r.wheelMailbox;
  m->events[(m->head + m->count) % 129] = event;
  m->count++;
}
// Normal admission stops at 128, reserving the 129th slot for cancellation. No
// previously suppressed event is discarded, including during hide/destruction.
static void cancelWheel(OTDockPanelRecord *r, int reason) {
  if ((r.wheelGesture || r.wheelValidGesture) && r.wheelMailbox &&
      r.wheelMailbox->count < 129) {
    OTPanelWheelInput input = {.phase = 4,
                               .precise = 1,
                               .unixTime = NSDate.date.timeIntervalSince1970};
    OTDockPanelWheelEvent event = wheelEvent(r, input);
    event.reason = reason;
    queueWheel(r, event);
  }
  r.wheelGesture = 0;
  r.wheelValidGesture = 0;
  r.wheelEnded = YES;
  r.wheelBypass = YES;
}
static BOOL applyWheelPolicy(OTDockPanelRecord *r, uint64_t session,
                             uint64_t revision, BOOL enabled,
                             NSArray *regions) {
  if (!r || (enabled && (!session || !revision)) || regions.count > 256)
    return NO;
  if (!session && !enabled) {
    session = r.wheelSession;
    revision = r.wheelRevision;
  }
  if (session < r.wheelSession ||
      (session == r.wheelSession && revision < r.wheelRevision))
    return NO;
  NSMutableArray *copies = [NSMutableArray new];
  for (id item in regions) {
    if (![item isKindOfClass:NSDictionary.class])
      return NO;
    for (NSString *key in @[ @"window", @"app", @"x", @"y", @"w", @"h" ])
      if (![item[key] isKindOfClass:NSNumber.class])
        return NO;
    double x = [item[@"x"] doubleValue], y = [item[@"y"] doubleValue],
           w = [item[@"w"] doubleValue], h = [item[@"h"] doubleValue];
    if (![item[@"window"] unsignedLongLongValue] ||
        [item[@"app"] longLongValue] <= 0 || !isfinite(x) || !isfinite(y) ||
        !isfinite(w) || !isfinite(h) || w <= 0 || h <= 0)
      return NO;
    [copies addObject:@{
      @"window" : item[@"window"],
      @"app" : item[@"app"],
      @"x" : @(x),
      @"y" : @(y),
      @"w" : @(w),
      @"h" : @(h)
    }];
  }
  if (r.wheelSession == session && r.wheelRevision == revision &&
      r.wheelEnabled == enabled && [r.wheelRegions isEqualToArray:copies])
    return YES;
  cancelWheel(r, 1);
  if (!r.wheelMailbox)
    r.wheelMailbox = [OTPanelWheelMailbox new];
  r.wheelSession = session;
  r.wheelRevision = revision;
  r.wheelEnabled = enabled;
  r.wheelRegions = [copies copy];
  r.wheelBypass = NO;
  return YES;
}
static void acknowledgeWheel(OTDockPanelRecord *r, uint64_t session,
                             uint64_t revision, uint64_t gesture) {
  if (r.wheelSession == session && r.wheelRevision == revision &&
      r.wheelValidGesture == gesture)
    r.wheelValidGesture =
        0; // Tail ownership remains until terminal/idle/new begin.
}
static BOOL admitWheel(OTDockPanelRecord *r, OTPanelWheelInput input) {
  if (!r || !r.wheelEnabled || !r.wheelVisible || !input.precise ||
      !isfinite(input.x) || !isfinite(input.y) || !isfinite(input.dx) ||
      !isfinite(input.dy))
    return NO;
  if (r.wheelEnded && input.timestamp - r.wheelLastTime > 0.25)
    r.wheelGesture = 0;
  if (input.momentum) {
    if (!r.wheelGesture || r.wheelBypass)
      return NO;
  } else if (input.phase == 1) {
    // A pending action token cannot be displaced by a new suppressed gesture.
    if (r.wheelValidGesture || (r.wheelGesture && !r.wheelEnded)) {
      r.wheelBypass = YES;
      return NO;
    }
    r.wheelGesture = 0;
    r.wheelBypass = NO;
  } else if (r.wheelBypass)
    return NO;
  if (!r.wheelGesture) {
    if (input.momentum || (input.phase != 1 && input.phase != 2) ||
        r.wheelValidGesture)
      return NO;
    NSDictionary *target = nil;
    for (NSDictionary *region in r.wheelRegions) {
      NSRect bounds =
          NSMakeRect([region[@"x"] doubleValue], [region[@"y"] doubleValue],
                     [region[@"w"] doubleValue], [region[@"h"] doubleValue]);
      if (NSPointInRect(NSMakePoint(input.x, input.y), bounds)) {
        target = region;
        break;
      }
    }
    if (!target || r.wheelMailbox->count >= 128) {
      r.wheelBypass = YES;
      return NO;
    }
    r.wheelGesture = ++wheelGestureSequence;
    r.wheelValidGesture = r.wheelGesture;
    r.wheelWindow = [target[@"window"] unsignedLongLongValue];
    r.wheelApp = [target[@"app"] intValue];
    r.wheelEnded = NO;
  }
  if (r.wheelMailbox->count >= 128) {
    cancelWheel(r, 4);
    return NO;
  }
  queueWheel(r, wheelEvent(r, input));
  r.wheelLastTime = input.timestamp;
  if (input.phase == 4 || input.momentum == 4) {
    r.wheelValidGesture = 0;
    r.wheelGesture = 0;
    r.wheelBypass = YES;
  } else if (input.momentum == 3) {
    r.wheelGesture = 0;
    r.wheelEnded = YES;
  } else if (input.phase == 3)
    r.wheelEnded = YES;
  return YES;
}
static int wheelPhase(NSEventPhase phase) {
  if (phase & NSEventPhaseCancelled)
    return 4;
  if (phase & NSEventPhaseEnded)
    return 3;
  if (phase & NSEventPhaseBegan)
    return 1;
  if (phase & (NSEventPhaseChanged | NSEventPhaseStationary))
    return 2;
  return 0;
}
static BOOL handlePanelWheel(OTDockPanel *panel, NSEvent *event) {
  OTDockPanelRecord *r = panelRecords()[@(panel.panelToken)];
  if (!r || !r.wheelEnabled || !event.hasPreciseScrollingDeltas)
    return NO;
  NSPoint point = [r.content convertPoint:event.locationInWindow fromView:nil];
  point.x -= NSMinX(r.content.bounds);
  point.y = r.content.flipped ? point.y - NSMinY(r.content.bounds)
                              : NSMaxY(r.content.bounds) - point.y;
  OTPanelWheelInput input = {
      .x = point.x,
      .y = point.y,
      .dx = event.scrollingDeltaX,
      .dy = event.scrollingDeltaY,
      .timestamp = event.timestamp,
      .unixTime = NSDate.date.timeIntervalSince1970 -
                  NSProcessInfo.processInfo.systemUptime + event.timestamp,
      .precise = event.hasPreciseScrollingDeltas,
      .inverted = event.isDirectionInvertedFromDevice,
      .phase = wheelPhase(event.phase),
      .momentum = wheelPhase(event.momentumPhase)};
  return admitWheel(r, input);
}

static void destroyPanel(uint64_t token, BOOL restore) {
  OTDockPanelRecord *r = panelRecords()[@(token)];
  if (!r)
    return;
  if (!restore)
    [retiredPanelHosts() addObject:r.host];
  OTMediaDestroy(token,!restore);
  cancelWheel(r, 3);
  r.wheelVisible = NO;
  if (r.wheelMailbox && r.wheelMailbox->count)
    retiredWheelMailboxes()[@(token)] = r.wheelMailbox;
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
    r.panel.panelToken = token;
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
    NSRect frame = OTMediaPrepare(token,NSMakeRect(x, top - y - h, w, h));

    [r.panel setFrame:frame display:YES];
    NSRect contentFrame = NSMakeRect(0, 0, frame.size.width, frame.size.height);
    r.content.frame = contentFrame;
    for (NSView *view in r.content.subviews) {
      view.frame = r.content.bounds;
    }
    [r.host orderOut:nil];
    [r.panel orderFrontRegardless];
    r.wheelVisible = YES;
    OTMediaShown(token);
    ok = 1;
  });
  return ok;
}
int ot_dock_panel_hide(uint64_t token) {
  __block int ok = 0;
  panelMain(^{
    OTDockPanelRecord *r = panelRecords()[@(token)];
    if (r) {
      OTMediaHide(token);
      cancelWheel(r, 2);
      r.wheelVisible = NO;
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

int ot_dock_panel_wheel_policy(uint64_t token, const char *json) {
  @autoreleasepool {
    NSData *data = json ? [[NSString stringWithUTF8String:json]
                              dataUsingEncoding:NSUTF8StringEncoding]
                        : nil;
    NSDictionary *policy = data ? [NSJSONSerialization JSONObjectWithData:data
                                                                  options:0
                                                                    error:nil]
                                : nil;
    if (![policy isKindOfClass:NSDictionary.class] ||
        ![policy[@"regions"] isKindOfClass:NSArray.class])
      return 0;
    for (NSString *key in @[ @"session", @"revision", @"enabled" ])
      if (![policy[key] isKindOfClass:NSNumber.class])
        return 0;
    __block int result = 0;
    panelMain(^{
      OTDockPanelRecord *r = panelRecords()[@(token)];
      if (!r) {
        result = -1;
        return;
      }
      result =
          applyWheelPolicy(r, [policy[@"session"] unsignedLongLongValue],
                           [policy[@"revision"] unsignedLongLongValue],
                           [policy[@"enabled"] boolValue], policy[@"regions"]);
    });
    return result;
  }
}
int ot_dock_panel_wheel_next(uint64_t token, OTDockPanelWheelEvent *event) {
  __block int result = 0;
  panelMain(^{
    OTDockPanelRecord *r = panelRecords()[@(token)];
    OTPanelWheelMailbox *m =
        r ? r.wheelMailbox : retiredWheelMailboxes()[@(token)];
    if (m && m->count) {
      *event = m->events[m->head];
      m->head = (m->head + 1) % 129;
      m->count--;
      result = 1;
    } else if (!r) {
      [retiredWheelMailboxes() removeObjectForKey:@(token)];
      result = -1;
    }
  });
  return result;
}
int ot_dock_panel_wheel_valid(uint64_t token, uint64_t session,
                              uint64_t revision, uint64_t gesture) {
  __block int result = 0;
  panelMain(^{
    OTDockPanelRecord *r = panelRecords()[@(token)];
    result = r && r.wheelVisible && r.wheelEnabled && gesture &&
             r.wheelSession == session && r.wheelRevision == revision &&
             r.wheelValidGesture == gesture;
  });
  return result;
}
void ot_dock_panel_wheel_complete(uint64_t token, uint64_t session,
                                  uint64_t revision, uint64_t gesture) {
  panelMain(^{
    acknowledgeWheel(panelRecords()[@(token)], session, revision, gesture);
  });
}

uint64_t ot_media_panel_create(void *host,uint64_t session) {
  if (!session) return 0;
  uint64_t token=ot_dock_panel_create(host);
  if (!token) return 0;
  __block BOOL attached=NO;
  panelMain(^{OTDockPanelRecord *r=panelRecords()[@(token)];if(r){OTMediaAttach(token,session,r.panel);attached=YES;}});
  return attached?token:0;
}
