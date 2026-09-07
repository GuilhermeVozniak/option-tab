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
@end
@implementation FakeLauncherPanel
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
    printf("PASS native launcher final preparation guard, process/Space/host "
           "invalidation and native refusal; no GUI or activation\n");
  }
  return 0;
}
