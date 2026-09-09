//go:build darwin
#import "darwin_launcher_gestures.h"
#include <math.h>
@interface OTLauncherGestureOwner : NSObject {
@public
  OTLauncherGesturePolicy policy;
  OTLauncherGestureEvent queue[129];
  NSUInteger head, count;
  uint64_t gesture, valid;
  int kind;
  BOOL ended, bypass, closed;
  double last;
}
@end
@implementation OTLauncherGestureOwner
@end
static NSMutableDictionary<NSNumber *, OTLauncherGestureOwner *> *
gestureOwners(void) {
  static NSMutableDictionary *values;
  static dispatch_once_t once;
  dispatch_once(&once, ^{
    values = [NSMutableDictionary new];
  });
  return values;
}
static uint64_t launcherGestureSequence, launcherEventSequence;
static void gestureMain(void (^work)(void)) {
  if (NSThread.isMainThread)
    work();
  else
    dispatch_sync(dispatch_get_main_queue(), work);
}
static int gesturePhase(NSEventPhase p) {
  if (p & NSEventPhaseCancelled)
    return 4;
  if (p & NSEventPhaseEnded)
    return 3;
  if (p & NSEventPhaseBegan)
    return 1;
  if (p & NSEventPhaseChanged)
    return 2;
  return 0;
}
static void gestureCancel(OTLauncherGestureOwner *o) {
  if (o->valid || o->gesture) {
    OTLauncherGestureEvent e = {.policy = o->policy,
                                .sequence = ++launcherEventSequence,
                                .gesture = o->gesture ?: o->valid,
                                .kind = o->kind,
                                .phase = 4,
                                .owned = 1,
                                .time = NSDate.date.timeIntervalSince1970};
    if (o->count < 129) {
      o->queue[(o->head + o->count) % 129] = e;
      o->count++;
    }
  }
  o->valid = o->gesture = 0;
  o->bypass = YES;
  o->ended = YES;
}
void OTLauncherGestureRetire(uint64_t token, BOOL close) {
  OTLauncherGestureOwner *o = gestureOwners()[@(token)];
  if (!o)
    return;
  gestureCancel(o);
  o->policy.enabled = 0;
  o->closed = close;
  if (close)
    [gestureOwners() removeObjectForKey:@(token)];
}
int ot_launcher_gesture_policy(uint64_t token, OTLauncherGesturePolicy p) {
  return ot_launcher_gesture_policy_guarded(token, p, 0);
}
int ot_launcher_gesture_policy_guarded(uint64_t token, OTLauncherGesturePolicy p,
                                       uintptr_t guard) {
  __block int ok = 0;
  gestureMain(^{
    if (!token || !p.admission || !p.epoch || !p.session || !p.revision ||
        !memchr(p.display, 0, sizeof(p.display)) || !p.display[0] ||
        !isfinite(p.w) || !isfinite(p.h) || p.w <= 0 || p.h <= 0 ||
        p.w > 16384 || p.h > 16384)
      return;
    OTLauncherGestureOwner *o = gestureOwners()[@(token)];
    if (o && (o->closed || p.admission < o->policy.admission ||
              p.epoch < o->policy.epoch ||
              (p.epoch == o->policy.epoch && p.session < o->policy.session) ||
              (p.epoch == o->policy.epoch && p.session == o->policy.session &&
               p.revision < o->policy.revision)))
      return;
    if (!OTLauncherGestureHost(token, p.display))
      return;
    // The synchronous Go guard reads only worker lifetime state. No native
    // preparation or dispatcher wait is permitted inside it.
    if (guard && !ot_go_launcher_gesture_policy_current(guard))
      return;
    if (o && p.admission == o->policy.admission) {
      ok = memcmp(&o->policy, &p, sizeof(p)) == 0;
      return;
    }
    if (!o) {
      o = [OTLauncherGestureOwner new];
      gestureOwners()[@(token)] = o;
    } else
      gestureCancel(o);
    o->policy = p;
    o->bypass = NO;
    o->ended = YES;
    ok = 1;
  });
  return ok;
}
static BOOL gestureAdmit(OTLauncherGestureOwner *o, OTLauncherGestureEvent e,
                         double stamp) {
  if (!o || o->closed || !o->policy.enabled)
    return NO;
  if (o->gesture && stamp - o->last > .25) {
    if (o->ended) {
      o->gesture = 0;
    } else {
      gestureCancel(o);
    }
    o->bypass = NO;
  }
  BOOL momentum = e.momentum != 0;
  BOOL begin = e.phase == 1 || (e.kind == 3 && e.phase == 0);
  if (begin && !momentum) {
    if (o->valid || (o->gesture && !o->ended)) {
      o->bypass = YES;
      return NO;
    }
    o->gesture = 0;
    o->bypass = NO;
  }
  if (o->bypass || (o->ended && o->gesture && !momentum && !begin))
    return NO;
  if (!o->gesture) {
    if (momentum || !begin || o->valid || o->count >= 128 || e.x < 0 ||
        e.y < 0 || e.x >= o->policy.w || e.y >= o->policy.h)
      return NO;
    o->gesture = ++launcherGestureSequence;
    o->valid = o->gesture;
    o->kind = e.kind;
    o->ended = NO;
  }
  if (o->kind != e.kind) {
    gestureCancel(o);
    return NO;
  }
  if (o->count >= 128) {
    gestureCancel(o);
    return NO;
  }
  o->last = stamp;
  e.policy = o->policy;
  e.sequence = ++launcherEventSequence;
  e.gesture = o->gesture;
  e.owned = 1;
  if (e.kind == 3 && e.phase == 0)
    e.phase = 1;
  o->queue[(o->head + o->count) % 129] = e;
  o->count++;
  if (e.phase == 3 || e.phase == 4 || e.momentum == 3 || e.momentum == 4 ||
      e.kind == 3)
    o->ended = YES;
  if (e.phase == 4 || e.momentum == 4)
    o->valid = 0;
  if (e.momentum == 3 || e.momentum == 4 || e.phase == 4)
    o->bypass = YES;
  return YES;
}
BOOL OTLauncherGestureHandle(uint64_t token, NSEvent *event, NSView *view) {
  OTLauncherGestureOwner *o = gestureOwners()[@(token)];
  if (!o || !o->policy.enabled || o->closed || !view)
    return NO;
  NSEventType type = event.type;
  int kind = 0;
  if (type == NSEventTypeScrollWheel && o->policy.scroll)
    kind = 1;
  else if (type == NSEventTypeMagnify && o->policy.magnify)
    kind = 2;
  else if (type == NSEventTypeSwipe && o->policy.swipe)
    kind = 3;
  else
    return NO;
  OTLauncherGestureEvent e = {.kind = kind};
  double stamp = event.timestamp;
  NSPoint point = [view convertPoint:event.locationInWindow fromView:nil];
  e.x = point.x - NSMinX(view.bounds);
  e.y = view.flipped ? point.y - NSMinY(view.bounds)
                     : NSMaxY(view.bounds) - point.y;
  e.phase = gesturePhase(event.phase);
  e.time = NSDate.date.timeIntervalSince1970 -
           NSProcessInfo.processInfo.systemUptime + stamp;
  if (kind == 1) {
    e.precise = event.hasPreciseScrollingDeltas;
    if (!e.precise)
      return NO;
    double sign = event.isDirectionInvertedFromDevice ? -1 : 1;
    e.dx = -event.scrollingDeltaX * sign;
    e.dy = -event.scrollingDeltaY * sign;
    e.momentum = gesturePhase(event.momentumPhase);
  } else if (kind == 2)
    e.magnification = event.magnification;
  else {
    e.dx = -event.deltaX;
    e.dy = -event.deltaY;
  }
  if (!isfinite(stamp) || stamp <= 0 || !isfinite(e.x) || !isfinite(e.y) ||
      !isfinite(e.dx) || !isfinite(e.dy) || !isfinite(e.magnification)) {
    gestureCancel(o);
    return NO;
  }
  return gestureAdmit(o, e, stamp);
}
int ot_launcher_gesture_next(uint64_t token, OTLauncherGestureEvent *out) {
  __block int status = 0;
  gestureMain(^{
    OTLauncherGestureOwner *o = gestureOwners()[@(token)];
    if (!o) {
      status = -1;
      return;
    }
    if (o->count) {
      *out = o->queue[o->head];
      o->head = (o->head + 1) % 129;
      o->count--;
      status = 1;
    } else if (o->closed) {
      [gestureOwners() removeObjectForKey:@(token)];
      status = -1;
    }
  });
  return status;
}
static BOOL sameGesture(OTLauncherGestureOwner *o, uint64_t e, uint64_t s,
                        uint64_t r, uint64_t a, uint64_t g) {
  return o && !o->closed && o->policy.enabled && g && o->valid == g &&
         o->policy.epoch == e && o->policy.session == s &&
         o->policy.revision == r && o->policy.admission == a;
}
int ot_launcher_gesture_valid(uint64_t token, uint64_t e, uint64_t s,
                              uint64_t r, uint64_t a, uint64_t g) {
  __block int ok = 0;
  gestureMain(^{
    OTLauncherGestureOwner *o = gestureOwners()[@(token)];
    ok = sameGesture(o, e, s, r, a, g) &&
         OTLauncherGestureHost(token, o->policy.display);
  });
  return ok;
}
void ot_launcher_gesture_complete(uint64_t token, uint64_t e, uint64_t s,
                                  uint64_t r, uint64_t a, uint64_t g) {
  gestureMain(^{
    OTLauncherGestureOwner *o = gestureOwners()[@(token)];
    if (sameGesture(o, e, s, r, a, g))
      o->valid = 0;
  });
}
