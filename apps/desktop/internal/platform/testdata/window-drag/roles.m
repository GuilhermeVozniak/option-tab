#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef, CGWindowID *);
static CFTypeRef value(AXUIElementRef node, CFStringRef name) {
  CFTypeRef result = NULL;
  AXUIElementSetMessagingTimeout(node, .04);
  AXUIElementCopyAttributeValue(node, name, &result);
  return result;
}
static void inspect(AXUIElementRef node, int depth, int *budget) {
  if (depth > 5 || --*budget < 0)
    return;
  CGWindowID wid = 0, parentID = 0;
  AXError idError = _AXUIElementGetWindow(node, &wid);
  CFTypeRef role = value(node, kAXRoleAttribute),
            subrole = value(node, kAXSubroleAttribute),
            parent = value(node, kAXParentAttribute);
  if (parent && CFGetTypeID(parent) == AXUIElementGetTypeID())
    _AXUIElementGetWindow((AXUIElementRef)parent, &parentID);
  printf("depth=%d window=%u windowError=%d parent=%u role=%s subrole=%s\n",
         depth, wid, idError, parentID,
         role ? [(__bridge id)role description].UTF8String : "?",
         subrole ? [(__bridge id)subrole description].UTF8String : "?");
  if (role)
    CFRelease(role);
  if (subrole)
    CFRelease(subrole);
  if (parent)
    CFRelease(parent);
  CFTypeRef children = value(node, kAXChildrenAttribute);
  if (children && CFGetTypeID(children) == CFArrayGetTypeID())
    for (id child in (__bridge NSArray *)children) {
      if (CFGetTypeID((__bridge CFTypeRef)child) == AXUIElementGetTypeID())
        inspect((__bridge AXUIElementRef)child, depth + 1, budget);
    }
  if (children)
    CFRelease(children);
}
int main(int argc, const char **argv) {
  @autoreleasepool {
    if (argc != 2)
      return 2;
    pid_t pid = atoi(argv[1]);
    NSRunningApplication *app =
        [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (![app.bundleIdentifier
            isEqualToString:@"com.optiontab.windowdragfixture"])
      return 3;
    AXUIElementRef application = AXUIElementCreateApplication(pid);
    CFTypeRef roots = value(application, kAXWindowsAttribute);
    CFRelease(application);
    if (!roots || CFGetTypeID(roots) != CFArrayGetTypeID()) {
      if (roots)
        CFRelease(roots);
      return 4;
    }
    int budget = 80;
    for (id root in (__bridge NSArray *)roots)
      if (CFGetTypeID((__bridge CFTypeRef)root) == AXUIElementGetTypeID())
        inspect((__bridge AXUIElementRef)root, 0, &budget);
    CFRelease(roots);
  }
  return 0;
}
