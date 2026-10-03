//go:build darwin
#import "darwin_launcher_badges.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <libproc.h>
extern int goLauncherBadgeCurrent(uintptr_t);
static NSDictionary *badgeProcess(pid_t pid) {
  struct proc_bsdinfo info = {0};
  if (pid <= 0 ||
      proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) !=
          sizeof(info) ||
      !info.pbi_start_tvsec)
    return nil;
  return @{
    @"PID" : @(pid),
    @"StartSeconds" : @(info.pbi_start_tvsec),
    @"StartMicros" : @(info.pbi_start_tvusec)
  };
}
static NSDictionary *badgeDock(void) {
  NSArray *apps = [NSRunningApplication
      runningApplicationsWithBundleIdentifier:@"com.apple.dock"];
  if (apps.count != 1)
    return nil;
  NSRunningApplication *app = apps.firstObject;
  if (app.terminated)
    return nil;
  return badgeProcess(app.processIdentifier);
}
static NSString *badgePath(NSURL *url) {
  if (!url.isFileURL)
    return nil;
  NSURL *u = url.URLByStandardizingPath.URLByResolvingSymlinksInPath;
  return [u.path.pathExtension.lowercaseString isEqualToString:@"app"] ? u.path
                                                                       : nil;
}
static BOOL badgeTarget(NSDictionary *t) {
  NSString *path = t[@"path"], *bundle = t[@"bundleID"];
  if (![path isKindOfClass:NSString.class] ||
      ![bundle isKindOfClass:NSString.class])
    return NO;
  NSURL *url = [NSURL fileURLWithPath:path];
  if (![badgePath(url) isEqualToString:path])
    return NO;
  NSDictionary *process = t[@"process"];
  pid_t pid = [process[@"PID"] intValue];
  if (pid) {
    NSRunningApplication *app =
        [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    return app && !app.terminated &&
           [app.bundleIdentifier isEqualToString:bundle] &&
           [badgePath(app.bundleURL) isEqualToString:path] &&
           [badgeProcess(pid) isEqual:process];
  }
  return
      [[[NSBundle bundleWithURL:url] bundleIdentifier] isEqualToString:bundle];
}
static CFTypeRef badgeCopy(AXUIElementRef node, CFStringRef name,
                           CFAbsoluteTime deadline) {
  double remaining = deadline - CFAbsoluteTimeGetCurrent();
  if (remaining <= 0)
    return NULL;
  AXUIElementSetMessagingTimeout(node, MIN(.025, remaining));
  CFTypeRef value = NULL;
  if (AXUIElementCopyAttributeValue(node, name, &value) != kAXErrorSuccess) {
    if (value)
      CFRelease(value);
    return NULL;
  }
  return value;
}
static CFArrayRef badgeNames(AXUIElementRef node, CFAbsoluteTime deadline) {
  double remaining = deadline - CFAbsoluteTimeGetCurrent();
  if (remaining <= 0)
    return NULL;
  AXUIElementSetMessagingTimeout(node, MIN(.025, remaining));
  CFArrayRef names = NULL;
  if (AXUIElementCopyAttributeNames(node, &names) != kAXErrorSuccess) {
    if (names)
      CFRelease(names);
    return NULL;
  }
  return names;
}
typedef struct {
  BOOL (*trusted)(void);
  NSDictionary *(*dock)(void);
  BOOL (*target)(NSDictionary *);
  CFTypeRef (*copy)(AXUIElementRef, CFStringRef, CFAbsoluteTime);
  CFArrayRef (*names)(AXUIElementRef, CFAbsoluteTime);
  AXUIElementRef (*root)(pid_t);
  NSString *(*path)(NSURL *);
} OTBadgeOps;
static BOOL badgeTrusted(void) { return AXIsProcessTrusted(); }
static OTBadgeOps badgeOps = {badgeTrusted, badgeDock,
                              badgeTarget,  badgeCopy,
                              badgeNames,   AXUIElementCreateApplication,
                              badgePath};
static BOOL badgeCurrent(uintptr_t token, CFAbsoluteTime deadline) {
  return CFAbsoluteTimeGetCurrent() < deadline && goLauncherBadgeCurrent(token);
}
static BOOL badgeEqual(CFTypeRef value, CFStringRef expected) {
  return value && CFGetTypeID(value) == CFStringGetTypeID() &&
         CFEqual(value, expected);
}
static NSDictionary *badgeReadValue(AXUIElementRef node,
                                    CFAbsoluteTime deadline, uintptr_t token) {
  if (!badgeCurrent(token, deadline))
    return @{@"state" : @"unavailable"};
  CFArrayRef names = badgeOps.names(node, deadline);
  BOOL typed = names && CFGetTypeID(names) == CFArrayGetTypeID() &&
               CFArrayGetCount(names) <= 256;
  BOOL advertised = typed && CFArrayContainsValue(
                                 names, CFRangeMake(0, CFArrayGetCount(names)),
                                 CFSTR("AXStatusLabel"));
  if (names)
    CFRelease(names);
  if (!typed)
    return @{@"state" : @"unavailable"};
  if (!advertised)
    return @{@"state" : @"unsupported"};
  if (!badgeCurrent(token, deadline))
    return @{@"state" : @"unavailable"};
  CFTypeRef value = badgeOps.copy(node, CFSTR("AXStatusLabel"), deadline);
  NSDictionary *out = @{@"state" : @"unavailable"};
  if (value && CFGetTypeID(value) == CFStringGetTypeID()) {
    NSString *text = (__bridge NSString *)value;
    out = @{@"state" : @"known", @"text" : text.length <= 32 ? text : @"*"};
  }
  if (value)
    CFRelease(value);
  return out;
}
char *ot_launcher_badges_read(const char *json, uintptr_t token) {
  @autoreleasepool {
    NSData *data = json ? [[NSString stringWithUTF8String:json]
                              dataUsingEncoding:NSUTF8StringEncoding]
                        : nil;
    NSArray *targets = data ? [NSJSONSerialization JSONObjectWithData:data
                                                              options:0
                                                                error:nil]
                            : nil;
    if (![targets isKindOfClass:NSArray.class] || targets.count > 256)
      return NULL;
    CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 3;
    NSDictionary *dock = nil;
    NSMutableDictionary<NSString *, NSMutableArray *> *nodes =
        [NSMutableDictionary new];
    BOOL complete = NO;
    if (badgeOps.trusted() && badgeCurrent(token, deadline) &&
        (dock = badgeOps.dock())) {
      pid_t pid = [dock[@"PID"] intValue];
      AXUIElementRef root = badgeOps.root(pid);
      NSMutableArray *queue =
          [NSMutableArray arrayWithObject:(__bridge id)root];
      CFRelease(root);
      BOOL capped = NO;
      NSUInteger processed = 0;
      for (NSUInteger i = 0; i < queue.count && badgeCurrent(token, deadline);
           i++) {
        AXUIElementRef node = (__bridge AXUIElementRef)queue[i];
        pid_t owner = 0;
        if (AXUIElementGetPid(node, &owner) != kAXErrorSuccess ||
            owner != pid) {
          capped = YES;
          continue;
        }
        processed++;
        CFTypeRef role = badgeOps.copy(node, kAXRoleAttribute, deadline),
                  sub = badgeOps.copy(node, kAXSubroleAttribute, deadline);
        BOOL dockItem = badgeEqual(role, kAXDockItemRole);
        BOOL app = dockItem && badgeEqual(sub, kAXApplicationDockItemSubrole);
        if (role)
          CFRelease(role);
        if (sub)
          CFRelease(sub);
        if (app) {
          CFTypeRef raw = badgeOps.copy(node, kAXURLAttribute, deadline);
          NSString *path = raw && CFGetTypeID(raw) == CFURLGetTypeID()
                               ? badgeOps.path((__bridge NSURL *)raw)
                               : nil;
          if (raw)
            CFRelease(raw);
          if (path) {
            if (!nodes[path])
              nodes[path] = [NSMutableArray new];
            [nodes[path] addObject:(__bridge id)node];
          }
        } else if (!dockItem) {
          CFTypeRef children =
              badgeOps.copy(node, kAXChildrenAttribute, deadline);
          if (children && CFGetTypeID(children) == CFArrayGetTypeID()) {
            CFIndex n = CFArrayGetCount(children);
            for (CFIndex j = 0; j < n; j++) {
              if (queue.count >= 256) {
                capped = YES;
                break;
              }
              CFTypeRef child = CFArrayGetValueAtIndex(children, j);
              if (child && CFGetTypeID(child) == AXUIElementGetTypeID())
                [queue addObject:(__bridge id)child];
              else
                capped = YES;
            }
          } else
            capped = YES;
          if (children)
            CFRelease(children);
        }
      }
      complete =
          !capped && processed == queue.count && badgeCurrent(token, deadline);
    }
    NSMutableArray *entries = [NSMutableArray new];
    for (NSDictionary *t in targets) {
      NSMutableDictionary *entry = [@{
        @"itemKey" : t[@"itemKey"] ?: @"",
        @"targetRevision" : t[@"targetRevision"] ?: @0,
        @"state" : @"unavailable"
      } mutableCopy];
      NSArray *matches = nodes[t[@"path"]];
      if (complete && matches.count == 1 && badgeCurrent(token, deadline) &&
          badgeOps.target(t)) {
        NSDictionary *value = badgeReadValue(
            (__bridge AXUIElementRef)matches.firstObject, deadline, token);
        if (badgeCurrent(token, deadline) && badgeOps.target(t))
          [entry addEntriesFromDictionary:value];
      }
      [entries addObject:entry];
    }
    BOOL stable = dock && [dock isEqual:badgeOps.dock()] &&
                  badgeCurrent(token, deadline) && badgeOps.trusted();
    if (!stable || !complete) {
      for (NSMutableDictionary *entry in entries) {
        entry[@"state"] = @"unavailable";
        [entry removeObjectForKey:@"text"];
      }
    }
    NSDictionary *result = @{
      @"dock" : stable ? dock : @{},
      @"status" : stable && complete ? @"ready" : @"unavailable",
      @"entries" : entries
    };
    NSData *out = [NSJSONSerialization dataWithJSONObject:result
                                                  options:0
                                                    error:nil];
    return out ? strdup([[NSString alloc] initWithData:out
                                              encoding:NSUTF8StringEncoding]
                            .UTF8String)
               : NULL;
  }
}

// Private read-only running-app resolution. No AX badge or launch work here.
char *ot_launcher_badge_running_target(int pid, uint64_t seconds, uint64_t micros,
                                      const char *bundle, uintptr_t guard) {
  @autoreleasepool {
    if (!bundle || !goLauncherBadgeCurrent(guard)) return NULL;
    NSString *expectedBundle = [NSString stringWithUTF8String:bundle];
    NSDictionary *expected = @{ @"PID":@(pid), @"StartSeconds":@(seconds), @"StartMicros":@(micros) };
    if (![badgeProcess(pid) isEqual:expected]) return NULL;
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    if (!app || app.terminated || ![app.bundleIdentifier isEqual:expectedBundle]) return NULL;
    NSString *path = badgePath(app.bundleURL);
    if (!path) return NULL;
    NSDictionary *target = @{ @"process":expected, @"bundleID":expectedBundle, @"path":path };
    if (!badgeTarget(target) || !goLauncherBadgeCurrent(guard)) return NULL;
    NSData *data = [NSJSONSerialization dataWithJSONObject:target options:0 error:nil];
    return data ? strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String) : NULL;
  }
}
