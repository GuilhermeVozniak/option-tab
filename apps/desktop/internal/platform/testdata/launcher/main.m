#include "../../darwin_dock_panel.m"
#include "../../darwin_launcher.m"
#include "../../darwin_media_panel.m"
#import <Cocoa/Cocoa.h>
int goLauncherFinalGuard(uintptr_t token) { abort(); }
char *ot_lock_read_container(void) { abort(); }
@interface FakeLauncherPanel : NSObject
@property(getter=isVisible) BOOL visible;
@property(getter=isOnActiveSpace) BOOL onActiveSpace;
@property(getter=isMiniaturized) BOOL miniaturized;
@property NSWindowOcclusionState occlusionState;
@property NSView *contentView;
@property NSAppearance *appearance;
@property BOOL closed;
@end
@implementation FakeLauncherPanel
- (void)orderOut:(id)sender {
  self.visible = NO;
}
- (void)close {
  self.closed = YES;
}
@end
int main(void) {
  @autoreleasepool {
    NSDictionary *current =
        @{@"id64" : @6,
          @"ManagedSpaceID" : @6,
          @"type" : @0};
    NSDictionary * (^record)(id) = ^NSDictionary *(id entry) {
      return @{
        @"Display Identifier" : @"display",
        @"Current Space" : current,
        @"Spaces" : @[ entry ]
      };
    };
    NSCAssert(launcherSpaceRecords(@[ record(current) ], @"display", 6) == 6,
              @"valid exact Space refused");
    NSCAssert(launcherSpaceRecords(
                  @[ record(
                      @{@"id64" : @6,
                        @"ManagedSpaceID" : @6,
                        @"type" : @NO}) ],
                  @"display", 6) == 0,
              @"boolean corroborating Space accepted as ordinary");
    __block uint64_t start = 100;
    NSDictionary * (^dockProcess)(void) = ^NSDictionary * {
      return @{@"PID" : @42, @"StartSeconds" : @(start), @"StartMicros" : @1};
    };
    NSDictionary *bound =
        launcherDockBoundSnapshot(dockProcess, ^NSDictionary * {
          start = 101;
          return @{@"pid" : @42, @"reason" : @""};
        });
    NSCAssert(!bound, @"Dock restart during AX read retained stale bounds");
    bound = launcherDockBoundSnapshot(dockProcess, ^NSDictionary * {
      return @{@"pid" : @42, @"reason" : @""};
    });
    NSCAssert([bound[@"Process"] isEqual:dockProcess()],
              @"stable Dock snapshot lost identity");
    NSCAssert(!launcherDockBoundSnapshot(dockProcess,
                                         ^NSDictionary * {
                                           return @{@"pid" : @43};
                                         }),
              @"other AX process admitted");

    __block BOOL identity = YES, space = YES, visible = YES;
    __block int dispatches = 0, guards = 0;
    BOOL (^id)(void) = ^BOOL {
      return identity;
    };
    BOOL (^sp)(void) = ^BOOL {
      return space;
    };
    BOOL (^panel)(void) = ^BOOL {
      return visible;
    };
    BOOL (^send)(void) = ^BOOL {
      dispatches++;
      return YES;
    };
    BOOL (^guard)(void) = ^BOOL {
      guards++;
      return YES;
    };
    NSCAssert(launcherActivatePrepared(id, sp, panel, guard, send) &&
                  dispatches == 1,
              @"valid prepared target refused");
    for (int mode = 0; mode < 4; mode++) {
      identity = space = visible = YES;
      dispatches = guards = 0;
      BOOL result = launcherActivatePrepared(
          id, sp, panel,
          ^BOOL {
            guards++;
            if (mode == 0)
              identity = NO;
            if (mode == 1)
              space = NO;
            if (mode == 2)
              visible = NO;
            return mode != 3;
          },
          send);
      NSCAssert(!result && dispatches == 0,
                @"stale identity/space/host/cancel admitted after preparation");
    }
    identity = space = visible = YES;
    dispatches = 0;
    NSCAssert(!launcherActivatePrepared(id, sp, panel, guard,
                                        ^BOOL {
                                          return NO;
                                        }),
              @"native refusal lost");
    FakeLauncherPanel *host = [FakeLauncherPanel new];
    host.visible = YES;
    host.onActiveSpace = YES;
    host.occlusionState = NSWindowOcclusionStateVisible;
    OTDockPanelRecord *lease = [OTDockPanelRecord new];
    lease.launcherDisplay = @"display";
    lease.launcherSpace = 6;
    lease.panel = (OTDockPanel *)host;
    panelRecords()[@77] = lease;
    NSCAssert(ot_launcher_panel_visible(77, "display") &&
                  ot_launcher_panel_space(77) == 6,
              @"current exact host refused");
    NSCAssert(!ot_launcher_panel_visible(78, "display") &&
                  !ot_launcher_panel_visible(77, "other"),
              @"wrong host/display admitted");
    for (int mode = 0; mode < 4; mode++) {
      host.visible = YES;
      host.onActiveSpace = YES;
      host.miniaturized = NO;
      host.occlusionState = NSWindowOcclusionStateVisible;
      if (mode == 0)
        host.visible = NO;
      if (mode == 1)
        host.onActiveSpace = NO;
      if (mode == 2)
        host.miniaturized = YES;
      if (mode == 3)
        host.occlusionState = 0;
      NSCAssert(!ot_launcher_panel_visible(77, "display"),
                @"noninteractive physical host admitted");
    }
    host.visible = YES;
    host.onActiveSpace = YES;
    host.miniaturized = NO;
    host.occlusionState = NSWindowOcclusionStateVisible;
    identity = space = YES;
    dispatches = 0;
    BOOL accepted = launcherActivatePrepared(
        id, sp,
        ^BOOL {
          return ot_launcher_panel_visible(77, "display") &&
                 ot_launcher_panel_space(77) == 6;
        },
        ^BOOL {
          lease.launcherSpace = 7;
          return YES;
        },
        send);
    NSCAssert(!accepted && dispatches == 0,
              @"changed shown-Space lease dispatched");
    [panelRecords() removeObjectForKey:@77];
    NSCAssert(!ot_launcher_panel_visible(77, "display"),
              @"retired token admitted");
    FakeLauncherPanel *styleHost = [FakeLauncherPanel new],
                      *stylePanel = [FakeLauncherPanel new];
    NSView *original = [[NSView alloc] initWithFrame:NSMakeRect(3, 4, 300, 80)];
    NSView *child = [[NSView alloc] initWithFrame:NSMakeRect(5, 6, 20, 30)];
    [original addSubview:child];
    OTDockPanelRecord *styled = [OTDockPanelRecord new];
    styled.host = (NSWindow *)styleHost;
    styled.panel = (OTDockPanel *)stylePanel;
    styled.content = original;
    styled.originalFrame = original.frame;
    styled.originalAutoresizingMask = original.autoresizingMask;
    styled.originalSubviewFrames = [NSMapTable weakToStrongObjectsMapTable];
    [styled.originalSubviewFrames setObject:[NSValue valueWithRect:child.frame]
                                     forKey:child];
    styled.launcherDisplay = @"display";
    stylePanel.contentView = original;
    panelRecords()[@88] = styled;
    NSCAssert(ot_launcher_panel_style(88, "system", "dark", 18),
              @"valid launcher material refused");
    NSCAssert(styled.launcherEffect &&
                  styled.launcherRoot.subviews.firstObject ==
                      styled.launcherEffect &&
                  styled.launcherRoot.subviews.lastObject == original &&
                  styled.launcherRoot.layer.cornerRadius == 18 &&
                  styled.launcherRoot.layer.masksToBounds &&
                  [styled.launcherRoot.appearance.name
                      isEqual:NSAppearanceNameDarkAqua],
              @"effect/theme/clipping not behind original content");

    NSCAssert(!ot_launcher_panel_style(88, "remote", "dark", 18) &&
                  !ot_launcher_panel_style(88, "solid", "dark", 29),
              @"untrusted material/radius accepted");
    styled.launcherDisplay = nil;
    NSCAssert(!ot_launcher_panel_style(88, "solid", "light", 0),
              @"non-launcher styled");
    styled.launcherDisplay = @"display";
    NSCAssert(ot_launcher_panel_style(88, "solid", "system", 0),
              @"solid reset refused");
    NSCAssert(!styled.launcherEffect &&
                  styled.launcherRoot.subviews.count == 1 &&
                  !styled.launcherRoot.appearance &&
                  styled.launcherRoot.layer.cornerRadius == 0,
              @"solid/system reset retained effect/theme");

    destroyPanel(88, YES);
    NSCAssert(styleHost.contentView == original &&
                  NSEqualRects(original.frame, NSMakeRect(3, 4, 300, 80)) &&
                  NSEqualRects(child.frame, NSMakeRect(5, 6, 20, 30)) &&
                  stylePanel.closed,
              @"exact original content/frame restoration failed");
    NSCAssert(!ot_launcher_panel_style(88, "system", "dark", 18),
              @"retired token styled");
    NSRect usable = launcherSafeFrame(NSMakeRect(-1200, -200, 1200, 900),
                                      NSMakeRect(-1200, -200, 1200, 878),
                                      NSEdgeInsetsMake(40, 10, 20, 30));
    NSCAssert(NSEqualRects(usable, NSMakeRect(-1190, -180, 1160, 840)),
              @"safe area intersection must preserve negative coordinates and "
              @"top notch");
    NSDictionary *converted = launcherRect(usable, 700);
    NSCAssert([converted[@"X"] doubleValue] == -1190 &&
                  [converted[@"Y"] doubleValue] == 40 &&
                  [converted[@"W"] doubleValue] == 1160,
              @"safe area top-left logical conversion wrong");
    NSCAssert(NSEqualRects(launcherSafeFrame(NSMakeRect(0, 0, 100, 100),
                                             NSMakeRect(0, 0, 100, 90),
                                             NSEdgeInsetsMake(110, 0, 0, 0)),
                           NSZeroRect),
              @"impossible safe area accepted");
    NSDictionary *focus = @{
      @"Process" : @{@"PID" : @42, @"StartSeconds" : @100, @"StartMicros" : @1},
      @"BundleID" : @"com.example.one"
    };
    NSCAssert(launcherFocusEvidence(focus, focus),
              @"stable foreground evidence refused");
    for (NSDictionary *changed in @[
           @{
             @"Process" :
                 @{@"PID" : @43, @"StartSeconds" : @100, @"StartMicros" : @1},
             @"BundleID" : @"com.example.one"
           },
           @{
             @"Process" :
                 @{@"PID" : @42, @"StartSeconds" : @101, @"StartMicros" : @1},
             @"BundleID" : @"com.example.one"
           },
           @{
             @"Process" :
                 @{@"PID" : @42, @"StartSeconds" : @100, @"StartMicros" : @1},
             @"BundleID" : @"com.example.two"
           }
         ]) {
      NSCAssert(!launcherFocusEvidence(focus, changed),
                @"changed PID/start/bundle admitted as known focus");
    }
    NSCAssert(!launcherFocusEvidence(focus, nil) &&
                  !launcherFocusEvidence(nil, focus),
              @"unknown focus admitted");
    NSCAssert(launcherNotificationMatters(
                  NSWorkspaceDidActivateApplicationNotification,
                  @"com.example.foreground"),
              @"activation did not dirty observation");
    NSCAssert(!launcherNotificationMatters(
                  NSWorkspaceDidLaunchApplicationNotification,
                  @"com.example.background"),
              @"background launch dirtied focus");
    NSCAssert(
        launcherNotificationMatters(
            NSWorkspaceDidTerminateApplicationNotification, @"com.apple.dock"),
        @"Dock lifecycle dirtiness lost");
    printf("PASS native launcher final preparation guard, process/Space/host "
           "invalidation and native refusal; no GUI or activation\n");
  }
  return 0;
}
