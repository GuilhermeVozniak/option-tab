#import <Cocoa/Cocoa.h>
static BOOL admission = YES;
static NSMutableArray *mainQueue;
static void (^beforeGuard)(void);
static int opened;
static id selectedProcess;
static BOOL refuseTerminate, holdTermination;
static int terminateCount;
#define OT_ITEM_PROCESS_APP(pid) (selectedProcess)
#define OT_ITEM_WORK(block) block()
#define OT_ITEM_RELAUNCH_SECONDS 0
static NSArray *running;
static int processReads;
static BOOL changeProcess;
#define OT_ITEM_RUNNING(bundle) (running)
#define OT_ITEM_PROCESS_INFO(pid, info)                                        \
  ({                                                                           \
    processReads++;                                                            \
    (info)->pbi_start_tvsec = (changeProcess && processReads > 1) ? 2 : 1;     \
    (info)->pbi_start_tvusec = 0;                                              \
    YES;                                                                       \
  })
@interface FixtureRunning : NSObject
@property(getter=isTerminated) BOOL terminated;
@property int processIdentifier;
@property NSString *bundleIdentifier;
@property NSURL *bundleURL;
@end
@implementation FixtureRunning
- (BOOL)terminate {
  terminateCount++;
  if (refuseTerminate)
    return NO;
  if (!holdTermination)
    self.terminated = YES;
  return YES;
}
@end

#define OT_ITEM_MAIN(block) [mainQueue addObject:[block copy]]
#define OT_ITEM_PANEL()                                                        \
  ({                                                                           \
    abort();                                                                   \
    (NSOpenPanel *)nil;                                                        \
  })
#define OT_ITEM_OPEN_APP(url, config, done)                                    \
  do {                                                                         \
    opened++;                                                                  \
    done(nil, nil);                                                            \
  } while (0)
#define OT_ITEM_OPEN_URL(url, config, done)                                    \
  do {                                                                         \
    opened++;                                                                  \
    done(nil, nil);                                                            \
  } while (0)
#define OT_ITEM_CURRENT(t) (admission)
#define OT_ITEM_PANEL_CURRENT(t, d) (YES)
#define OT_ITEM_BEFORE_GUARD()                                                 \
  do {                                                                         \
    if (beforeGuard)                                                           \
      beforeGuard();                                                           \
  } while (0)
#include "../../darwin_launcher_items.m"
static void require(BOOL value, NSString *message) {
  if (!value) {
    fprintf(stderr, "%s\n", message.UTF8String);
    exit(1);
  }
}
int main() {
  @autoreleasepool {
    mainQueue = [NSMutableArray new];
    void *job = ot_launcher_item_choose("file", 1);
    admission = NO;
    ((void (^)(void))mainQueue.firstObject)();
    [mainQueue removeAllObjects];
    char *reply = ot_launcher_item_poll(job);
    require(reply && strstr(reply, "cancelled"),
            @"queued canceled chooser created panel");
    free(reply);
    ot_launcher_item_release(job);
    admission = YES;
    NSString *dir = [NSTemporaryDirectory()
        stringByAppendingPathComponent:NSUUID.UUID.UUIDString];
    require([NSFileManager.defaultManager createDirectoryAtPath:dir
                                    withIntermediateDirectories:NO
                                                     attributes:nil
                                                          error:nil],
            @"mkdir");
    @try {
      NSString *path = [dir stringByAppendingPathComponent:@"owned.txt"];
      require([@"fixture" writeToFile:path
                           atomically:YES
                             encoding:NSUTF8StringEncoding
                                error:nil],
              @"write");
      NSDictionary *record =
          itemSelect([NSURL fileURLWithPath:path], @"file", 1);
      require(!record[@"error"] && [record[@"bookmark"] length] > 0,
              @"selection");
      OTLauncherItemScope *scope = itemResolve(record);
      require(scope != nil && itemSame(scope), @"resolve");
      NSString *appPath = [dir stringByAppendingPathComponent:@"Owned.app"];
      [NSFileManager.defaultManager
                createDirectoryAtPath:
                    [appPath stringByAppendingPathComponent:@"Contents"]
          withIntermediateDirectories:YES
                           attributes:nil
                                error:nil];
      [@{
        @"CFBundleIdentifier" : @"org.optiontab.fixture",
        @"CFBundlePackageType" : @"APPL",
        @"CFBundleDisplayName" : @"Fixture Editor"
      } writeToFile:[appPath
                        stringByAppendingPathComponent:@"Contents/Info.plist"]
          atomically:YES];
      NSDictionary *appRecord =
          itemSelect([NSURL fileURLWithPath:appPath], @"app", 1);
      require([appRecord[@"label"] isEqual:@"Fixture Editor"],
              @"application intrinsic name includes .app");
      OTLauncherItemScope *appScope = itemResolve(appRecord);
      require(appScope != nil, @"app resolve");
      FixtureRunning *app = [FixtureRunning new];
      app.processIdentifier = 42;
      app.bundleIdentifier = @"org.optiontab.fixture";
      app.bundleURL = [NSURL fileURLWithPath:appPath];
      FixtureRunning *other = [FixtureRunning new];
      other.processIdentifier = 43;
      other.bundleIdentifier = app.bundleIdentifier;
      other.bundleURL = [NSURL
          fileURLWithPath:[dir stringByAppendingPathComponent:@"Other.app"]];
      running = @[ app, other ];
      processReads = 0;
      require([itemRunning(appScope)[@"pid"] intValue] == 42,
              @"exact app match");
      other.bundleURL = app.bundleURL;
      running = @[ app, other ];
      require(![itemRunning(appScope)[@"pid"] intValue],
              @"ambiguous app match");
      running = @[ app ];
      processReads = 0;
      changeProcess = YES;
      require(![itemRunning(appScope)[@"pid"] intValue], @"PID reuse match");
      changeProcess = NO;
      running = @[ app, other ];
      require([itemRunning(appScope)[@"state"] isEqual:@"ambiguous"],
              @"ambiguity looks stopped");
      running = @[ app ];
      opened = 0;
      job = ot_launcher_item_open((__bridge void *)appScope, "open", "fixture",
                                  1, 0, 0, 0, "", 1);
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeAllObjects];
      reply = ot_launcher_item_poll(job);
      require(reply && opened == 0 && strstr(reply, "changed"),
              @"stopped target retargeted appearing process");
      free(reply);
      ot_launcher_item_release(job);
      running = @[];
      selectedProcess = app;
      app.terminated = NO;
      processReads = 0;
      job = ot_launcher_item_open((__bridge void *)appScope, "relaunch",
                                  "fixture", 1, 42, 1, 0, "", 1);
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeObjectAtIndex:0];
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeAllObjects];
      reply = ot_launcher_item_poll(job);
      require(reply && terminateCount == 1 && opened == 1,
              @"graceful exact relaunch");
      free(reply);
      ot_launcher_item_release(job);
      opened = 0;
      app.terminated = NO;
      refuseTerminate = YES;
      job = ot_launcher_item_open((__bridge void *)appScope, "relaunch",
                                  "fixture", 1, 42, 1, 0, "", 1);
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeAllObjects];
      reply = ot_launcher_item_poll(job);
      require(reply && strstr(reply, "dispatchRefused") && opened == 0,
              @"termination refusal launched");
      free(reply);
      ot_launcher_item_release(job);
      refuseTerminate = NO;
      holdTermination = YES;
      job = ot_launcher_item_open((__bridge void *)appScope, "relaunch",
                                  "fixture", 1, 42, 1, 0, "", 1);
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeObjectAtIndex:0];
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeAllObjects];
      reply = ot_launcher_item_poll(job);
      require(reply && strstr(reply, "timeout") && opened == 0,
              @"termination timeout launched");
      free(reply);
      ot_launcher_item_release(job);
      holdTermination = NO;

      beforeGuard = ^{
        admission = NO;
      };
      job = ot_launcher_item_open((__bridge void *)scope, "open", "fixture", 1,
                                  0, 0, 0, "", 1);
      ((void (^)(void))mainQueue.firstObject)();
      [mainQueue removeAllObjects];
      reply = ot_launcher_item_poll(job);
      require(reply && opened == 0, @"canceled preparation dispatched");
      free(reply);
      ot_launcher_item_release(job);
      beforeGuard = nil;
      admission = YES;
      require([[NSFileManager defaultManager] removeItemAtPath:path error:nil],
              @"remove");
      require([@"replacement" writeToFile:path
                               atomically:YES
                                 encoding:NSUTF8StringEncoding
                                    error:nil],
              @"replace");
      require(!itemSame(scope), @"replacement accepted");
      require(itemResolve(record) == nil,
              @"fresh bookmark accepted replacement");
      admission = NO;
      require([itemSelect([NSURL fileURLWithPath:path], @"file", 1)[@"error"]
                  isEqual:@"cancelled"],
              @"cancel selection");
    } @finally {
      [NSFileManager.defaultManager removeItemAtPath:dir error:nil];
    }
    puts("launcher item native identity seams pass");
  }
  return 0;
}
