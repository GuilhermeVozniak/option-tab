#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
static BOOL identityValid = YES, permission = YES, guardAllowed = YES,
            badRole = NO, changeDuringPreparation = NO, churn = NO,
            revokeDuringPreparation = NO;
static int writes = 0, presses = 0, guards = 0, focusReads = 0;
static AXError writeError = kAXErrorSuccess;
static AXError raiseError = kAXErrorSuccess;
static CFTypeRef lastValue = NULL;
static AXUIElementRef firstRoot, secondRoot;
static float messageTimeout;
static BOOL slowFocusRaise, expireWhileFronting, expireWhileRaising,
            retireWhileFronting;
static double clockOffset;
static double testClock(void) {
  return CFAbsoluteTimeGetCurrent() + clockOffset;
}
static AXError fakeMessagingTimeout(AXUIElementRef element, float seconds) {
  (void)element;
  messageTimeout = seconds;
  return kAXErrorSuccess;
}
int ot_retirement_matches(uint32_t window, int pid, uint64_t sec,
                          uint64_t usec) {
  return identityValid &&
         ((window == 9 && pid == 7) || (window == 10 && pid == 8)) &&
         sec == 10 && usec == 2;
}
int ot_retirement_identity(uint32_t window, int *pid, uint64_t *sec,
                           uint64_t *usec) {
  *pid = window == 9 ? 7 : 8;
  *sec = 10;
  *usec = 2;
  return identityValid;
}
int goAutomationWindowFinalGuard(uintptr_t token) {
  (void)token;
  guards++;
  return guardAllowed;
}
#define AXUIElementSetMessagingTimeout fakeMessagingTimeout
#define CFAbsoluteTimeGetCurrent testClock
#include "../../darwin_active_window.m"
#undef AXUIElementSetMessagingTimeout
#undef CFAbsoluteTimeGetCurrent
static Boolean fakeTrusted(void) { return permission; }
static AXError fakePID(AXUIElementRef root, pid_t *pid) {
  *pid = CFEqual(root, secondRoot) ? 8 : 7;
  return kAXErrorSuccess;
}
static AXError fakeWindow(AXUIElementRef root, CGWindowID *window) {
  *window = CFEqual(root, secondRoot) ? 10 : 9;
  return kAXErrorSuccess;
}
static AXError fakeCopy(AXUIElementRef element, CFStringRef name,
                        CFTypeRef *out) {
  *out = NULL;
  if (CFEqual(name, kAXFocusedApplicationAttribute)) {
    *out = CFRetain(churn && focusReads % 2 ? secondRoot : firstRoot);
  } else if (CFEqual(name, kAXFocusedWindowAttribute)) {
    *out = CFRetain(churn && focusReads % 2 ? secondRoot : firstRoot);
    focusReads++;
  } else if (CFEqual(name, kAXWindowsAttribute)) {
    const void *root = CFEqual(element, secondRoot) ? secondRoot : firstRoot;
    *out = CFArrayCreate(NULL, &root, 1, &kCFTypeArrayCallBacks);
  } else if (CFEqual(name, kAXRoleAttribute)) {
    *out = CFRetain(badRole ? CFSTR("AXButton") : kAXWindowRole);
  } else if (CFEqual(name, kAXCloseButtonAttribute)) {
    *out = CFRetain(element);
  } else if (CFEqual(name, kAXPositionAttribute)) {
    CGPoint p = CGPointMake(-100, 40);
    *out = AXValueCreate(kAXValueCGPointType, &p);
  } else if (CFEqual(name, kAXSizeAttribute)) {
    CGSize s = CGSizeMake(300, 200);
    *out = AXValueCreate(kAXValueCGSizeType, &s);
  } else if (CFEqual(name, kAXTitleAttribute)) {
    *out = CFRetain(CFSTR("Disposable seam title"));
  } else {
    *out = CFRetain(kCFBooleanFalse);
  }
  return kAXErrorSuccess;
}
static AXError fakeSettable(AXUIElementRef root, CFStringRef name,
                            Boolean *value) {
  (void)root;
  (void)name;
  *value = true;
  if (changeDuringPreparation)
    identityValid = NO;
  if (revokeDuringPreparation)
    permission = NO;
  return kAXErrorSuccess;
}
static AXError fakeSet(AXUIElementRef root, CFStringRef name, CFTypeRef value) {
  (void)root;
  (void)name;
  writes++;
  lastValue = value;
  if (CFEqual(name, kAXFrontmostAttribute)) {
    if (expireWhileFronting)
      clockOffset += 2;
    if (retireWhileFronting)
      guardAllowed = NO;
  }
  return writeError;
}
static AXError fakePerform(AXUIElementRef root, CFStringRef action) {
  (void)root;
  presses++;
  // AppKit can spend several hundred milliseconds restoring a minimized
  // document. A short lookup timeout rejects the action before it replies.
  if (slowFocusRaise && CFEqual(action, kAXRaiseAction) && messageTimeout < .3)
    return kAXErrorCannotComplete;
  if (expireWhileRaising && CFEqual(action, kAXRaiseAction))
    clockOffset += 2;
  if (CFEqual(action, kAXRaiseAction) && raiseError != kAXErrorSuccess)
    return raiseError;
  return writeError;
}
static BOOL fakeHide(int pid) {
  NSCAssert(pid == 7, @"wrong process hidden");
  writes++;
  return YES;
}
static CFArrayRef fakeDescriptions(CGWindowListOption option,
                                   CGWindowID window) {
  (void)option;
  NSDictionary *v = @{
    (__bridge NSString *)kCGWindowNumber : @(window),
    (__bridge NSString *)kCGWindowOwnerPID : @(window == 9 ? 7 : 8),
    (__bridge NSString *)kCGWindowIsOnscreen : @YES
  };
  return CFBridgingRetain(@[ v ]);
}
int main(void) {
  @autoreleasepool {
    firstRoot = AXUIElementCreateApplication(7);
    secondRoot = AXUIElementCreateApplication(8);
    trusted = fakeTrusted;
    windowDescriptions = fakeDescriptions;
    copyAttribute = fakeCopy;
    setAttribute = fakeSet;
    performAction = fakePerform;
    getElementPID = fakePID;
    getWindowNumber = fakeWindow;
    isSettable = fakeSettable;
    hideProcess = fakeHide;
    OTAutomationIdentity id = {.window = 9, .pid = 7, .sec = 10, .usec = 2};
    guardAllowed = NO;
    NSCAssert(ot_automation_window_action(3, id, 0, 1) == 7 && writes == 0,
              @"cancelled preparation mutated");
    guardAllowed = YES;
    changeDuringPreparation = YES;
    NSCAssert(ot_automation_window_action(3, id, 0, 1) == 3 && writes == 0,
              @"replaced identity mutated");
    changeDuringPreparation = NO;
    identityValid = YES;
    revokeDuringPreparation = YES;
    NSCAssert(ot_automation_window_action(3, id, 0, 1) == 2 && writes == 0,
              @"permission revoked during preparation mutated");
    revokeDuringPreparation = NO;
    permission = YES;
    NSCAssert(ot_automation_window_action(3, id, 0, 1) == 0 &&
                  lastValue == kCFBooleanTrue,
              @"minimize must set true");
    NSCAssert(ot_automation_window_action(3, id, 0, 1) == 0 &&
                  lastValue == kCFBooleanTrue,
              @"repeat minimize restored");
    NSCAssert(ot_automation_window_action(5, id, 1, 1) == 0 &&
                  lastValue == kCFBooleanTrue,
              @"fullscreen true missing");
    NSCAssert(ot_automation_window_action(5, id, 0, 1) == 0 &&
                  lastValue == kCFBooleanFalse,
              @"fullscreen false missing");
    writeError = kAXErrorActionUnsupported;
    NSCAssert(ot_automation_window_action(2, id, 0, 1) == 5,
              @"AX rejection became success");
    writeError = kAXErrorSuccess;
    NSCAssert(ot_automation_window_action(4, id, 0, 1) == 0,
              @"explicit app hide refused");
    slowFocusRaise = YES;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 0,
              @"focus rejected a native restore that fits its action deadline");
    NSCAssert(messageTimeout > .3 && messageTimeout <= 1.5,
              @"focus did not retain its bounded action deadline");
    slowFocusRaise = NO;
    int beforeRaise = presses;
    retireWhileFronting = YES;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 7 &&
                  presses == beforeRaise,
              @"retired focus request raised a window");
    retireWhileFronting = NO;
    guardAllowed = YES;
    expireWhileFronting = YES;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 8 &&
                  presses == beforeRaise,
              @"focus raised after its original deadline");
    expireWhileFronting = NO;
    expireWhileRaising = YES;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 8,
              @"late native focus success escaped the action deadline");
    expireWhileRaising = NO;
    raiseError = kAXErrorCannotComplete;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 6,
              @"native focus rejection became success");
    raiseError = kAXErrorSuccess;
    permission = NO;
    NSCAssert(ot_automation_window_action(1, id, 0, 1) == 2,
              @"permission refusal missing");
    permission = YES;
    char *json = ot_automation_active_window();
    NSCAssert(json && strstr(json, "\"Status\":0") && focusReads == 2,
              @"authoritative focus was not revalidated");
    free(json);
    churn = YES;
    focusReads = 0;
    json = ot_automation_active_window();
    NSCAssert(json && strstr(json, "\"Status\":4") && focusReads == 4,
              @"focus churn not bounded/refused");
    free(json);
    churn = NO;
    badRole = YES;
    NSCAssert(!ot_automation_window_current(id),
              @"retained CG surface without AX root considered current");
    CFRelease(firstRoot);
    CFRelease(secondRoot);
    puts("PASS exact AX roots, focused snapshot/recheck, churn refusal, "
         "preparation cancellation, process replacement, minimize/fullscreen "
         "set semantics, native errors; no actual AX messages or UI");
    return 0;
  }
}
