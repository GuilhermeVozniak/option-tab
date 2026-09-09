#import "../../darwin_media_panel.m"
#import "../../darwin_launcher_gestures.m"
#import "../../darwin_dock_panel.m"
#include <assert.h>
// This wheel-only fixture must never query launcher Space authority.
uint64_t ot_launcher_space_id(const char *display) { abort(); }
// Property seam exercises actual NSEvent extraction without registering a
// window, installing a tap, showing UI, or moving the desktop pointer.
@interface OTWheelEventFixture : NSEvent
@property BOOL fixturePrecise;
@end
@implementation OTWheelEventFixture
- (BOOL)hasPreciseScrollingDeltas {
  return self.fixturePrecise;
}
- (BOOL)isDirectionInvertedFromDevice {
  return YES;
}
- (NSPoint)locationInWindow {
  return NSMakePoint(10, 90);
}
- (CGFloat)scrollingDeltaX {
  return 12;
}
- (CGFloat)scrollingDeltaY {
  return -8;
}
- (NSEventPhase)phase {
  return NSEventPhaseBegan;
}
- (NSEventPhase)momentumPhase {
  return NSEventPhaseNone;
}
- (NSTimeInterval)timestamp {
  return NSProcessInfo.processInfo.systemUptime;
}
@end
@interface OTWheelPanelFixture : NSObject
@property uint64_t panelToken;
@end
@implementation OTWheelPanelFixture
@end
static OTPanelWheelInput input(double x, double y, int phase, int momentum) {
  return (OTPanelWheelInput){.x = x,
                             .y = y,
                             .dx = 20,
                             .dy = 30,
                             .timestamp = 1,
                             .unixTime = 100,
                             .precise = 1,
                             .phase = phase,
                             .momentum = momentum};
}
static OTDockPanelRecord *record(void) {
  OTDockPanelRecord *r = [OTDockPanelRecord new];
  r.wheelVisible = YES;
  assert(applyWheelPolicy(r, 9, 1, YES, @[ @{
                            @"window" : @42,
                            @"app" : @7,
                            @"x" : @0,
                            @"y" : @0,
                            @"w" : @100,
                            @"h" : @100
                          } ]));
  return r;
}
int main(void) {
  @autoreleasepool {
    OTDockPanelRecord *r = record();
    OTPanelWheelInput e = input(10, 10, 1, 0);
    e.precise = 0;
    assert(!admitWheel(r, e));
    assert(r.wheelMailbox->count == 0);
    e = input(200, 200, 1, 0);
    assert(!admitWheel(r, e));
    e = input(10, 10, 2, 0);
    assert(!admitWheel(r, e));
    e = input(10, 10, 1, 0);
    assert(admitWheel(r, e));
    uint64_t g = r.wheelGesture;
    assert(r.wheelValidGesture == g && r.wheelWindow == 42 && r.wheelApp == 7);
    e = input(200, 200, 2, 0);
    e.inverted = 1;
    assert(admitWheel(r, e));
    assert(r.wheelMailbox->events[1].dx == -20 &&
           r.wheelMailbox->events[1].dy == -30);
    e = input(200, 200, 3, 0);
    assert(admitWheel(r, e));
    e = input(10, 10, 1, 0);
    assert(!admitWheel(r, e));
    assert(r.wheelValidGesture == g);
    acknowledgeWheel(r, 9, 1, g);
    assert(r.wheelValidGesture == 0);
    e = input(10, 10, 1, 0);
    assert(admitWheel(r, e));
    assert(r.wheelGesture != g);
    g = r.wheelGesture;
    acknowledgeWheel(r, 9, 1, g);
    e = input(10, 10, 3, 0);
    assert(admitWheel(r, e));
    e = input(200, 200, 0, 1);
    assert(admitWheel(r, e));
    assert(r.wheelGesture == g);
    e = input(200, 200, 0, 3);
    assert(admitWheel(r, e));
    cancelWheel(r, 2);
    assert(r.wheelGesture == 0 && r.wheelValidGesture == 0);
    assert(applyWheelPolicy(r, 9, 2, NO, @[]));
    e = input(10, 10, 1, 0);
    assert(!admitWheel(r, e));
    OTDockPanelRecord *overflow = record();
    e = input(10, 10, 1, 0);
    assert(admitWheel(overflow, e));
    e.phase = 2;
    for (int i = 1; i < 128; i++)
      assert(admitWheel(overflow, e));
    assert(!admitWheel(overflow, e));
    assert(overflow.wheelMailbox->count == 129);
    assert(overflow.wheelMailbox->events[128].reason == 4);
    assert(overflow.wheelValidGesture == 0);
    // Extract the real NSEvent property contract and top-left content
    // coordinates.
    OTDockPanelRecord *native = record();
    native.content = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, 100, 100)];
    OTWheelPanelFixture *panel = [OTWheelPanelFixture new];
    panel.panelToken = 999;
    panelRecords()[@999] = native;
    OTWheelEventFixture *event = [OTWheelEventFixture new];
    assert(!handlePanelWheel((OTDockPanel *)panel, event));
    event.fixturePrecise = YES;
    assert(handlePanelWheel((OTDockPanel *)panel, event));
    OTDockPanelWheelEvent extracted = native.wheelMailbox->events[0];
    assert(extracted.x == 10 && extracted.y == 10 && extracted.dx == -12 &&
           extracted.dy == 8);
    assert(extracted.phase == 1 && extracted.momentum == 0 &&
           extracted.inverted);
    assert(ot_dock_panel_wheel_valid(999, 9, 1, extracted.gesture));
    assert(!ot_dock_panel_wheel_valid(999, 9, 2, extracted.gesture));
    assert(ot_dock_panel_hide(999));
    assert(!ot_dock_panel_wheel_valid(999, 9, 1, extracted.gesture));
    assert(native.wheelMailbox->events[1].reason == 2);
    [panelRecords() removeObjectForKey:@999];
    // Tail expiration releases suppression without invalidating pending
    // actions.
    OTDockPanelRecord *tail = record();
    e = input(10, 10, 1, 0);
    assert(admitWheel(tail, e));
    g = tail.wheelGesture;
    e.phase = 3;
    assert(admitWheel(tail, e));
    e.phase = 0;
    e.momentum = 1;
    e.timestamp = 1.251;
    assert(!admitWheel(tail, e));
    assert(tail.wheelValidGesture == g);
    acknowledgeWheel(tail, 9, 1, g);
    e = input(10, 10, 1, 0);
    assert(admitWheel(tail, e));
    g = tail.wheelGesture;
    assert(applyWheelPolicy(tail, 9, 2, YES, @[]));
    assert(!tail.wheelValidGesture && tail.wheelMailbox->events[3].reason == 1);
    acknowledgeWheel(tail, 9, 1, g);
    assert(tail.wheelRevision == 2);
    assert(!applyWheelPolicy(tail, 9, 1, YES, @[]));
    e = input(10, 10, 1, 0);
    assert(!admitWheel(tail, e));
    puts("PASS NSEvent extraction/top-left coordinates/hide "
         "invalidation/revision/250ms tail expiry");
    puts("PASS native precise/momentum/immutable "
         "target/normalization/ack/backpressure/coarse/miss/disabled/overflow "
         "classification");
  }
  return 0;
}

int ot_go_launcher_gesture_policy_current(uintptr_t handle) { return 0; }
#import "../../darwin_launcher_keyboard.m"
int ot_go_launcher_keyboard_current(uintptr_t handle) { return 0; }
