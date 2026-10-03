//go:build darwin

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>
#import <unistd.h>
#import "darwin_apps.h"

// A command-line `go test` process has no AppKit main-loop to service a
// synchronous dispatch. Production has NSApp and uses the main queue; the
// no-NSApp branch is limited to NSWorkspace's thread-safe process snapshot.
static void OTAppsOnMain(void (^work)(void)) {
  if ([NSThread isMainThread] || NSApp == nil) {
    work();
  } else {
    dispatch_sync(dispatch_get_main_queue(), work);
  }
}

char *ot_list_apps_json(void) {
  @autoreleasepool {
    __block NSData *data = nil;
    OTAppsOnMain(^{
      NSMutableArray *records = [NSMutableArray array];
      for (NSRunningApplication *app in NSWorkspace.sharedWorkspace.runningApplications) {
        [records addObject:@{
          @"pid": @(app.processIdentifier),
          @"name": app.localizedName ?: @"",
          @"bundleId": app.bundleIdentifier ?: @"",
          @"hidden": @(app.hidden),
          @"terminated": @(app.terminated),
          @"policy": @(app.activationPolicy)
        }];
      }
      data = [NSJSONSerialization dataWithJSONObject:records options:0 error:nil];
    });
    if (!data) return strdup("[]");
    NSString *json = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
    return strdup(json.UTF8String ?: "[]");
  }
}

int ot_activate_app(int pid) {
  if (pid <= 0 || pid == getpid()) return -1;
  @autoreleasepool {
    __block int result = -1;
    OTAppsOnMain(^{
      NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
      if (!app || app.terminated || app.activationPolicy != NSApplicationActivationPolicyRegular || !app.bundleURL || !app.bundleIdentifier.length) {
        result = -1;
        return;
      }
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
      result = [app activateWithOptions:NSApplicationActivateIgnoringOtherApps] ? 1 : 0;
#pragma clang diagnostic pop
    });
    return result;
  }
}

int ot_frontmost_app_pid(void) {
  @autoreleasepool {
    __block int pid = 0;
    OTAppsOnMain(^{ pid = NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier; });
    return pid;
  }
}
