//go:build darwin
#import "darwin_active_window.h"
#import "darwin_retirement.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#include <libproc.h>
#include <sys/proc_info.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef, CGWindowID *);
extern int goAutomationWindowFinalGuard(uintptr_t);

// File-private seams are overridable only by the standalone test translation
// unit. Production always uses native APIs, with no synthetic admission switch.
static AXError (*copyAttribute)(AXUIElementRef, CFStringRef,
                                CFTypeRef *) = AXUIElementCopyAttributeValue;
static AXError (*setAttribute)(AXUIElementRef, CFStringRef,
                               CFTypeRef) = AXUIElementSetAttributeValue;
static AXError (*performAction)(AXUIElementRef,
                                CFStringRef) = AXUIElementPerformAction;
static AXError (*getWindowNumber)(AXUIElementRef,
                                  CGWindowID *) = _AXUIElementGetWindow;
static AXError (*getElementPID)(AXUIElementRef, pid_t *) = AXUIElementGetPid;
static Boolean (*trusted)(void) = AXIsProcessTrusted;
static int (*matchesIdentity)(uint32_t, int, uint64_t,
                              uint64_t) = ot_retirement_matches;
static int (*readIdentity)(uint32_t, int *, uint64_t *,
                           uint64_t *) = ot_retirement_identity;
static AXError (*isSettable)(AXUIElementRef, CFStringRef,
                             Boolean *) = AXUIElementIsAttributeSettable;
static CFArrayRef (*windowDescriptions)(CGWindowListOption, CGWindowID) =
    CGWindowListCopyWindowInfo;
static BOOL nativeHide(int pid) {
  NSRunningApplication *app =
      [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
  return app && [app hide];
}
static BOOL (*hideProcess)(int) = nativeHide;

static int axStatus(AXError e) {
  switch (e) {
  case kAXErrorSuccess:
    return 0;
  case kAXErrorAPIDisabled:
    return 2;
  case kAXErrorInvalidUIElement:
    return 3;
  case kAXErrorActionUnsupported:
  case kAXErrorAttributeUnsupported:
  case kAXErrorNotImplemented:
    return 5;
  default:
    return 6;
  }
}
static double now(void) { return CFAbsoluteTimeGetCurrent(); }
static AXError readAttribute(AXUIElementRef element, CFStringRef name,
                             CFTypeRef *out, double deadline) {
  *out = NULL;
  if (now() >= deadline)
    return kAXErrorCannotComplete;
  AXUIElementSetMessagingTimeout(element, (float)MIN(0.10, deadline - now()));
  return copyAttribute(element, name, out);
}
static BOOL processStart(int pid, uint64_t *sec, uint64_t *usec) {
  struct proc_bsdinfo info = {0};
  if (pid <= 0 ||
      proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) !=
          sizeof(info) ||
      !info.pbi_start_tvsec)
    return NO;
  *sec = info.pbi_start_tvsec;
  *usec = info.pbi_start_tvusec;
  return YES;
}
int ot_automation_process_identity(int pid, OTAutomationIdentity *out) {
  if (!out || pid <= 0)
    return 1;
  *out = (OTAutomationIdentity){.pid = pid};
  return processStart(pid, &out->sec, &out->usec) ? 0 : 3;
}
int ot_automation_window_identity(uint32_t window, OTAutomationIdentity *out) {
  if (!out || !window)
    return 1;
  *out = (OTAutomationIdentity){.window = window};
  return readIdentity(window, &out->pid, &out->sec, &out->usec) ? 0 : 3;
}
static int identityCurrentFast(OTAutomationIdentity id) {
  return id.window && id.pid > 0 && id.sec &&
         matchesIdentity(id.window, id.pid, id.sec, id.usec);
}
static BOOL rootMatches(AXUIElementRef root, OTAutomationIdentity id,
                        double deadline) {
  if (now() >= deadline || !identityCurrentFast(id))
    return NO;
  pid_t pid = 0;
  CGWindowID window = 0;
  if (getElementPID(root, &pid) != kAXErrorSuccess || pid != id.pid ||
      getWindowNumber(root, &window) != kAXErrorSuccess || window != id.window)
    return NO;
  CFTypeRef role = NULL;
  AXError e = readAttribute(root, kAXRoleAttribute, &role, deadline);
  BOOL valid = e == kAXErrorSuccess && role &&
               CFGetTypeID(role) == CFStringGetTypeID() &&
               CFEqual(role, kAXWindowRole);
  if (role)
    CFRelease(role);
  return valid && now() < deadline;
}
static AXUIElementRef copyRoot(OTAutomationIdentity id, double deadline,
                               int *status) {
  *status = 4;
  AXUIElementRef app = AXUIElementCreateApplication(id.pid);
  CFTypeRef roots = NULL;
  AXError e = readAttribute(app, kAXWindowsAttribute, &roots, deadline);
  CFRelease(app);
  if (e != kAXErrorSuccess) {
    *status = axStatus(e);
    if (roots)
      CFRelease(roots);
    return NULL;
  }
  if (!roots || CFGetTypeID(roots) != CFArrayGetTypeID()) {
    if (roots)
      CFRelease(roots);
    return NULL;
  }
  CFIndex n = CFArrayGetCount(roots);
  if (n > 512) {
    CFRelease(roots);
    return NULL;
  }
  AXUIElementRef found = NULL;
  for (CFIndex i = 0; i < n && now() < deadline; i++) {
    CFTypeRef value = CFArrayGetValueAtIndex(roots, i);
    if (!value || CFGetTypeID(value) != AXUIElementGetTypeID())
      continue;
    CGWindowID window = 0;
    AXUIElementSetMessagingTimeout((AXUIElementRef)value, 0.10);
    if (getWindowNumber((AXUIElementRef)value, &window) == kAXErrorSuccess &&
        window == id.window) {
      if (found) {
        CFRelease(found);
        found = NULL;
        break;
      }
      found = (AXUIElementRef)CFRetain(value);
    }
  }
  CFRelease(roots);
  if (found && !rootMatches(found, id, deadline)) {
    CFRelease(found);
    found = NULL;
    *status = 3;
  }
  if (now() >= deadline) {
    if (found)
      CFRelease(found);
    found = NULL;
    *status = 8;
  }
  if (found)
    *status = 0;
  return found;
}
int ot_automation_window_current(OTAutomationIdentity id) {
  if (!trusted() || !identityCurrentFast(id))
    return 0;
  int status = 0;
  AXUIElementRef root = copyRoot(id, now() + 0.5, &status);
  if (!root)
    return 0;
  CFRelease(root);
  return status == 0;
}
static int copyFocused(AXUIElementRef *rootOut, OTAutomationIdentity *id,
                       double deadline) {
  *rootOut = NULL;
  AXUIElementRef system = AXUIElementCreateSystemWide();
  CFTypeRef focusedApp = NULL, focusedWindow = NULL;
  AXError e = readAttribute(system, kAXFocusedApplicationAttribute, &focusedApp,
                            deadline);
  CFRelease(system);
  if (e != kAXErrorSuccess || !focusedApp ||
      CFGetTypeID(focusedApp) != AXUIElementGetTypeID()) {
    if (focusedApp)
      CFRelease(focusedApp);
    return e == kAXErrorSuccess ? 4 : axStatus(e);
  }
  pid_t pid = 0;
  if (getElementPID((AXUIElementRef)focusedApp, &pid) != kAXErrorSuccess ||
      pid <= 0) {
    CFRelease(focusedApp);
    return 4;
  }
  e = readAttribute((AXUIElementRef)focusedApp, kAXFocusedWindowAttribute,
                    &focusedWindow, deadline);
  CFRelease(focusedApp);
  if (e != kAXErrorSuccess || !focusedWindow ||
      CFGetTypeID(focusedWindow) != AXUIElementGetTypeID()) {
    if (focusedWindow)
      CFRelease(focusedWindow);
    return e == kAXErrorSuccess ? 4 : axStatus(e);
  }
  CGWindowID window = 0;
  pid_t owner = 0;
  getElementPID((AXUIElementRef)focusedWindow, &owner);
  if (owner != pid ||
      getWindowNumber((AXUIElementRef)focusedWindow, &window) !=
          kAXErrorSuccess ||
      !window || ot_automation_window_identity(window, id) != 0 ||
      id->pid != pid) {
    CFRelease(focusedWindow);
    return 3;
  }
  int status = 0;
  AXUIElementRef root = copyRoot(*id, deadline, &status);
  BOOL same = root && CFEqual(root, focusedWindow);
  if (root)
    CFRelease(root);
  if (!same) {
    CFRelease(focusedWindow);
    return status ? status : 4;
  }
  *rootOut = (AXUIElementRef)focusedWindow;
  return 0;
}
static NSDictionary *identityJSON(OTAutomationIdentity id) {
  return @{
    @"ID" : @(id.window),
    @"Process" : @{
      @"PID" : @(id.pid),
      @"StartSeconds" : @(id.sec),
      @"StartMicros" : @(id.usec)
    }
  };
}
static NSString *textAttribute(AXUIElementRef root, CFStringRef attribute,
                               double deadline) {
  CFTypeRef value = NULL;
  readAttribute(root, attribute, &value, deadline);
  NSString *text = @"";
  if (value && CFGetTypeID(value) == CFStringGetTypeID())
    text = [(__bridge NSString *)value copy];
  if (value)
    CFRelease(value);
  return text;
}
static BOOL boolAttribute(AXUIElementRef root, CFStringRef attribute,
                          double deadline) {
  CFTypeRef value = NULL;
  readAttribute(root, attribute, &value, deadline);
  BOOL result = value && CFGetTypeID(value) == CFBooleanGetTypeID() &&
                CFBooleanGetValue(value);
  if (value)
    CFRelease(value);
  return result;
}
static NSDictionary *windowJSON(AXUIElementRef root, OTAutomationIdentity id,
                                double deadline) {
  CFTypeRef position = NULL, size = NULL;
  CGPoint point = CGPointZero;
  CGSize dimensions = CGSizeZero;
  AXError p = readAttribute(root, kAXPositionAttribute, &position, deadline),
          s = readAttribute(root, kAXSizeAttribute, &size, deadline);
  BOOL valid = p == kAXErrorSuccess && s == kAXErrorSuccess && position &&
               size && CFGetTypeID(position) == AXValueGetTypeID() &&
               CFGetTypeID(size) == AXValueGetTypeID() &&
               AXValueGetValue(position, kAXValueCGPointType, &point) &&
               AXValueGetValue(size, kAXValueCGSizeType, &dimensions);
  if (position)
    CFRelease(position);
  if (size)
    CFRelease(size);
  if (!valid)
    return nil;
  CFArrayRef descriptions =
      windowDescriptions(kCGWindowListOptionIncludingWindow, id.window);
  NSDictionary *description = nil;
  for (NSDictionary *candidate in (__bridge NSArray *)descriptions) {
    if ([candidate[(__bridge NSString *)kCGWindowNumber] unsignedIntValue] ==
            id.window &&
        [candidate[(__bridge NSString *)kCGWindowOwnerPID] intValue] ==
            id.pid) {
      description = candidate;
      break;
    }
  }
  NSNumber *onScreen = description[(__bridge NSString *)kCGWindowIsOnscreen];
  if (descriptions)
    CFRelease(descriptions);
  if (!onScreen)
    return nil;
  NSRunningApplication *app =
      [NSRunningApplication runningApplicationWithProcessIdentifier:id.pid];
  return @{
    @"ID" : @(id.window),
    @"AppID" : @(id.pid),
    @"PID" : @(id.pid),
    @"AppName" : app.localizedName ?: @"",
    @"BundleID" : app.bundleIdentifier ?: @"",
    @"Title" : textAttribute(root, kAXTitleAttribute, deadline),
    @"Bounds" : @{
      @"X" : @(point.x),
      @"Y" : @(point.y),
      @"W" : @(dimensions.width),
      @"H" : @(dimensions.height)
    },
    @"Minimized" : @(boolAttribute(root, kAXMinimizedAttribute, deadline)),
    @"Fullscreen" : @(boolAttribute(root, CFSTR("AXFullScreen"), deadline)),
    @"Hidden" : @(app.hidden),
    @"OnScreen" : @([onScreen boolValue])
  };
}
char *ot_automation_active_window(void) {
  @autoreleasepool {
    NSDictionary *result = @{@"Status" : @4};
    if (!trusted())
      result = @{@"Status" : @2};
    else {
      double deadline = now() + 2.0;
      for (int attempt = 0; attempt < 2 && now() < deadline; attempt++) {
        AXUIElementRef root = NULL, again = NULL;
        OTAutomationIdentity first = {0}, second = {0};
        int status = copyFocused(&root, &first, deadline);
        if (status) {
          result = @{@"Status" : @(status)};
          if (root)
            CFRelease(root);
          break;
        }
        NSDictionary *window = windowJSON(root, first, deadline);
        status = copyFocused(&again, &second, deadline);
        BOOL same = !status && window && first.window == second.window &&
                    first.pid == second.pid && first.sec == second.sec &&
                    first.usec == second.usec && CFEqual(root, again) &&
                    identityCurrentFast(first) && now() < deadline;
        if (root)
          CFRelease(root);
        if (again)
          CFRelease(again);
        if (same) {
          result = @{
            @"Status" : @0,
            @"Window" : window,
            @"Identity" : identityJSON(first)
          };
          break;
        }
        result = @{@"Status" : @(now() >= deadline ? 8 : 4)};
      }
    }
    NSData *json = [NSJSONSerialization dataWithJSONObject:result
                                                   options:0
                                                     error:nil];
    return json ? strdup([[NSString alloc] initWithData:json
                                               encoding:NSUTF8StringEncoding]
                             .UTF8String)
                : NULL;
  }
}
static int finalAdmission(AXUIElementRef root, OTAutomationIdentity id,
                          double deadline, uintptr_t guard) {
  if (now() >= deadline)
    return 8;
  if (!trusted())
    return 2;
  if (!guard || !goAutomationWindowFinalGuard(guard))
    return 7;
  if (!rootMatches(root, id, deadline))
    return 3;
  // The AX root recheck can itself block. Repeat request admission after it,
  // then make the cheap captured CG/process check immediately before mutation.
  if (!goAutomationWindowFinalGuard(guard))
    return 7;
  if (!trusted())
    return 2;
  if (now() >= deadline)
    return 8;
  return identityCurrentFast(id) ? 0 : 3;
}
static int writeBoolean(AXUIElementRef root, CFStringRef attribute,
                        Boolean desired, OTAutomationIdentity id,
                        double deadline, uintptr_t guard) {
  Boolean settable = false;
  AXError e = isSettable(root, attribute, &settable);
  if (e != kAXErrorSuccess)
    return axStatus(e);
  if (!settable)
    return 5;
  int status = finalAdmission(root, id, deadline, guard);
  if (status)
    return status;
  return axStatus(setAttribute(root, attribute,
                               desired ? kCFBooleanTrue : kCFBooleanFalse));
}
int ot_automation_window_action(int action, OTAutomationIdentity id,
                                int desired, uintptr_t guard) {
  @autoreleasepool {
    if (action < 1 || action > 5 || !id.window || id.pid <= 0 || !id.sec ||
        !guard)
      return 1;
    if (!trusted())
      return 2;
    if (!identityCurrentFast(id))
      return 3;
    double deadline = now() + 1.5;
    int status = 0;
    AXUIElementRef root = copyRoot(id, deadline, &status);
    if (!root)
      return status;
    if (action == 3 || action == 5)
      status = writeBoolean(
          root, action == 3 ? kAXMinimizedAttribute : CFSTR("AXFullScreen"),
          action == 3 ? true : desired != 0, id, deadline, guard);
    else if (action == 2) {
      CFTypeRef button = NULL;
      AXError e =
          readAttribute(root, kAXCloseButtonAttribute, &button, deadline);
      status = axStatus(e);
      if (!status && (!button || CFGetTypeID(button) != AXUIElementGetTypeID()))
        status = 5;
      if (!status) {
        status = finalAdmission(root, id, deadline, guard);
        if (!status)
          status =
              axStatus(performAction((AXUIElementRef)button, kAXPressAction));
      }
      if (button)
        CFRelease(button);
    } else if (action == 4) {
      status = finalAdmission(root, id, deadline, guard);
      if (!status)
        status = hideProcess(id.pid) ? 0 : 6;
    } else {
      AXUIElementRef app = AXUIElementCreateApplication(id.pid);
      AXUIElementSetMessagingTimeout(app, 0.10);
      status = finalAdmission(root, id, deadline, guard);
      if (!status)
        status = axStatus(setAttribute(app, kAXFocusedWindowAttribute, root));
      if (!status) {
        status = finalAdmission(root, id, deadline, guard);
        if (!status)
          status = axStatus(
              setAttribute(app, kAXFrontmostAttribute, kCFBooleanTrue));
      }
      if (!status) {
        status = finalAdmission(root, id, deadline, guard);
        if (!status) {
          // Raising a minimized window can wait for AppKit's restore
          // animation. The preceding AX lookup set a 100 ms timeout; give
          // this admitted action the remainder of its original deadline.
          double remaining = deadline - now();
          if (remaining <= 0)
            status = 8;
          else {
            AXUIElementSetMessagingTimeout(root, (float)remaining);
            status = axStatus(performAction(root, kAXRaiseAction));
            if (now() >= deadline)
              status = 8;
          }
        }
      }
      CFRelease(app);
    }
    CFRelease(root);
    return status;
  }
}
