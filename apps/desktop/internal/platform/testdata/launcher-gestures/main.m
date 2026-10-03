#import "../../darwin_launcher_gestures.m"
#include <assert.h>
int OTLauncherGestureHost(uint64_t token, const char *display) {
  return token == 1 && strcmp(display, "fixture") == 0;
}
@interface FixtureEvent : NSEvent
@property int fixtureKind;
@property NSEventPhase fixturePhase;
@property NSEventPhase fixtureMomentum;
@property double fixtureDX, fixtureDY, fixtureMag;
@property BOOL fixturePrecise;
@end
@implementation FixtureEvent
- (NSEventType)type {
  return self.fixtureKind == 1   ? NSEventTypeScrollWheel
         : self.fixtureKind == 2 ? NSEventTypeMagnify
                                 : NSEventTypeSwipe;
}
- (NSPoint)locationInWindow {
  return NSMakePoint(10, 10);
}
- (NSTimeInterval)timestamp {
  return NSProcessInfo.processInfo.systemUptime;
}
- (NSEventPhase)phase {
  return self.fixturePhase;
}
- (NSEventPhase)momentumPhase {
  assert(self.fixtureKind == 1);
  return self.fixtureMomentum;
}
- (BOOL)hasPreciseScrollingDeltas {
  assert(self.fixtureKind == 1);
  return self.fixturePrecise;
}
- (BOOL)isDirectionInvertedFromDevice {
  assert(self.fixtureKind == 1);
  return NO;
}
- (CGFloat)scrollingDeltaX {
  assert(self.fixtureKind == 1);
  return self.fixtureDX;
}
- (CGFloat)scrollingDeltaY {
  assert(self.fixtureKind == 1);
  return self.fixtureDY;
}
- (CGFloat)magnification {
  assert(self.fixtureKind == 2);
  return self.fixtureMag;
}
- (CGFloat)deltaX {
  assert(self.fixtureKind == 3);
  return self.fixtureDX;
}
- (CGFloat)deltaY {
  assert(self.fixtureKind == 3);
  return self.fixtureDY;
}
@end
static BOOL fixturePolicyCurrent = YES;
int ot_go_launcher_gesture_policy_current(uintptr_t handle) {
 assert(handle == 1);
 return fixturePolicyCurrent;
}
int main() {
  @autoreleasepool {
    NSView *view = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 100, 100)];
    OTLauncherGesturePolicy p = {.epoch = 1,
                                 .session = 1,
                                 .revision = 1,
                                 .admission = 1,
                                 .enabled = 1,
                                 .scroll = 1,
                                 .magnify = 1,
                                 .swipe = 1,
                                 .w = 100,
                                 .h = 100};
    strcpy(p.display, "fixture");
    assert(ot_launcher_gesture_policy(1, p));
    FixtureEvent *event = [FixtureEvent new];
    event.fixtureKind = 1;
    event.fixturePhase = NSEventPhaseBegan;
    event.fixturePrecise = NO;
    assert(!OTLauncherGestureHandle(1, event, view));
    event.fixturePrecise = YES;
    event.fixtureDX = 4;
    assert(OTLauncherGestureHandle(1, event, view));
    OTLauncherGestureEvent e;
    assert(ot_launcher_gesture_next(1, &e) == 1);
    assert(e.dx == -4 && e.x == 10 && e.y == 90 && e.owned);
    uint64_t g = e.gesture;
    assert(ot_launcher_gesture_valid(1, 1, 1, 1, 1, g));
    event.fixturePhase = NSEventPhaseEnded;
    assert(OTLauncherGestureHandle(1, event, view));
    assert(ot_launcher_gesture_valid(1, 1, 1, 1, 1, g));
    assert(!OTLauncherGestureHandle(1, event, view));
    ot_launcher_gesture_complete(1, 1, 1, 1, 1, g);
    assert(!ot_launcher_gesture_valid(1, 1, 1, 1, 1, g));
    event.fixturePhase = NSEventPhaseNone;
    event.fixtureMomentum = NSEventPhaseBegan;
    assert(OTLauncherGestureHandle(1, event, view));
    event.fixtureMomentum = NSEventPhaseChanged;
    assert(OTLauncherGestureHandle(1, event, view));
    event.fixtureMomentum = NSEventPhaseEnded;
    assert(OTLauncherGestureHandle(1, event, view));
    assert(!OTLauncherGestureHandle(1, event, view));
    event.fixtureMomentum = NSEventPhaseNone;
    p.admission++;
    assert(ot_launcher_gesture_policy(1, p));
    event.fixtureKind = 2;
    event.fixturePhase = NSEventPhaseBegan;
    event.fixtureMag = .2;
    assert(OTLauncherGestureHandle(1, event, view));
    OTLauncherGestureRetire(1, NO);
    assert(!ot_launcher_gesture_valid(1, 1, 1, 1, 2, g));
    assert(!OTLauncherGestureHandle(1, event, view));
    assert(!ot_launcher_gesture_policy(1, p));
    p.admission++;
    assert(ot_launcher_gesture_policy(1, p));
    event.fixtureKind = 3;
    event.fixturePhase = NSEventPhaseNone;
    event.fixtureDX = 1;
    assert(OTLauncherGestureHandle(1, event, view));

    while (ot_launcher_gesture_next(1, &e) == 1) {
    }
    p.admission++;
    assert(ot_launcher_gesture_policy(1, p));
    while (ot_launcher_gesture_next(1, &e) == 1) {
    }
    event.fixtureKind = 1;
    event.fixturePhase = NSEventPhaseBegan;
    event.fixtureDX = 1;
    assert(OTLauncherGestureHandle(1, event, view));
    event.fixturePhase = NSEventPhaseChanged;
    for (int i = 1; i < 128; i++)
      assert(OTLauncherGestureHandle(1, event, view));
    assert(!OTLauncherGestureHandle(1, event, view));
    int count = 0;
    while (ot_launcher_gesture_next(1, &e) == 1) {
      count++;
    }
    assert(count == 129);
    assert(e.phase == 4);
    assert(!ot_launcher_gesture_valid(1, 1, 1, 1, p.admission, e.gesture));
    OTLauncherGesturePolicy stale = p;
    stale.admission--;
    assert(!ot_launcher_gesture_policy(1, stale));
    p.admission++;
    assert(ot_launcher_gesture_policy(1, p));
    event.fixturePhase = NSEventPhaseBegan;
    event.fixtureDX = NAN;
    assert(!OTLauncherGestureHandle(1, event, view));
    // A queued setter carries a retired Go lifetime after native Hide/Show.
    OTLauncherGestureRetire(1, NO);
    p.admission++;
    fixturePolicyCurrent = NO;
    assert(!ot_launcher_gesture_policy_guarded(1, p, 1));
    event.fixtureDX = 1;
    assert(!OTLauncherGestureHandle(1, event, view));
    // Refusing the old lifetime cannot clobber a newer admitted policy.
    fixturePolicyCurrent = YES;
    p.admission++;
    assert(ot_launcher_gesture_policy_guarded(1, p, 1));
    fixturePolicyCurrent = NO;
    OTLauncherGesturePolicy rejected = p;
    rejected.admission++;
    assert(!ot_launcher_gesture_policy_guarded(1, rejected, 1));
    assert(OTLauncherGestureHandle(1, event, view));
    OTLauncherGestureRetire(1, YES);
    assert(ot_launcher_gesture_next(1, &e) == -1);
    puts("launcher gesture unattached extraction PASS");
  }
  return 0;
}
