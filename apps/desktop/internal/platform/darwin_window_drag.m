// go:build darwin
#import "darwin_window_drag.h"
#import "darwin_retirement.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#include <pthread.h>
#include <stdatomic.h>
#include <time.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef, CGWindowID *);

typedef struct {
  CFMachPortRef tap;
  CFRunLoopSourceRef source;
  OTWindowDragRaw queue[64];
  size_t head, count;
  uint64_t generation, gesture;
  int test;
} DragOwner;
static atomic_uint_fast64_t nextGeneration, nextGesture, nextSequence,
    activeGeneration, activeGesture;
static atomic_bool physicalDown;
static atomic_flag installed = ATOMIC_FLAG_INIT;
static pthread_mutex_t sampleMutex = PTHREAD_MUTEX_INITIALIZER;
typedef struct {
  AXUIElementRef root;
  OTWindowDragSample sample;
  uint64_t sec, usec;
  double discontinuity;
  BOOL correlated;
} DragEvidence;
static DragEvidence evidence;
static uint64_t evidenceEpoch;

double ot_window_drag_now(void) {
  struct timespec t;
  clock_gettime(CLOCK_MONOTONIC, &t);
  return t.tv_sec + t.tv_nsec / 1e9;
}
static BOOL liveRaw(OTWindowDragRaw r) {
  return r.generation && r.gesture &&
         atomic_load(&activeGeneration) == r.generation &&
         atomic_load(&activeGesture) == r.gesture && atomic_load(&physicalDown);
}
static void invalidateDrag(void) {
  atomic_store(&physicalDown, false);
  atomic_store(&activeGesture, 0);
}
static void queueDrag(DragOwner *s, OTWindowDragRaw r) {
  if (s->count == 64) {
    s->head = 0;
    s->count = 0;
    r.kind = 4;
    invalidateDrag();
  }
  s->queue[(s->head + s->count) % 64] = r;
  s->count++;
}
static CGEventRef dragCallback(CGEventTapProxy proxy, CGEventType type,
                               CGEventRef event, void *context) {
  (void)proxy;
  DragOwner *s = context;
  OTWindowDragRaw r = {.generation = s->generation,
                       .gesture = s->gesture,
                       .sequence = atomic_fetch_add(&nextSequence, 1) + 1,
                       .at = ot_window_drag_now()};
  if (type == kCGEventTapDisabledByTimeout ||
      type == kCGEventTapDisabledByUserInput) {
    r.kind = 4;
    invalidateDrag();
    queueDrag(s, r);
    if (s->tap)
      CGEventTapEnable(s->tap, true);
    return event;
  }
  if (!event)
    return event;
  CGPoint p = CGEventGetLocation(event);
  r.x = p.x;
  r.y = p.y;
  if (type == kCGEventLeftMouseDown) {
    s->gesture = atomic_fetch_add(&nextGesture, 1) + 1;
    r.gesture = s->gesture;
    r.kind = 1;
    atomic_store(&activeGesture, s->gesture);
    atomic_store(&physicalDown, true);
  } else if (type == kCGEventLeftMouseDragged && atomic_load(&physicalDown)) {
    r.kind = 2;
  } else if (type == kCGEventLeftMouseUp) {
    r.kind = 3;
    invalidateDrag();
  } else
    return event;
  queueDrag(s, r);
  return event; // Listen-only: never suppress, replay, AX-query or call Go.
}
void *ot_window_drag_create(int test) {
  if (atomic_flag_test_and_set(&installed))
    return NULL;
  DragOwner *s = calloc(1, sizeof(*s));
  s->test = test;
  s->generation = atomic_fetch_add(&nextGeneration, 1) + 1;
  atomic_store(&activeGeneration, s->generation);
  invalidateDrag();
  if (!test) {
    CGEventMask mask = CGEventMaskBit(kCGEventLeftMouseDown) |
                       CGEventMaskBit(kCGEventLeftMouseDragged) |
                       CGEventMaskBit(kCGEventLeftMouseUp);
    s->tap =
        CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap,
                         kCGEventTapOptionListenOnly, mask, dragCallback, s);
    if (!s->tap) {
      atomic_store(&activeGeneration, 0);
      free(s);
      atomic_flag_clear(&installed);
      return NULL;
    }
    s->source = CFMachPortCreateRunLoopSource(NULL, s->tap, 0);
    if (!s->source) {
      CFRelease(s->tap);
      atomic_store(&activeGeneration, 0);
      free(s);
      atomic_flag_clear(&installed);
      return NULL;
    }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), s->source, kCFRunLoopCommonModes);
    CGEventTapEnable(s->tap, true);
  }
  return s;
}
void ot_window_drag_pump(void *owner) {
  DragOwner *s = owner;
  if (s && !s->test)
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.005, true);
  else {
    struct timespec delay = {.tv_nsec = 1000000};
    nanosleep(&delay, NULL);
  }
}
int ot_window_drag_pop(void *owner, OTWindowDragRaw *out) {
  DragOwner *s = owner;
  if (!s || !s->count)
    return 0;
  *out = s->queue[s->head];
  s->head = (s->head + 1) % 64;
  s->count--;
  return 1;
}
void ot_window_drag_stop(void *owner) {
  DragOwner *s = owner;
  if (!s)
    return;
  invalidateDrag();
  atomic_store(&activeGeneration, 0);
  if (s->tap) {
    CGEventTapEnable(s->tap, false);
    CFMachPortInvalidate(s->tap);
  }
  if (s->source) {
    CFRunLoopRemoveSource(CFRunLoopGetCurrent(), s->source,
                          kCFRunLoopCommonModes);
    CFRelease(s->source);
  }
  if (s->tap)
    CFRelease(s->tap);
  free(s);
  atomic_flag_clear(&installed);
}
void ot_window_drag_clear(uint64_t generation) {
  if (!generation)
    return;
  pthread_mutex_lock(&sampleMutex);
  if (evidence.sample.raw.generation != generation) {
    pthread_mutex_unlock(&sampleMutex);
    return;
  }
  evidenceEpoch++;
  if (evidence.root)
    CFRelease(evidence.root);
  memset(&evidence, 0, sizeof(evidence));
  pthread_mutex_unlock(&sampleMutex);
}
static CFTypeRef attr(AXUIElementRef node, CFStringRef key, double deadline,
                      AXError *status) {
  if (!node || ot_window_drag_now() >= deadline) {
    if (status)
      *status = kAXErrorCannotComplete;
    return NULL;
  }
  AXUIElementSetMessagingTimeout(node,
                                 fmin(0.025, deadline - ot_window_drag_now()));
  CFTypeRef v = NULL;
  AXError e = AXUIElementCopyAttributeValue(node, key, &v);
  if (status)
    *status = e;
  return v;
}
static BOOL stringAttr(AXUIElementRef node, CFStringRef key, char *out,
                       size_t size, double deadline) {
  CFTypeRef v = attr(node, key, deadline, NULL);
  BOOL ok = v && CFGetTypeID(v) == CFStringGetTypeID() &&
            CFStringGetCString(v, out, size, kCFStringEncodingUTF8);
  if (v)
    CFRelease(v);
  return ok;
}
static BOOL pointAttr(AXUIElementRef root, CFStringRef key, AXValueType type,
                      void *out, double deadline) {
  CFTypeRef v = attr(root, key, deadline, NULL);
  BOOL ok = v && CFGetTypeID(v) == AXValueGetTypeID() &&
            AXValueGetType(v) == type && AXValueGetValue(v, type, out);
  if (v)
    CFRelease(v);
  return ok;
}
static AXUIElementRef copyExactRoot(uint32_t wid, int pid, double deadline) {
  AXUIElementRef app = AXUIElementCreateApplication(pid);
  CFTypeRef roots = attr(app, kAXWindowsAttribute, deadline, NULL);
  CFRelease(app);
  AXUIElementRef found = NULL;
  if (roots && CFGetTypeID(roots) == CFArrayGetTypeID())
    for (CFIndex i = 0; i < CFArrayGetCount(roots) && i < 128 &&
                        ot_window_drag_now() < deadline;
         i++) {
      AXUIElementRef root = (AXUIElementRef)CFArrayGetValueAtIndex(roots, i);
      if (CFGetTypeID(root) != AXUIElementGetTypeID())
        continue;
      AXUIElementSetMessagingTimeout(root, 0.015);
      CGWindowID id = 0;
      if (_AXUIElementGetWindow(root, &id) == kAXErrorSuccess && id == wid) {
        found = (AXUIElementRef)CFRetain(root);
        break;
      }
    }
  if (roots)
    CFRelease(roots);
  return found;
}
static BOOL correlated(double dx, double dy, double px, double py,
                       BOOL remainder) {
  if (!isfinite(dx) || !isfinite(dy) || !isfinite(px) || !isfinite(py))
    return NO;
  if (remainder && fabs(dx) <= 2 && fabs(dy) <= 2 && fabs(px) <= 2 &&
      fabs(py) <= 2)
    return YES;
  return dx != 0 && px * dx > 0 && fabs(dy) <= fabs(dx) &&
         fabs(py) <= fabs(px) && fabs(dx - px) <= fmax(2, fabs(dx) * .2) &&
         fabs(dy - py) <= fmax(2, fabs(dy) * .2);
}
// Keep the resolved root across the OS drag threshold. Every actual AX
// position reaches the pure recognizer; discontinuities reset action evidence.
static BOOL advanceEvidence(DragEvidence *state, OTWindowDragRaw raw,
                            CGPoint pos) {
  BOOL continues =
      correlated(pos.x - state->sample.wx, pos.y - state->sample.wy,
                 raw.x - state->sample.raw.x, raw.y - state->sample.raw.y, NO);
  if (!continues)
    state->discontinuity = raw.at;
  state->correlated = continues;
  state->sample.raw = raw;
  state->sample.wx = pos.x;
  state->sample.wy = pos.y;
  return continues;
}
static BOOL intentAfterDiscontinuity(DragEvidence state, double observed) {
  return state.correlated && observed > state.discontinuity;
}
static AXUIElementRef resolvePoint(OTWindowDragRaw r, uint32_t *wid, int *pid,
                                   double deadline) {
  AXUIElementRef system = AXUIElementCreateSystemWide(), node = NULL;
  AXUIElementSetMessagingTimeout(system, .05);
  AXUIElementCopyElementAtPosition(system, r.x, r.y, &node);
  CFRelease(system);
  AXUIElementRef root = NULL;
  for (int depth = 0; node && depth < 8 && ot_window_drag_now() < deadline;
       depth++) {
    char role[64] = {0};
    stringAttr(node, kAXRoleAttribute, role, sizeof(role), deadline);
    if (!strcmp(role, "AXWindow")) {
      root = node;
      node = NULL;
      break;
    }
    CFTypeRef win = attr(node, kAXWindowAttribute, deadline, NULL);
    if (win && CFGetTypeID(win) == AXUIElementGetTypeID()) {
      CFRelease(node);
      node = (AXUIElementRef)win;
      continue;
    }
    if (win)
      CFRelease(win);
    CFTypeRef parent = attr(node, kAXParentAttribute, deadline, NULL);
    CFRelease(node);
    node = NULL;
    if (parent && CFGetTypeID(parent) == AXUIElementGetTypeID())
      node = (AXUIElementRef)parent;
    else if (parent)
      CFRelease(parent);
  }
  if (node)
    CFRelease(node);
  if (!root)
    return NULL;
  CGWindowID id = 0;
  pid_t owner = 0;
  if (_AXUIElementGetWindow(root, &id) != kAXErrorSuccess || !id ||
      AXUIElementGetPid(root, &owner) != kAXErrorSuccess || owner <= 0) {
    CFRelease(root);
    return NULL;
  }
  AXUIElementRef exact = copyExactRoot(id, owner, deadline);
  BOOL same = exact && CFEqual(exact, root);
  if (exact)
    CFRelease(exact);
  if (!same) {
    CFRelease(root);
    return NULL;
  }
  *wid = id;
  *pid = owner;
  return root;
}
int ot_window_drag_sample(OTWindowDragRaw raw, OTWindowDragSample *out) {
  @autoreleasepool {
    memset(out, 0, sizeof(*out));
    out->raw = raw;
    if (raw.kind == 3 || raw.kind == 4) {
      ot_window_drag_clear(raw.generation);
      return 1;
    }
    if (!liveRaw(raw) || ot_window_drag_now() - raw.at > .15) {
      out->raw.kind = 4;
      ot_window_drag_clear(raw.generation);
      return 1;
    }
    double deadline = ot_window_drag_now() + .12;
    if (raw.kind == 1) {
      ot_window_drag_clear(raw.generation);
      uint32_t wid = 0;
      int pid = 0;
      AXUIElementRef root = resolvePoint(raw, &wid, &pid, deadline);
      CGPoint pos;
      uint64_t sec = 0, usec = 0;
      int owner = 0;
      BOOL valid = root &&
                   stringAttr(root, kAXRoleAttribute, out->role,
                              sizeof(out->role), deadline) &&
                   stringAttr(root, kAXSubroleAttribute, out->subrole,
                              sizeof(out->subrole), deadline) &&
                   !strcmp(out->role, "AXWindow") &&
                   !strcmp(out->subrole, "AXStandardWindow") &&
                   pointAttr(root, kAXPositionAttribute, kAXValueCGPointType,
                             &pos, deadline) &&
                   ot_retirement_identity(wid, &owner, &sec, &usec) &&
                   owner == pid && liveRaw(raw);
      if (!valid) {
        if (root)
          CFRelease(root);
        out->raw.kind = 4;
        return 1;
      }
      out->window = wid;
      out->app = pid;
      out->wx = pos.x;
      out->wy = pos.y;
      pthread_mutex_lock(&sampleMutex);
      if (evidence.root)
        CFRelease(evidence.root);
      evidenceEpoch++;
      evidence = (DragEvidence){.root = root,
                                .sample = *out,
                                .sec = sec,
                                .usec = usec,
                                .discontinuity = raw.at};
      pthread_mutex_unlock(&sampleMutex);
      return 1;
    }
    pthread_mutex_lock(&sampleMutex);
    DragEvidence old = evidence;
    if (old.root)
      CFRetain(old.root);
    pthread_mutex_unlock(&sampleMutex);
    if (!old.root || old.sample.raw.generation != raw.generation ||
        old.sample.raw.gesture != raw.gesture) {
      if (old.root)
        CFRelease(old.root);
      return 0;
    }
    CGPoint pos;
    BOOL valid = pointAttr(old.root, kAXPositionAttribute, kAXValueCGPointType,
                           &pos, deadline) &&
                 ot_retirement_matches(old.sample.window, old.sample.app,
                                       old.sec, old.usec) &&
                 liveRaw(raw);
    CFRelease(old.root);
    if (!valid) {
      out->raw.kind = 4;
      ot_window_drag_clear(raw.generation);
      return 1;
    }
    BOOL continues = advanceEvidence(&old, raw, pos);
    *out = old.sample;
    pthread_mutex_lock(&sampleMutex);
    if (evidence.root && liveRaw(raw)) {
      evidence.sample = *out;
      evidence.discontinuity = old.discontinuity;
      evidence.correlated = old.correlated;
      if (!continues)
        evidenceEpoch++;
    } else
      out->raw.kind = 4;
    pthread_mutex_unlock(&sampleMutex);
    return 1;
  }
}
int ot_window_drag_current(uint64_t gen, uint64_t gesture, uint32_t wid,
                           int pid, double x, double y, double observed) {
  @autoreleasepool {
    if (!isfinite(x) || !isfinite(y) || !isfinite(observed))
      return 0;
    pthread_mutex_lock(&sampleMutex);
    DragEvidence old = evidence;
    uint64_t epoch = evidenceEpoch;
    if (old.root)
      CFRetain(old.root);
    pthread_mutex_unlock(&sampleMutex);
    if (!old.root)
      return 0;
    OTWindowDragRaw raw = old.sample.raw;
    double now = ot_window_drag_now();
    BOOL valid = liveRaw(raw) && intentAfterDiscontinuity(old, observed) &&
                 raw.kind == 2 && raw.generation == gen &&
                 raw.gesture == gesture && old.sample.window == wid &&
                 old.sample.app == pid && observed <= raw.at + .003 &&
                 now - raw.at <= .15 && observed <= now &&
                 ot_retirement_matches(wid, pid, old.sec, old.usec);
    CGPoint pos;
    CGEventRef pointer = NULL;
    if (valid) {
      AXUIElementRef fresh = copyExactRoot(wid, pid, now + .07);
      char role[64] = {0}, subrole[64] = {0};
      valid =
          fresh && CFEqual(fresh, old.root) &&
          stringAttr(fresh, kAXRoleAttribute, role, sizeof(role), now + .09) &&
          stringAttr(fresh, kAXSubroleAttribute, subrole, sizeof(subrole),
                     now + .09) &&
          !strcmp(role, "AXWindow") && !strcmp(subrole, "AXStandardWindow") &&
          pointAttr(fresh, kAXPositionAttribute, kAXValueCGPointType, &pos,
                    now + .1);
      if (fresh)
        CFRelease(fresh);
      pointer = CGEventCreate(NULL);
      valid = valid && pointer;
      if (valid) {
        CGPoint p = CGEventGetLocation(pointer);
        valid = correlated(pos.x - old.sample.wx, pos.y - old.sample.wy,
                           p.x - raw.x, p.y - raw.y, YES) &&
                CGEventSourceButtonState(kCGEventSourceStateHIDSystemState,
                                         kCGMouseButtonLeft);
      }
    }
    if (pointer)
      CFRelease(pointer);
    CFRelease(old.root);
    pthread_mutex_lock(&sampleMutex);
    BOOL unchanged = epoch == evidenceEpoch && evidence.root;
    pthread_mutex_unlock(&sampleMutex);
    return valid && unchanged && liveRaw(raw) &&
           ot_window_drag_now() - raw.at <= .15;
  }
}
static BOOL selfApp(int pid) {
  if (pid == getpid())
    return YES;
  NSString *bundle =
      [NSRunningApplication runningApplicationWithProcessIdentifier:pid]
          .bundleIdentifier;
  NSString *own = NSBundle.mainBundle.bundleIdentifier;
  return [bundle isEqualToString:@"com.optiontab.app"] ||
         (own.length && [bundle isEqualToString:own]);
}
static BOOL boolAttr(AXUIElementRef node, CFStringRef key, BOOL *out,
                     double deadline) {
  CFTypeRef v = attr(node, key, deadline, NULL);
  BOOL ok = v && CFGetTypeID(v) == CFBooleanGetTypeID();
  if (ok)
    *out = CFBooleanGetValue(v);
  if (v)
    CFRelease(v);
  return ok;
}
// Traverse a bounded complete child tree. Exhaustion/refusal is unknown, not a
// statement that no modal descendants exist.
static BOOL relationshipLeaf(const char *role) {
  const char *known[] = {
      "AXButton",        "AXStaticText", "AXImage",
      "AXTextField",     "AXTextArea",   "AXCheckBox",
      "AXRadioButton",   "AXSlider",     "AXProgressIndicator",
      "AXValueIndicator"};
  for (size_t i = 0; i < sizeof(known) / sizeof(known[0]); i++)
    if (!strcmp(role, known[i]))
      return YES;
  return NO;
}
static BOOL inspectChildren(AXUIElementRef node, int depth, int *budget,
                            double deadline, OTActionWindowRole *out) {
  if (depth > 8 || --*budget < 0 || ot_window_drag_now() >= deadline)
    return NO;
  AXError error;
  CFTypeRef children = attr(node, kAXChildrenAttribute, deadline, &error);
  if (error == kAXErrorNoValue || error == kAXErrorAttributeUnsupported) {
    if (children)
      CFRelease(children);
    if (!depth)
      return NO;
    if (error == kAXErrorNoValue)
      return YES;
    char role[64] = {0};
    return stringAttr(node, kAXRoleAttribute, role, sizeof(role), deadline) &&
           relationshipLeaf(role);
  }
  if (!children || CFGetTypeID(children) != CFArrayGetTypeID()) {
    if (children)
      CFRelease(children);
    return NO;
  }
  BOOL known = YES;
  for (CFIndex i = 0; i < CFArrayGetCount(children); i++) {
    AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, i);
    if (CFGetTypeID(child) != AXUIElementGetTypeID()) {
      known = NO;
      break;
    }
    char role[64] = {0}, subrole[64] = {0};
    if (!stringAttr(child, kAXRoleAttribute, role, sizeof(role), deadline)) {
      known = NO;
      break;
    }
    if (!strcmp(role, "AXSheet")) {
      out->sheet = 1;
      continue;
    }
    if (!strcmp(role, "AXWindow")) {
      BOOL modal = NO;
      if (!boolAttr(child, kAXModalAttribute, &modal, deadline) ||
          !stringAttr(child, kAXSubroleAttribute, subrole, sizeof(subrole),
                      deadline)) {
        known = NO;
        break;
      }
      if (modal || !strcmp(subrole, "AXDialog") ||
          !strcmp(subrole, "AXSystemDialog"))
        out->child = 1;
    }
    if (!inspectChildren(child, depth + 1, budget, deadline, out)) {
      known = NO;
      break;
    }
  }
  CFRelease(children);
  return known;
}
static OTActionWindowRole classifyRoot(AXUIElementRef root, uint32_t wid,
                                       int pid, double deadline) {
  OTActionWindowRole out = {0};
  out.self = selfApp(pid);
  strlcpy(out.reason, "AX role or relationship unresolved", sizeof(out.reason));
  if (!root)
    return out;
  NSRunningApplication *application =
      [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
  if (!out.self && (!application || application.terminated ||
                    !application.bundleIdentifier.length))
    return out;
  CGWindowID current = 0;
  pid_t owner = 0;
  if (_AXUIElementGetWindow(root, &current) != kAXErrorSuccess ||
      current != wid || AXUIElementGetPid(root, &owner) != kAXErrorSuccess ||
      owner != pid)
    return out;
  out.root = 1;
  if (!stringAttr(root, kAXRoleAttribute, out.role, sizeof(out.role),
                  deadline) ||
      !stringAttr(root, kAXSubroleAttribute, out.subrole, sizeof(out.subrole),
                  deadline))
    return out;
  CGPoint pos;
  CGSize size;
  if (!pointAttr(root, kAXPositionAttribute, kAXValueCGPointType, &pos,
                 deadline) ||
      !pointAttr(root, kAXSizeAttribute, kAXValueCGSizeType, &size, deadline))
    return out;
  out.x = pos.x;
  out.y = pos.y;
  out.w = size.width;
  out.h = size.height;
  if (!isfinite(out.x) || !isfinite(out.y) || !isfinite(out.w) ||
      !isfinite(out.h) || out.w <= 0 || out.h <= 0)
    return out;
  BOOL modal = NO;
  if (!boolAttr(root, kAXModalAttribute, &modal, deadline))
    return out;
  out.modal = modal;
  CFTypeRef parent = attr(root, kAXParentAttribute, deadline, NULL);
  BOOL parentKnown = NO;
  if (parent && CFGetTypeID(parent) == AXUIElementGetTypeID()) {
    char parentRole[64] = {0};
    if (stringAttr((AXUIElementRef)parent, kAXRoleAttribute, parentRole,
                   sizeof(parentRole), deadline)) {
      if (!strcmp(parentRole, "AXApplication"))
        parentKnown = YES;
      else if (!strcmp(parentRole, "AXWindow") ||
               !strcmp(parentRole, "AXSheet")) {
        CGWindowID parentID = 0;
        AXUIElementSetMessagingTimeout((AXUIElementRef)parent, .015);
        if (_AXUIElementGetWindow((AXUIElementRef)parent, &parentID) ==
                kAXErrorSuccess &&
            parentID) {
          out.parent = parentID;
          parentKnown = YES;
        }
      }
    }
  }
  if (parent)
    CFRelease(parent);
  if (!parentKnown)
    return out;
  AXError error;
  CFTypeRef sheets = attr(root, CFSTR("AXSheets"), deadline, &error);
  if (error == kAXErrorSuccess && sheets &&
      CFGetTypeID(sheets) == CFArrayGetTypeID())
    out.sheet = CFArrayGetCount(sheets) > 0;
  else if (error != kAXErrorNoValue && error != kAXErrorAttributeUnsupported) {
    if (sheets)
      CFRelease(sheets);
    return out;
  }
  if (sheets)
    CFRelease(sheets);
  int budget = 128;
  if (!inspectChildren(root, 0, &budget, deadline, &out))
    return out;
  // Unsupported AXSheets is covered by a complete AX child-tree inspection.
  out.known = 1;
  out.reason[0] = 0;
  return out;
}
// AppKit sheets are positive AXSheet children of a listed root, while their
// separate CG surfaces are absent from AXWindows. Identify only that exact
// protected child; never promote it to an actionable root or infer by bounds.
static BOOL classifyAttachedSheet(uint32_t wid, int pid, double deadline,
                                  OTActionWindowRole *out) {
  AXUIElementRef application = AXUIElementCreateApplication(pid);
  CFTypeRef roots = attr(application, kAXWindowsAttribute, deadline, NULL);
  CFRelease(application);
  if (!roots || CFGetTypeID(roots) != CFArrayGetTypeID()) {
    if (roots)
      CFRelease(roots);
    return NO;
  }
  BOOL found = NO;
  int budget = 128;
  for (CFIndex i = 0; i < CFArrayGetCount(roots) && budget > 0 && !found &&
                      ot_window_drag_now() < deadline;
       i++) {
    AXUIElementRef root = (AXUIElementRef)CFArrayGetValueAtIndex(roots, i);
    if (CFGetTypeID(root) != AXUIElementGetTypeID())
      continue;
    AXUIElementSetMessagingTimeout(root, .015);
    CGWindowID parentID = 0;
    if (_AXUIElementGetWindow(root, &parentID) != kAXErrorSuccess || !parentID)
      continue;
    CFTypeRef children = attr(root, kAXChildrenAttribute, deadline, NULL);
    if (children && CFGetTypeID(children) == CFArrayGetTypeID())
      for (CFIndex j = 0; j < CFArrayGetCount(children) && budget-- > 0 &&
                          ot_window_drag_now() < deadline;
           j++) {
        AXUIElementRef child =
            (AXUIElementRef)CFArrayGetValueAtIndex(children, j);
        if (CFGetTypeID(child) != AXUIElementGetTypeID())
          continue;
        char role[64] = {0};
        if (!stringAttr(child, kAXRoleAttribute, role, sizeof(role),
                        deadline) ||
            strcmp(role, "AXSheet"))
          continue;
        CGWindowID childID = 0;
        pid_t childPID = 0;
        if (_AXUIElementGetWindow(child, &childID) != kAXErrorSuccess ||
            childID != wid ||
            AXUIElementGetPid(child, &childPID) != kAXErrorSuccess ||
            childPID != pid)
          continue;
        CFTypeRef parent = attr(child, kAXParentAttribute, deadline, NULL);
        BOOL exactParent = parent &&
                           CFGetTypeID(parent) == AXUIElementGetTypeID() &&
                           CFEqual(parent, root);
        if (parent)
          CFRelease(parent);
        if (!exactParent)
          continue;
        out->root = 0;
        out->parent = parentID;
        strlcpy(out->role, "AXSheet", sizeof(out->role));
        out->subrole[0] = 0;
        CGPoint position;
        CGSize size;
        BOOL modal = NO;
        BOOL known = pointAttr(child, kAXPositionAttribute, kAXValueCGPointType,
                               &position, deadline) &&
                     pointAttr(child, kAXSizeAttribute, kAXValueCGSizeType,
                               &size, deadline);
        if (known) {
          out->x = position.x;
          out->y = position.y;
          out->w = size.width;
          out->h = size.height;
        }
        known = boolAttr(child, kAXModalAttribute, &modal, deadline) && known;
        out->modal = modal;
        known = inspectChildren(child, 0, &budget, deadline, out) && known;
        out->known = known;
        strlcpy(out->reason,
                known ? ""
                      : "attached AXSheet identified; modal relationship "
                        "lookup unresolved",
                sizeof(out->reason));
        found = YES;
        break;
      }
    if (children)
      CFRelease(children);
  }
  CFRelease(roots);
  return found;
}
OTActionWindowRole ot_window_drag_role(uint32_t wid, int pid) {
  @autoreleasepool {
    OTActionWindowRole out = {0};
    out.self = selfApp(pid);
    strlcpy(out.reason, "exact AX root unavailable", sizeof(out.reason));
    if (!wid || pid <= 0)
      return out;
    uint64_t sec, usec;
    int owner;
    if (!ot_retirement_identity(wid, &owner, &sec, &usec) || owner != pid)
      return out;
    double deadline = ot_window_drag_now() + .25;
    AXUIElementRef root = copyExactRoot(wid, pid, deadline);
    if (root) {
      out = classifyRoot(root, wid, pid, deadline);
      CFRelease(root);
    } else {
      classifyAttachedSheet(wid, pid, deadline, &out);
    }
    if (!ot_retirement_matches(wid, pid, sec, usec)) {
      out.root = 0;
      out.known = 0;
      strlcpy(out.reason, "window process identity changed",
              sizeof(out.reason));
    }
    return out;
  }
}
static BOOL finalOtherGuard(uintptr_t token) {
  return !token || goWindowDragFinalGuard(token);
}
int ot_window_drag_guard_probe(uintptr_t token) {
  return finalOtherGuard(token);
}
int ot_window_drag_perform(int kind, uint32_t wid, int pid) {
  return ot_window_drag_perform_guarded(kind, wid, pid, 0);
}
int ot_window_drag_perform_guarded(int kind, uint32_t wid, int pid,
                                   uintptr_t guard) {
  @autoreleasepool {
    if ((kind != 1 && kind != 2) || !wid || pid <= 0 || selfApp(pid))
      return 0;
    int owner;
    uint64_t sec, usec;
    if (!ot_retirement_identity(wid, &owner, &sec, &usec) || owner != pid)
      return 0;
    double deadline = ot_window_drag_now() + .3;
    AXUIElementRef root = copyExactRoot(wid, pid, deadline);
    if (!root)
      return 0;
    OTActionWindowRole role = classifyRoot(root, wid, pid, deadline);
    BOOL safe = role.root && role.known && !role.modal && !role.sheet &&
                !role.child && !role.parent && !role.self &&
                !strcmp(role.role, "AXWindow") &&
                !strcmp(role.subrole, "AXStandardWindow") &&
                ot_retirement_matches(wid, pid, sec, usec) && !selfApp(pid) &&
                ot_window_drag_now() < deadline;
    AXError result = kAXErrorFailure;
    if (safe && kind == 2) {
      Boolean settable = false;
      safe = AXUIElementIsAttributeSettable(root, kAXMinimizedAttribute,
                                            &settable) == kAXErrorSuccess &&
             settable;
      if (safe && ot_retirement_matches(wid, pid, sec, usec) &&
          finalOtherGuard(guard))
        result = AXUIElementSetAttributeValue(root, kAXMinimizedAttribute,
                                              kCFBooleanTrue);
    }
    if (safe && kind == 1) {
      CFTypeRef button = attr(root, kAXCloseButtonAttribute, deadline, NULL);
      if (button && CFGetTypeID(button) == AXUIElementGetTypeID()) {
        BOOL enabled = NO;
        if (boolAttr((AXUIElementRef)button, kAXEnabledAttribute, &enabled,
                     deadline) &&
            enabled && ot_retirement_matches(wid, pid, sec, usec) &&
            finalOtherGuard(guard))
          result =
              AXUIElementPerformAction((AXUIElementRef)button, kAXPressAction);
      }
      if (button)
        CFRelease(button);
    }
    CFRelease(root);
    return result == kAXErrorSuccess;
  }
}
uint64_t ot_window_drag_probe(void) {
  uint64_t result = 0;
  DragOwner *s = ot_window_drag_create(1);
  if (!s)
    return 0;
  CGEventRef event = CGEventCreateMouseEvent(
      NULL, kCGEventLeftMouseDown, CGPointMake(10, 10), kCGMouseButtonLeft);
  if (dragCallback(NULL, kCGEventLeftMouseDown, event, s) == event)
    result |= 1;
  OTWindowDragRaw down;
  ot_window_drag_pop(s, &down);
  if (!ot_window_drag_current(down.generation, down.gesture, 42, 7, 0, 0,
                              down.at))
    result |= 2;
  dragCallback(NULL, kCGEventLeftMouseUp, event, s);
  if (!liveRaw(down))
    result |= 4;
  dragCallback(NULL, kCGEventLeftMouseDown, event, s);
  for (int i = 0; i < 70; i++)
    dragCallback(NULL, kCGEventLeftMouseDragged, event, s);
  if (s->count <= 64 && !atomic_load(&physicalDown))
    result |= 8;
  if (correlated(30, 0, 30, 0, NO))
    result |= 16;
  if (!correlated(0, 0, 30, 0, NO))
    result |= 32;
  if (!correlated(5, 30, 5, 30, NO))
    result |= 64;
  DragEvidence track = {0};
  track.root = AXUIElementCreateApplication(getpid());
  AXUIElementRef original = track.root;
  track.sample.raw = (OTWindowDragRaw){.at = 1, .x = 10, .y = 10};
  OTWindowDragRaw step = {.at = 2, .x = 15, .y = 10, .kind = 2};
  BOOL stationary = advanceEvidence(&track, step, CGPointZero);
  step.at = 3;
  step.x = 45;
  BOOL moved = advanceEvidence(&track, step, CGPointMake(30, 0));
  if (!stationary && moved && track.root == original && track.sample.wx == 30)
    result |= 128;
  if (!intentAfterDiscontinuity(track, 1.5) &&
      intentAfterDiscontinuity(track, 3))
    result |= 256;
  CFRelease(track.root);
  CFRelease(event);
  uint64_t generation = s->generation;
  ot_window_drag_stop(s);
  ot_window_drag_clear(generation);
  return result;
}

int ot_window_drag_active(void) { return atomic_load(&activeGeneration) != 0; }

uint64_t ot_window_drag_evidence_epoch(void) {
  pthread_mutex_lock(&sampleMutex);
  uint64_t v = evidenceEpoch;
  pthread_mutex_unlock(&sampleMutex);
  return v;
}

uint64_t ot_window_drag_generation(void *owner) {
  return owner ? ((DragOwner *)owner)->generation : 0;
}

int ot_window_drag_trusted(void) { return AXIsProcessTrusted(); }

int ot_window_drag_listening_allowed(void) {
  if (@available(macOS 10.15, *))
    return CGPreflightListenEventAccess();
  return 1;
}
int ot_window_drag_healthy(void *owner) {
  DragOwner *s = owner;
  return s && (s->test || (s->tap && CFMachPortIsValid(s->tap) &&
                           CGEventTapIsEnabled(s->tap)));
}
