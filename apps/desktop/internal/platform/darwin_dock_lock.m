//go:build darwin

#import "darwin_dock_lock.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <ColorSync/ColorSync.h>
#import <pthread.h>
#import <stdatomic.h>
#import <time.h>
#define LOCK_PREFIX 0x4f54000000000000ULL
#define LOCK_PREFIX_MASK 0xffff000000000000ULL
static atomic_uint_fast64_t nextDelivery;
static atomic_uint_fast64_t environment = 1;
static atomic_bool installed = false;
typedef struct {
  pthread_mutex_t mutex;
  OTLockSegment segments[128];
  int count, test;
  double expiry;
  uint64_t generation, bypass;
  atomic_uint_fast64_t physical, placement;
  atomic_bool placementCancelled, retired;
  int testMoves, transportOnly;
  atomic_int delivery, carrierSeen;
  uint64_t deliveryID, deliveryGeneration, deliveryPhysical, deliveryRequest;
  double deliveryDeadline, deliveryX, deliveryY, deliveryDX, deliveryDY;
  BOOL deliveryRestoring;
  CFMachPortRef tap;
  CFRunLoopSourceRef source;
  CFTypeRef notifications;
} LockOwner;
double ot_lock_now(void) {
  struct timespec t;
  clock_gettime(CLOCK_MONOTONIC, &t);
  return t.tv_sec + t.tv_nsec / 1e9;
}
uint64_t ot_lock_generation(void) { return atomic_load(&environment); }
static void displayChanged(CGDirectDisplayID display,
                           CGDisplayChangeSummaryFlags flags, void *context) {
  atomic_fetch_add(&environment, 1);
}
static NSDictionary *rectJSON(CGRect b) {
  return @{
    @"x" : @(b.origin.x),
    @"y" : @(b.origin.y),
    @"w" : @(b.size.width),
    @"h" : @(b.size.height)
  };
}
static char *json(id value) {
  if (!value)
    return NULL;
  NSData *data = [NSJSONSerialization dataWithJSONObject:value
                                                 options:0
                                                   error:NULL];
  return data ? strdup([[NSString alloc] initWithData:data
                                             encoding:NSUTF8StringEncoding]
                           .UTF8String)
              : NULL;
}
static NSArray *displays(void) {
  CGDirectDisplayID ids[32];
  uint32_t n = 0;
  if (CGGetActiveDisplayList(32, ids, &n) != kCGErrorSuccess || !n || n >= 32)
    return nil;
  NSMutableArray *result = [NSMutableArray array];
  NSArray *screenNames = [NSScreen screens];
  for (uint32_t i = 0; i < n; i++) {
    CFUUIDRef uuid = CGDisplayCreateUUIDFromDisplayID(ids[i]);
    if (!uuid)
      return nil;
    NSString *name = CFBridgingRelease(CFUUIDCreateString(NULL, uuid));
    CFRelease(uuid);
    CGRect b = CGDisplayBounds(ids[i]);
    double scale =
        b.size.width > 0 ? CGDisplayPixelsWide(ids[i]) / b.size.width : 0;
    NSString *displayName = [NSString stringWithFormat:@"Display %u", ids[i]];
    for (NSScreen *screen in screenNames) {
      if ([screen.deviceDescription[@"NSScreenNumber"] unsignedIntValue] ==
              ids[i] &&
          screen.localizedName.length) {
        displayName = screen.localizedName;
        break;
      }
    }
    [result addObject:@{
      @"uuid" : name,
      @"id" : @(ids[i]),
      @"name" : displayName,
      @"bounds" : rectJSON(b),
      @"scale" : @(scale),
      @"main" : (CGDisplayIsMain(ids[i]) ? @YES : @NO),
      @"mirrored" : (CGDisplayIsInMirrorSet(ids[i]) ? @YES : @NO)
    }];
  }
  return result;
}
char *ot_lock_displays(void) {
  @autoreleasepool {
    return json(displays());
  }
}
static CFTypeRef attribute(AXUIElementRef node, CFStringRef key,
                           double deadline) {
  double left = deadline - ot_lock_now();
  if (left <= 0)
    return NULL;
  AXUIElementSetMessagingTimeout(node, fmin(left, .02));
  CFTypeRef value = NULL;
  if (AXUIElementCopyAttributeValue(node, key, &value) != kAXErrorSuccess) {
    if (value)
      CFRelease(value);
    return NULL;
  }
  return value;
}
static CGRect bounds(AXUIElementRef node, double deadline) {
  CGPoint p;
  CGSize s;
  CFTypeRef a = attribute(node, kAXPositionAttribute, deadline),
            b = attribute(node, kAXSizeAttribute, deadline);
  BOOL ok = a && b && CFGetTypeID(a) == AXValueGetTypeID() &&
            CFGetTypeID(b) == AXValueGetTypeID() &&
            AXValueGetType(a) == kAXValueCGPointType &&
            AXValueGetType(b) == kAXValueCGSizeType &&
            AXValueGetValue(a, kAXValueCGPointType, &p) &&
            AXValueGetValue(b, kAXValueCGSizeType, &s);
  if (a)
    CFRelease(a);
  if (b)
    CFRelease(b);
  return ok ? CGRectMake(p.x, p.y, s.width, s.height) : CGRectZero;
}
static char *lockReadSnapshot(BOOL includeDisplays) {
  @autoreleasepool {
    uint64_t gen = ot_lock_generation();
    NSArray *screens = includeDisplays ? displays() : @[];
    NSMutableDictionary *out = [@{
      @"generation" : @(gen),
      @"displays" : screens ?: @[],
      @"pid" : @0,
      @"edge" : @"",
      @"reason" : @"Dock container unavailable"
    } mutableCopy];
    if (!screens) {
      out[@"reason"] = @"display inventory unavailable";
      return json(out);
    }
    if (!AXIsProcessTrusted()) {
      out[@"reason"] = @"Accessibility permission unavailable";
      return json(out);
    }
    NSArray *apps = [NSRunningApplication
        runningApplicationsWithBundleIdentifier:@"com.apple.dock"];
    if (apps.count != 1)
      return json(out);
    NSRunningApplication *app = apps[0];
    if (app.terminated)
      return json(out);
    pid_t pid = app.processIdentifier;
    out[@"pid"] = @(pid);
    id pref = CFBridgingRelease(CFPreferencesCopyAppValue(
        CFSTR("orientation"), CFSTR("com.apple.dock")));
    if (pref && (![pref isKindOfClass:NSString.class] ||
                 ![@[ @"bottom", @"left", @"right" ] containsObject:pref])) {
      out[@"reason"] = @"Dock orientation unavailable";
      return json(out);
    }
    if (pref)
      out[@"edge"] = pref;
    double deadline = ot_lock_now() + .12;
    AXUIElementRef root = AXUIElementCreateApplication(pid);
    NSMutableArray *queue = [NSMutableArray arrayWithObject:(__bridge id)root];
    CFRelease(root);
    CGRect container = CGRectZero;
    int found = 0;
    BOOL complete = YES;
    for (NSUInteger i = 0;
         i < queue.count && i < 64 && ot_lock_now() < deadline; i++) {
      AXUIElementRef node = (__bridge AXUIElementRef)queue[i];
      pid_t actual = 0;
      if (AXUIElementGetPid(node, &actual) != kAXErrorSuccess || actual != pid)
        continue;
      CFTypeRef r = attribute(node, kAXRoleAttribute, deadline),
                children = attribute(node, kAXChildrenAttribute, deadline);
      BOOL list =
          r && CFGetTypeID(r) == CFStringGetTypeID() && CFEqual(r, kAXListRole);
      if (r)
        CFRelease(r);
      if (children && CFGetTypeID(children) == CFArrayGetTypeID()) {
        CFArrayRef array = children;
        BOOL dockItems = NO;
        for (CFIndex j = 0;
             j < CFArrayGetCount(array) && j < 64 && ot_lock_now() < deadline;
             j++) {
          CFTypeRef child = CFArrayGetValueAtIndex(array, j);
          if (CFGetTypeID(child) != AXUIElementGetTypeID())
            continue;
          if (list) {
            CFTypeRef role =
                attribute((AXUIElementRef)child, kAXRoleAttribute, deadline);
            if (role && CFGetTypeID(role) == CFStringGetTypeID() &&
                CFEqual(role, kAXDockItemRole))
              dockItems = YES;
            if (role)
              CFRelease(role);
            if (dockItems)
              break;
          }
          if (queue.count < 64)
            [queue addObject:(__bridge id)child];
          else
            complete = NO;
        }
        if (list && dockItems) {
          container = bounds(node, deadline);
          found++;
        }
      }
      if (children)
        CFRelease(children);
    }
    if (ot_lock_now() >= deadline)
      complete = NO;
    if (complete && found == 1 && container.size.width > 0 &&
        container.size.height > 0 && gen == ot_lock_generation()) {
      out[@"container"] = rectJSON(container);
      out[@"reason"] = @"";
    }
    return json(out);
  }
}
char *ot_lock_snapshot(void *owner) { return lockReadSnapshot(YES); }
char *ot_lock_read_container(void) { return lockReadSnapshot(NO); }
int ot_lock_buttons(void) {
  for (unsigned i = 0; i < 32; i++)
    if (CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState,
                                 (CGMouseButton)i))
      return 1;
  return 0;
}
int ot_lock_bypass(uint64_t mask) {
  return (CGEventSourceFlagsState(kCGEventSourceStateCombinedSessionState) &
          mask) != 0;
}
static BOOL adjust(LockOwner *s, uint64_t generation, double now,
                   uint64_t flags, BOOL held, BOOL synthetic, CGPoint *p) {
  if (held || synthetic || atomic_load(&s->retired))
    return NO;
  BOOL changed = NO;
  pthread_mutex_lock(&s->mutex);
  if (!atomic_load(&s->retired) && s->generation == generation &&
      generation == ot_lock_generation() && now <= s->expiry &&
      !(flags & s->bypass)) {
    for (int i = 0; i < s->count; i++) {
      OTLockSegment b = s->segments[i];
      if (p->x >= b.x && p->x < b.x + b.w && p->y >= b.y && p->y < b.y + b.h) {
        p->x += b.dx;
        p->y += b.dy;
        changed = YES;
        break;
      }
    }
  }
  pthread_mutex_unlock(&s->mutex);
  return changed;
}
static BOOL placementCurrent(LockOwner *, uint64_t, uint64_t, uint64_t, BOOL);
static CGEventRef deliverCarrier(LockOwner *s, CGEventRef event) {
  atomic_fetch_add(&s->carrierSeen, 1);
  uint64_t id =
      (uint64_t)CGEventGetIntegerValueField(event, kCGEventSourceUserData);
  pthread_mutex_lock(&s->mutex);
  BOOL matches = id == s->deliveryID;
  BOOL valid = matches && ot_lock_now() <= s->deliveryDeadline &&
               placementCurrent(s, s->deliveryGeneration, s->deliveryPhysical,
                                s->deliveryRequest, s->deliveryRestoring);
  if (valid && !s->test)
    valid = !ot_lock_buttons() && !ot_lock_bypass(s->bypass);
  if (valid && !s->transportOnly) {
    CGEventSetType(event, kCGEventMouseMoved);
    CGEventSetLocation(event, CGPointMake(s->deliveryX, s->deliveryY));
    CGEventSetIntegerValueField(event, kCGMouseEventDeltaX,
                                llround(s->deliveryDX));
    CGEventSetIntegerValueField(event, kCGMouseEventDeltaY,
                                llround(s->deliveryDY));
  }
  if (matches)
    atomic_store(&s->delivery, valid ? 1 : -1);
  BOOL convert = valid && !s->transportOnly;
  pthread_mutex_unlock(&s->mutex);
  return convert ? event : NULL;
}
static CGEventRef callback(CGEventTapProxy proxy, CGEventType type,
                           CGEventRef event, void *context) {
  LockOwner *s = context;
  if (type == kCGEventTapDisabledByTimeout ||
      type == kCGEventTapDisabledByUserInput) {
    ot_lock_disable(s);
    if (s->tap)
      CGEventTapEnable(s->tap, true);
    return event;
  }
  if (!event)
    return event;
  if (((uint64_t)CGEventGetIntegerValueField(event, kCGEventSourceUserData) &
       LOCK_PREFIX_MASK) == LOCK_PREFIX)
    return type == kCGEventNull ? deliverCarrier(s, event) : NULL;
  BOOL synthetic =
      CGEventGetIntegerValueField(event, kCGEventSourceUnixProcessID) != 0;
  if (!synthetic)
    atomic_fetch_add(&s->physical, 1);
  if (type != kCGEventMouseMoved || synthetic)
    return event;
  CGPoint p = CGEventGetLocation(event);
  if (adjust(s, ot_lock_generation(), ot_lock_now(), CGEventGetFlags(event),
             ot_lock_buttons(), NO, &p))
    CGEventSetLocation(event, p);
  return event;
}
void *ot_lock_create(int test) {
  LockOwner *s = calloc(1, sizeof(*s));
  if (!s)
    return NULL;
  s->test = test;
  pthread_mutex_init(&s->mutex, NULL);
  atomic_init(&s->physical, 0);
  atomic_init(&s->placement, 0);
  atomic_init(&s->placementCancelled, false);
  atomic_init(&s->retired, false);
  atomic_init(&s->delivery, 0);
  atomic_init(&s->carrierSeen, 0);
  return s;
}
int ot_lock_start(void *owner) {
  LockOwner *s = owner;
  if (s->test)
    return 0;
  if (atomic_exchange(&installed, true))
    return -1;
  if (!AXIsProcessTrusted() || !CGPreflightListenEventAccess()) {
    atomic_store(&installed, false);
    return -2;
  }
  CGEventMask mask =
      CGEventMaskBit(kCGEventNull) | CGEventMaskBit(kCGEventMouseMoved) |
      CGEventMaskBit(kCGEventLeftMouseDown) |
      CGEventMaskBit(kCGEventLeftMouseUp) |
      CGEventMaskBit(kCGEventRightMouseDown) |
      CGEventMaskBit(kCGEventRightMouseUp) |
      CGEventMaskBit(kCGEventOtherMouseDown) |
      CGEventMaskBit(kCGEventOtherMouseUp) |
      CGEventMaskBit(kCGEventLeftMouseDragged) |
      CGEventMaskBit(kCGEventRightMouseDragged) |
      CGEventMaskBit(kCGEventOtherMouseDragged) |
      CGEventMaskBit(kCGEventScrollWheel) | CGEventMaskBit(kCGEventKeyDown) |
      CGEventMaskBit(kCGEventFlagsChanged);
  s->tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
                            kCGEventTapOptionDefault, mask, callback, s);
  if (!s->tap) {
    atomic_store(&installed, false);
    return -3;
  }
  s->source = CFMachPortCreateRunLoopSource(NULL, s->tap, 0);
  if (!s->source)
    return -3;
  CFRunLoopAddSource(CFRunLoopGetCurrent(), s->source, kCFRunLoopCommonModes);
  CGEventTapEnable(s->tap, true);
  CGDisplayRegisterReconfigurationCallback(displayChanged, NULL);
  atomic_fetch_add(&environment, 1);
  @autoreleasepool {
    NSMutableArray *tokens = [NSMutableArray array];
    for (NSString *name in @[
           NSWorkspaceDidLaunchApplicationNotification,
           NSWorkspaceDidTerminateApplicationNotification
         ]) {
      id token = [NSWorkspace.sharedWorkspace.notificationCenter
          addObserverForName:name
                      object:nil
                       queue:nil
                  usingBlock:^(NSNotification *n) {
                    NSRunningApplication *a =
                        n.userInfo[NSWorkspaceApplicationKey];
                    if ([a.bundleIdentifier isEqualToString:@"com.apple.dock"])
                      atomic_fetch_add(&environment, 1);
                  }];
      [tokens addObject:token];
    }
    s->notifications = CFBridgingRetain(tokens);
  }
  return 0;
}
void ot_lock_pump(void *owner) {
  CFRunLoopRunInMode(kCFRunLoopDefaultMode, .005, true);
}
void ot_lock_disable(void *owner) {
  LockOwner *s = owner;
  pthread_mutex_lock(&s->mutex);
  s->count = 0;
  s->expiry = 0;
  pthread_mutex_unlock(&s->mutex);
}
void ot_lock_publish(void *owner, uint64_t generation, double expiry,
                     uint64_t bypass, OTLockSegment *segments, int count) {
  LockOwner *s = owner;
  pthread_mutex_lock(&s->mutex);
  s->generation = generation;
  s->expiry = expiry;
  s->bypass = bypass;
  s->count =
      !atomic_load(&s->retired) && count >= 0 && count <= 128 ? count : 0;
  if (s->count)
    memcpy(s->segments, segments, sizeof(OTLockSegment) * s->count);
  pthread_mutex_unlock(&s->mutex);
}
int ot_lock_health(void *owner) {
  LockOwner *s = owner;
  if (s->test)
    return 0;
  return AXIsProcessTrusted() && CGPreflightListenEventAccess() && s->tap &&
                 CFMachPortIsValid(s->tap) && CGEventTapIsEnabled(s->tap)
             ? 0
             : -1;
}
void ot_lock_stop(void *owner) {
  LockOwner *s = owner;
  ot_lock_disable(s);
  if (s->source) {
    CFRunLoopRemoveSource(CFRunLoopGetCurrent(), s->source,
                          kCFRunLoopCommonModes);
    CFRelease(s->source);
    s->source = NULL;
  }
  if (s->tap) {
    CGEventTapEnable(s->tap, false);
    CFMachPortInvalidate(s->tap);
    CFRelease(s->tap);
    s->tap = NULL;
    CGDisplayRemoveReconfigurationCallback(displayChanged, NULL);
    atomic_store(&installed, false);
  }
  @autoreleasepool {
    if (s->notifications) {
      NSArray *tokens = CFBridgingRelease(s->notifications);
      s->notifications = NULL;
      for (id t in tokens)
        [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:t];
    }
  }
}
void ot_lock_destroy(void *owner) {
  LockOwner *s = owner;
  if (s) {
    pthread_mutex_destroy(&s->mutex);
    free(s);
  }
}
uint64_t ot_lock_physical(void *owner) {
  return atomic_load(&((LockOwner *)owner)->physical);
}
void ot_lock_pointer(double *x, double *y) {
  CGEventRef e = CGEventCreate(NULL);
  CGPoint p = e ? CGEventGetLocation(e) : CGPointMake(NAN, NAN);
  if (e)
    CFRelease(e);
  *x = p.x;
  *y = p.y;
}

void ot_lock_begin_placement(void *owner, uint64_t request) {
  LockOwner *s = owner;
  atomic_store(&s->placementCancelled, false);
  atomic_store(&s->placement, request);
}
void ot_lock_cancel_placement(void *owner, uint64_t request) {
  LockOwner *s = owner;
  if (atomic_load(&s->placement) == request)
    atomic_store(&s->placementCancelled, true);
}
static BOOL placementCurrent(LockOwner *s, uint64_t generation,
                             uint64_t physical, uint64_t request,
                             BOOL restoring) {
  return request && (restoring || !atomic_load(&s->retired)) &&
         atomic_load(&s->placement) == request &&
         (restoring || !atomic_load(&s->placementCancelled)) &&
         generation == ot_lock_generation() && physical == ot_lock_physical(s);
}

static int placementMove(void *owner, uint64_t generation, uint64_t physical,
                         uint64_t request, double x, double y, double dx,
                         double dy, BOOL restoring) {
  LockOwner *s = owner;
  if (!placementCurrent(s, generation, physical, request, restoring) ||
      !isfinite(x) || !isfinite(y) || !isfinite(dx) || !isfinite(dy))
    return 0;
  if (!s->test && (ot_lock_buttons() || ot_lock_bypass(s->bypass) ||
                   ot_lock_health(s) != 0))
    return 0;
  CGEventRef event = CGEventCreate(NULL);
  if (!event)
    return 0;
  CGEventSetType(event, kCGEventNull);
  uint64_t id = LOCK_PREFIX | (atomic_fetch_add(&nextDelivery, 1) + 1);
  CGEventSetIntegerValueField(event, kCGEventSourceUserData, (int64_t)id);
  pthread_mutex_lock(&s->mutex);
  s->deliveryID = id;
  s->deliveryGeneration = generation;
  s->deliveryPhysical = physical;
  s->deliveryRequest = request;
  s->deliveryRestoring = restoring;
  s->deliveryDeadline = ot_lock_now() + .15;
  s->deliveryX = x;
  s->deliveryY = y;
  s->deliveryDX = dx;
  s->deliveryDY = dy;
  atomic_store(&s->delivery, 0);
  pthread_mutex_unlock(&s->mutex);
  if (s->test) {
    callback(NULL, kCGEventNull, event, s);
  } else if (placementCurrent(s, generation, physical, request, restoring)) {
    CGEventPost(kCGSessionEventTap, event);
  }
  CFRelease(event);
  double deadline = ot_lock_now() + .15;
  while (atomic_load(&s->delivery) == 0 && ot_lock_now() < deadline &&
         placementCurrent(s, generation, physical, request, restoring)) {
    struct timespec pause = {.tv_nsec = 1000000};
    nanosleep(&pause, NULL);
  }
  BOOL delivered = atomic_load(&s->delivery) == 1;
  pthread_mutex_lock(&s->mutex);
  s->deliveryID = 0;
  pthread_mutex_unlock(&s->mutex);
  if (delivered && s->test)
    s->testMoves++;
  return delivered;
}
int ot_lock_move(void *o, uint64_t g, uint64_t p, uint64_t r, double x,
                 double y, double dx, double dy) {
  return placementMove(o, g, p, r, x, y, dx, dy, NO);
}
int ot_lock_restore(void *o, uint64_t g, uint64_t p, uint64_t r, double x,
                    double y) {
  return placementMove(o, g, p, r, x, y, 0, 0, YES);
}
uint64_t ot_lock_probe(void) {
  LockOwner *s = ot_lock_create(1);
  uint64_t g = ot_lock_generation(), result = 0;
  OTLockSegment segment = {0, 97, 100, 4, 0, -6};
  ot_lock_publish(s, g, 20, kCGEventFlagMaskAlternate, &segment, 1);
  CGPoint p = CGPointMake(20, 99);
  if (adjust(s, g, 10, 0, NO, NO, &p) && p.y == 93)
    result |= 1;
  p = CGPointMake(20, 99);
  if (!adjust(s, g, 21, 0, NO, NO, &p))
    result |= 2;
  if (!adjust(s, g, 10, kCGEventFlagMaskAlternate, NO, NO, &p))
    result |= 4;
  if (!adjust(s, g, 10, 0, YES, NO, &p) && !adjust(s, g, 10, 0, NO, YES, &p))
    result |= 8;
  ot_lock_disable(s);
  if (!adjust(s, g, 10, 0, NO, NO, &p))
    result |= 16;
  ot_lock_destroy(s);
  return result;
}

uint64_t ot_lock_placement_probe(void) {
  LockOwner *s = ot_lock_create(1);
  uint64_t g = ot_lock_generation(), result = 0;
  ot_lock_begin_placement(s, 1);
  if (ot_lock_move(s, g, 0, 1, 10, 20, 0, 3))
    result |= 1;
  ot_lock_cancel_placement(s, 1);
  if (!ot_lock_move(s, g, 0, 1, 10, 20, 0, 3) &&
      ot_lock_restore(s, g, 0, 1, 0, 0))
    result |= 2;
  atomic_fetch_add(&s->physical, 1);
  if (!ot_lock_restore(s, g, 0, 1, 0, 0))
    result |= 4;
  if (!ot_lock_move(s, g + 1, 1, 1, 0, 0, 0, 0))
    result |= 8;
  ot_lock_begin_placement(s, 2);
  if (!ot_lock_restore(s, g, 1, 1, 0, 0) && s->testMoves == 2)
    result |= 16;
  ot_lock_destroy(s);
  return result;
}

uint64_t ot_lock_advance_environment(void) {
  return atomic_fetch_add(&environment, 1) + 1;
}

int ot_lock_test_moves(void *o) {
  LockOwner *s = o;
  return s->test ? s->testMoves : 0;
}

void ot_lock_retire(void *owner) {
  LockOwner *s = owner;
  atomic_store(&s->retired, true);
  ot_lock_disable(owner);
}

void ot_lock_transport_only(void *owner) {
  ((LockOwner *)owner)->transportOnly = 1;
}
int ot_lock_foreground(void) {
  @autoreleasepool {
    return NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
  }
}
uint64_t ot_lock_delivery_probe(void) {
  LockOwner *s = ot_lock_create(1);
  uint64_t g = ot_lock_generation(), result = 0;
  ot_lock_begin_placement(s, 1);
  CGEventRef e = CGEventCreate(NULL);
  CGEventSetType(e, kCGEventNull);
  uint64_t id = LOCK_PREFIX | 42;
  CGEventSetIntegerValueField(e, kCGEventSourceUserData, id);
  s->deliveryID = id;
  s->deliveryGeneration = g;
  s->deliveryPhysical = 0;
  s->deliveryRequest = 1;
  s->deliveryDeadline = ot_lock_now() + 1;
  s->deliveryX = 20;
  s->deliveryY = 99;
  s->deliveryDY = 3;
  if (callback(NULL, kCGEventNull, e, s) == e &&
      CGEventGetType(e) == kCGEventMouseMoved &&
      CGEventGetIntegerValueField(e, kCGMouseEventDeltaY) == 3)
    result |= 1;
  CGEventSetType(e, kCGEventNull);
  atomic_fetch_add(&s->physical, 1);
  if (callback(NULL, kCGEventNull, e, s) == NULL &&
      CGEventGetType(e) == kCGEventNull)
    result |= 2;
  s->deliveryPhysical = 1;
  ot_lock_cancel_placement(s, 1);
  if (callback(NULL, kCGEventNull, e, s) == NULL)
    result |= 4;
  s->deliveryRestoring = YES;
  if (callback(NULL, kCGEventNull, e, s) == e)
    result |= 8;
  CGEventSetType(e, kCGEventNull);
  s->deliveryGeneration = g + 1;
  if (callback(NULL, kCGEventNull, e, s) == NULL)
    result |= 16;
  CFRelease(e);
  ot_lock_destroy(s);
  return result;
}

int ot_lock_carriers_seen(void *o) {
  return atomic_load(&((LockOwner *)o)->carrierSeen);
}
