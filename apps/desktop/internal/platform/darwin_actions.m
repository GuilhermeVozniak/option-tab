//go:build darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import "darwin_actions.h"

static NSRunningApplication *actionApp(int pid) {
  if (pid <= 0 || pid == getpid()) return nil;
  NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
  if (!app || app.terminated || !app.bundleURL || app.activationPolicy == NSApplicationActivationPolicyProhibited) return nil;
  return app;
}
int ot_action_app_valid(int pid) {
  @autoreleasepool { return actionApp(pid) != nil; }
}
int ot_action_force_quit(int pid) {
  @autoreleasepool { NSRunningApplication *app = actionApp(pid); return app && [app forceTerminate]; }
}

// Deliberately use explicit menu titles rather than Cmd+N: that shortcut can
// create a document/tab or affect an unrelated app. Unrecognized/localized menu
// commands are unsupported until they have an unambiguous mapping.
static AXUIElementRef copyNewWindowCommand(AXUIElementRef node, int depth, int *budget, CFAbsoluteTime deadline) {
  if (depth > 5 || --*budget < 0 || CFAbsoluteTimeGetCurrent() > deadline) return NULL;
  AXUIElementSetMessagingTimeout(node, 0.1);
  CFTypeRef title = NULL, role = NULL, enabled = NULL;
  AXUIElementCopyAttributeValue(node, kAXTitleAttribute, &title);
  AXUIElementCopyAttributeValue(node, kAXRoleAttribute, &role);
  AXUIElementCopyAttributeValue(node, kAXEnabledAttribute, &enabled);
  BOOL match = title && CFGetTypeID(title) == CFStringGetTypeID() &&
    (CFEqual(title, CFSTR("New Window")) || CFEqual(title, CFSTR("New Finder Window")));
  BOOL menuItem = role && CFGetTypeID(role) == CFStringGetTypeID() && CFEqual(role, kAXMenuItemRole);
  BOOL active = enabled && CFGetTypeID(enabled) == CFBooleanGetTypeID() && CFBooleanGetValue(enabled);
  if (title) CFRelease(title);
  if (role) CFRelease(role);
  if (enabled) CFRelease(enabled);
  if (match && menuItem && active) return (AXUIElementRef)CFRetain(node);
  CFTypeRef children = NULL;
  AXUIElementCopyAttributeValue(node, kAXChildrenAttribute, &children);
  AXUIElementRef found = NULL;
  if (children && CFGetTypeID(children) == CFArrayGetTypeID()) {
    for (CFIndex i = 0; i < CFArrayGetCount(children) && *budget > 0; i++) {
      CFTypeRef child = CFArrayGetValueAtIndex(children, i);
      if (CFGetTypeID(child) != AXUIElementGetTypeID()) continue;
      found = copyNewWindowCommand((AXUIElementRef)child, depth + 1, budget, deadline);
      if (found) break;
    }
  }
  if (children) CFRelease(children);
  return found;
}
int ot_action_new_window(int pid) {
  @autoreleasepool {
    if (!actionApp(pid)) return 0;
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementSetMessagingTimeout(app, 0.25);
    CFTypeRef bar = NULL;
    AXError error = AXUIElementCopyAttributeValue(app, kAXMenuBarAttribute, &bar);
    CFRelease(app);
    if (error != kAXErrorSuccess || !bar) return -1;
    if (CFGetTypeID(bar) != AXUIElementGetTypeID()) { CFRelease(bar); return -1; }
    int budget = 400;
    AXUIElementRef command = copyNewWindowCommand((AXUIElementRef)bar, 0, &budget, CFAbsoluteTimeGetCurrent() + 1.0);
    CFRelease(bar);
    if (!command) return -1;
    // Recheck that the requested process still exists before pressing its own AX element.
    int result = actionApp(pid) && AXUIElementPerformAction(command, kAXPressAction) == kAXErrorSuccess;
    CFRelease(command);
    return result;
  }
}
