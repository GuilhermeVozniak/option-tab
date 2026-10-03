// Opt-in read-only AX inventory: no events, pointer movement or Dock changes.
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
static id copyAttribute(AXUIElementRef item, CFStringRef name) {
  CFTypeRef value = NULL;
  if (AXUIElementCopyAttributeValue(item, name, &value) != kAXErrorSuccess)
    return nil;
  return CFBridgingRelease(value);
}
int main(void) {
  @autoreleasepool {
    if (!AXIsProcessTrusted()) {
      puts("REFUSED AX permission");
      return 2;
    }
    NSArray *apps = [NSRunningApplication
        runningApplicationsWithBundleIdentifier:@"com.apple.dock"];
    if (apps.count != 1) {
      puts("REFUSED ambiguous Dock process");
      return 2;
    }
    pid_t pid = [apps.firstObject processIdentifier];
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementSetMessagingTimeout(app, .03);
    NSMutableArray *queue = [NSMutableArray arrayWithObject:(__bridge id)app];
    CFRelease(app);
    int inspected = 0, folders = 0;
    double deadline = NSDate.timeIntervalSinceReferenceDate + 1;
    while (queue.count && inspected++ < 128 &&
           NSDate.timeIntervalSinceReferenceDate < deadline) {
      id item = queue.firstObject;
      [queue removeObjectAtIndex:0];
      AXUIElementRef ax = (__bridge AXUIElementRef)item;
      pid_t owner = 0;
      if (AXUIElementGetPid(ax, &owner) != kAXErrorSuccess || owner != pid)
        continue;
      AXUIElementSetMessagingTimeout(ax, .03);
      NSString *role = copyAttribute(ax, kAXRoleAttribute),
               *sub = copyAttribute(ax, kAXSubroleAttribute);
      if ([role isEqual:@"AXDockItem"] && [sub isEqual:@"AXFolderDockItem"]) {
        id raw = copyAttribute(ax, kAXURLAttribute);
        NSURL *url = [raw isKindOfClass:NSURL.class]
                         ? raw
                         : ([raw isKindOfClass:NSString.class]
                                ? [NSURL URLWithString:raw]
                                : nil);
        if (url.isFileURL &&
            (!url.host.length || [url.host isEqual:@"localhost"])) {
          NSURL *canonical =
              url.URLByStandardizingPath.URLByResolvingSymlinksInPath;
          printf("FOLDER pid=%d role=%s subrole=%s canonical=%s\n", pid,
                 role.UTF8String, sub.UTF8String,
                 canonical.absoluteString.UTF8String);
          folders++;
        }
      }
      NSArray *children = copyAttribute(ax, kAXChildrenAttribute);
      if ([children isKindOfClass:NSArray.class] &&
          queue.count + children.count < 128)
        [queue addObjectsFromArray:children];
    }
    printf("READONLY inspected=%d folders=%d unfinished=%lu foreground=%d\n",
           inspected, folders, (unsigned long)queue.count,
           NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier);
    return folders > 0 ? 0 : 1;
  }
}
