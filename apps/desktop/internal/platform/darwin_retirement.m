//go:build darwin
#import "darwin_retirement.h"
#import "darwin.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#include <libproc.h>
#include <sys/proc_info.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef, CGWindowID *);

static BOOL processStart(int pid, uint64_t *sec, uint64_t *usec) {
  struct proc_bsdinfo info = {0};
  if (pid <= 0 ||
      proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) != sizeof(info) ||
      !info.pbi_start_tvsec)
    return NO;
  *sec = info.pbi_start_tvsec;
  *usec = info.pbi_start_tvusec;
  return YES;
}

int ot_retirement_matches(uint32_t window, int pid, uint64_t sec, uint64_t usec) {
  uint64_t currentSec = 0, currentUsec = 0;
  return window && pid > 0 && sec && ot_window_pid(window) == pid &&
         processStart(pid, &currentSec, &currentUsec) &&
         currentSec == sec && currentUsec == usec;
}

int ot_retirement_identity(uint32_t window, int *pid, uint64_t *sec, uint64_t *usec) {
  *pid = ot_window_pid(window);
  *sec = 0;
  *usec = 0;
  return processStart(*pid, sec, usec) &&
         ot_retirement_matches(window, *pid, *sec, *usec);
}

char *ot_retirement_inventory_json(void) {
  @autoreleasepool {
    CFArrayRef info = CGWindowListCopyWindowInfo(kCGWindowListOptionAll,
                                               kCGNullWindowID);
    if (!info)
      return NULL;
    NSMutableArray *out = [NSMutableArray new];
    NSMutableDictionary *starts = [NSMutableDictionary new];
    for (NSDictionary *window in (__bridge NSArray *)info) {
      NSNumber *number = window[(__bridge NSString *)kCGWindowNumber];
      NSNumber *pid = window[(__bridge NSString *)kCGWindowOwnerPID];
      if (!number || !pid) {
        CFRelease(info);
        return NULL; // Never prune from an incomplete identity snapshot.
      }
      NSArray *start = starts[pid];
      if (!start) {
        uint64_t sec = 0, usec = 0;
        processStart(pid.intValue, &sec, &usec);
        start = @[@(sec), @(usec)];
        starts[pid] = start;
      }
      [out addObject:@{@"window": number, @"pid": pid,
                      @"startSec": start[0], @"startUsec": start[1]}];
    }
    CFRelease(info);
    NSData *json = [NSJSONSerialization dataWithJSONObject:out options:0 error:nil];
    if (!json)
      return NULL;
    return strdup([[NSString alloc] initWithData:json
                                       encoding:NSUTF8StringEncoding].UTF8String);
  }
}

int ot_retirement_reappeared(uint32_t window, int pid, uint64_t sec, uint64_t usec) {
  @autoreleasepool {
    if (!AXIsProcessTrusted() || !ot_retirement_matches(window, pid, sec, usec))
      return 0;
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementSetMessagingTimeout(app, 0.1);
    CFTypeRef value = NULL;
    BOOL found = NO;
    CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 0.3;
    if (AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &value) ==
            kAXErrorSuccess &&
        value && CFGetTypeID(value) == CFArrayGetTypeID()) {
      CFArrayRef windows = (CFArrayRef)value;
      CFIndex count = MIN(CFArrayGetCount(windows), 64);
      for (CFIndex i = 0; i < count && CFAbsoluteTimeGetCurrent() < deadline; i++) {
        AXUIElementRef target = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
        AXUIElementSetMessagingTimeout(target, 0.03);
        CGWindowID identifier = 0;
        pid_t owner = 0;
        CFTypeRef role = NULL;
        if (_AXUIElementGetWindow(target, &identifier) != kAXErrorSuccess ||
            identifier != window ||
            AXUIElementGetPid(target, &owner) != kAXErrorSuccess || owner != pid)
          continue;
        // This is a newly fetched root, not the invalidated observed AX object.
        if (AXUIElementCopyAttributeValue(target, kAXRoleAttribute, &role) ==
                kAXErrorSuccess && role && CFEqual(role, kAXWindowRole))
          found = YES;
        if (role)
          CFRelease(role);
        break;
      }
    }
    if (value)
      CFRelease(value);
    CFRelease(app);
    return found && ot_retirement_matches(window, pid, sec, usec);
  }
}
