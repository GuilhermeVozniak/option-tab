#import "../../darwin_launcher_badges.m"
#include <assert.h>
static BOOL current = YES, advertised = YES;
static CFTypeRef fixtureValue;
int goLauncherBadgeCurrent(uintptr_t token) { return token == 1 && current; }
static CFArrayRef fixtureNames(AXUIElementRef n, CFAbsoluteTime d) {
  (void)n;
  (void)d;
  return CFBridgingRetain(advertised ? @[ @"AXStatusLabel" ] : @[]);
}
static CFTypeRef fixtureCopy(AXUIElementRef n, CFStringRef name,
                             CFAbsoluteTime d) {
  (void)n;
  (void)d;
  assert(CFEqual(name, CFSTR("AXStatusLabel")));
  return fixtureValue ? CFRetain(fixtureValue) : NULL;
}

static AXUIElementRef fixtureRoot, fixtureItem;
static BOOL duplicate, changed, refChanged;
static int dockReads, targetReads;
static BOOL trustedFixture(void) { return YES; }
static NSDictionary *dockFixture(void) {
  dockReads++;
  return @{
    @"PID" : @42,
    @"StartSeconds" : @(changed && dockReads > 1 ? 2 : 1),
    @"StartMicros" : @0
  };
}
static BOOL targetFixture(NSDictionary *t) {
  (void)t;
  targetReads++;
  return !(refChanged && targetReads > 1);
}
static AXUIElementRef rootFixture(pid_t pid) {
  assert(pid == 42);
  return (AXUIElementRef)CFRetain(fixtureRoot);
}
static NSString *pathFixture(NSURL *url) { return url.path; }
static CFTypeRef treeCopy(AXUIElementRef node, CFStringRef name,
                          CFAbsoluteTime deadline) {
  if (CFEqual(name, kAXRoleAttribute))
    return CFRetain(node == fixtureRoot ? kAXApplicationRole : kAXDockItemRole);
  if (CFEqual(name, kAXSubroleAttribute))
    return node == fixtureRoot ? NULL : CFRetain(kAXApplicationDockItemSubrole);
  if (CFEqual(name, kAXChildrenAttribute)) {
    NSArray *items =
        duplicate ? @[ (__bridge id)fixtureItem, (__bridge id)fixtureItem ]
                  : @[ (__bridge id)fixtureItem ];
    return CFBridgingRetain(items);
  }
  if (CFEqual(name, kAXURLAttribute))
    return CFBridgingRetain([NSURL fileURLWithPath:@"/fixture/Fixture.app"]);
  return fixtureCopy(node, name, deadline);
}
static NSDictionary *fullRead(void) {
  dockReads = targetReads = 0;
  char *raw =
      ot_launcher_badges_read("[{\"itemKey\":\"pin:a\",\"targetRevision\":1,"
                              "\"path\":\"/fixture/Fixture.app\"}]",
                              1);
  assert(raw);
  NSData *data = [[NSString stringWithUTF8String:raw]
      dataUsingEncoding:NSUTF8StringEncoding];
  free(raw);
  return [NSJSONSerialization JSONObjectWithData:data options:0 error:nil];
}
int main() {
  @autoreleasepool {
    badgeOps.names = fixtureNames;
    badgeOps.copy = fixtureCopy;
    CFAbsoluteTime deadline = CFAbsoluteTimeGetCurrent() + 1;
    NSDictionary *value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"state"] isEqual:@"unavailable"]);
    assert(!value[@"text"]);
    advertised = NO;
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"state"] isEqual:@"unsupported"]);
    advertised = YES;
    fixtureValue = CFSTR("");
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"state"] isEqual:@"known"] && [value[@"text"] isEqual:@""]);
    fixtureValue = CFSTR("0");
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"text"] isEqual:@"0"]);
    fixtureValue = kCFBooleanTrue;
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"state"] isEqual:@"unavailable"]);
    fixtureValue = CFSTR("1234567890123456789012345678901234567890");
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"text"] isEqual:@"*"]);
    current = NO;
    value = badgeReadValue(NULL, deadline, 1);
    assert([value[@"state"] isEqual:@"unavailable"]);

    current = YES;
    fixtureRoot = AXUIElementCreateApplication(42);
    fixtureItem = AXUIElementCreateApplication(42);
    badgeOps =
        (OTBadgeOps){trustedFixture, dockFixture, targetFixture, treeCopy,
                     fixtureNames,   rootFixture, pathFixture};
    fixtureValue = CFSTR("7");
    NSDictionary *full = fullRead();
    assert([full[@"status"] isEqual:@"ready"]);
    assert([full[@"entries"][0][@"state"] isEqual:@"known"]);
    duplicate = YES;
    full = fullRead();
    assert([full[@"entries"][0][@"state"] isEqual:@"unavailable"]);
    duplicate = NO;
    changed = YES;
    full = fullRead();
    assert([full[@"status"] isEqual:@"unavailable"]);
    assert(!full[@"entries"][0][@"text"]);
    changed = NO;
    refChanged = YES;
    full = fullRead();
    assert([full[@"entries"][0][@"state"] isEqual:@"unavailable"]);
    refChanged = NO;
    NSMutableArray *targets = [NSMutableArray new];
    for (int i = 0; i < 256; i++)
      [targets addObject:@{@"itemKey":[NSString stringWithFormat:@"item-%d",i],
                           @"targetRevision":@1,@"path":@"/fixture/Fixture.app"}];
    NSData *encoded = [NSJSONSerialization dataWithJSONObject:targets options:0 error:nil];
    NSString *input = [[NSString alloc] initWithData:encoded encoding:NSUTF8StringEncoding];
    char *raw = ot_launcher_badges_read(input.UTF8String,1);
    assert(raw); free(raw);
    [targets addObject:targets.firstObject];
    encoded = [NSJSONSerialization dataWithJSONObject:targets options:0 error:nil];
    input = [[NSString alloc] initWithData:encoded encoding:NSUTF8StringEncoding];
    assert(!ot_launcher_badges_read(input.UTF8String,1));
    CFRelease(fixtureRoot);
    CFRelease(fixtureItem);
    puts("injected badge extraction PASS");
  }
  return 0;
}
