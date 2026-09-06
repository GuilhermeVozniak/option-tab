//go:build darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <unistd.h>
#import "darwin_app_window_inventory.h"

// NSRunningApplication properties are documented as thread safe. AX queries
// stay off AppKit's main thread and have a 50ms per-message timeout plus a
// 150ms classification deadline. This read never prompts for permission.
OTAppWindowEvidence ot_app_window_evidence(int pid) {
  OTAppWindowEvidence result = {0, 0, -1, -1};
  if (pid <= 0 || pid == getpid() || !AXIsProcessTrusted()) return result;
  @autoreleasepool {
    NSRunningApplication *before = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (!before || before.terminated || before.activationPolicy != NSApplicationActivationPolicyRegular || !before.bundleIdentifier.length) return result;
    NSString *bundle = before.bundleIdentifier;
    NSDate *launch = before.launchDate;
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    if (!app) return result;
    AXUIElementSetMessagingTimeout(app, 0.05);
    CFTypeRef raw = NULL;
    CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 0.15;
    AXError error = AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &raw);
    if (error == kAXErrorSuccess && raw && CFGetTypeID(raw) == CFArrayGetTypeID()) {
      result.ax_readable = 1;
      CFArrayRef windows = (CFArrayRef)raw;
      result.ax_windows = CFArrayGetCount(windows) == 0 ? 0 : -1;
      // One confirmed root is enough to prove presence. A malformed or slow
      // app remains unknown; absence from this AX array alone proves nothing.
      for (CFIndex i = 0; i < CFArrayGetCount(windows) && i < 16 && CFAbsoluteTimeGetCurrent() < deadline; i++) {
        CFTypeRef value = CFArrayGetValueAtIndex(windows, i);
        if (CFGetTypeID(value) != AXUIElementGetTypeID()) continue;
        AXUIElementRef window = (AXUIElementRef)value;
        AXUIElementSetMessagingTimeout(window, 0.05);
        pid_t owner = 0;
        if (AXUIElementGetPid(window, &owner) != kAXErrorSuccess || owner != pid) continue;
        CFTypeRef role = NULL;
        AXError roleError = AXUIElementCopyAttributeValue(window, kAXRoleAttribute, &role);
        BOOL root = roleError == kAXErrorSuccess && role && CFGetTypeID(role) == CFStringGetTypeID() && CFEqual(role, kAXWindowRole);
        if (role) CFRelease(role);
        if (root) { result.ax_windows = 1; break; }
      }
    }
    if (raw) CFRelease(raw);
    CFRelease(app);
    if (result.ax_readable && result.ax_windows == 0) {
      // optionAll includes on- and off-screen windows. Count every owned CG
      // surface, not only layer zero: uncertain floating/off-Space surfaces
      // cannot establish an empty application inventory either.
      CFArrayRef list = CGWindowListCopyWindowInfo(kCGWindowListOptionAll, kCGNullWindowID);
      if (list) {
        result.cg_candidates = 0;
        for (NSDictionary *info in (__bridge NSArray *)list) {
          NSNumber *owner = info[(__bridge NSString *)kCGWindowOwnerPID];
          if (owner.intValue == pid) { result.cg_candidates++; break; }
        }
        CFRelease(list);
      }
    }
    NSRunningApplication *after = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    BOOL sameLaunch = launch == after.launchDate || [launch isEqualToDate:after.launchDate];
    result.identity_valid = after && !after.terminated && after.activationPolicy == NSApplicationActivationPolicyRegular && [bundle isEqualToString:after.bundleIdentifier] && sameLaunch;
  }
  return result;
}
