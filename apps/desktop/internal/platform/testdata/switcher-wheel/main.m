#import "../../darwin_media_panel.m"
#import "../../darwin_launcher_gestures.m"
#import "../../darwin_dock_panel.m"
#include <assert.h>
uint64_t ot_launcher_space_id(const char *display) { abort(); }
#ifndef OT_SWITCHER_WHEEL_AVAILABLE
int main(void) { assert(!"native overlay wheel owner is missing"); }
#else
// Property seam: no NSWindow creation, application launch or OS input.
@interface OTOverlayHostFixture : NSObject
@property NSView *contentView;
@property(getter=isVisible) BOOL visible;
@property(getter=isOnActiveSpace) BOOL onActiveSpace;
@property(getter=isMiniaturized) BOOL miniaturized;
@property NSWindowOcclusionState occlusionState;
@end
@implementation OTOverlayHostFixture
@end
@interface OTOverlayEventFixture : NSEvent
@property NSWindow *fixtureWindow;
@property BOOL precise, inverted;
@property NSEventPhase fixturePhase, fixtureMomentum;
@property NSPoint point;
@property double dx, dy, at;
@end
@implementation OTOverlayEventFixture
- (NSEventType)type { return NSEventTypeScrollWheel; }
- (NSWindow *)window { return self.fixtureWindow; }
- (BOOL)hasPreciseScrollingDeltas { return self.precise; }
- (BOOL)isDirectionInvertedFromDevice { return self.inverted; }
- (NSEventPhase)phase { return self.fixturePhase; }
- (NSEventPhase)momentumPhase { return self.fixtureMomentum; }
- (NSPoint)locationInWindow { return self.point; }
- (CGFloat)scrollingDeltaX { return self.dx; }
- (CGFloat)scrollingDeltaY { return self.dy; }
- (NSTimeInterval)timestamp { return self.at; }
@end
int main(void) {
 @autoreleasepool {
  OTOverlayHostFixture *host = [OTOverlayHostFixture new];
  host.contentView = [[NSView alloc] initWithFrame:NSMakeRect(0,0,300,200)];
  host.visible = host.onActiveSpace = YES;
  host.occlusionState = NSWindowOcclusionStateVisible;
  OTSwitcherWheelRecord *r = [OTSwitcherWheelRecord new];
  r.host = (NSWindow *)host;
  r.content = host.contentView;
  r.wheel = [OTDockPanelRecord new];
  r.wheel.content = r.content;
  r.wheel.wheelVisible = YES;
  NSArray *regions = @[@{@"window":@42,@"app":@7,@"x":@10,@"y":@20,@"w":@80,@"h":@80}];
  assert(applyWheelPolicy(r.wheel,9,1,YES,regions));
  r.bounds = r.content.bounds;
  OTOverlayEventFixture *e=[OTOverlayEventFixture new];
  e.fixtureWindow=r.host; e.precise=YES; e.fixturePhase=NSEventPhaseBegan;
  e.point=NSMakePoint(20,160); e.dy=-20; e.at=1;
  e.precise=NO; assert(!handleSwitcherWheel(r,e));
  e.precise=YES; e.fixturePhase=NSEventPhaseNone; assert(!handleSwitcherWheel(r,e));
  e.fixturePhase=NSEventPhaseBegan; e.fixtureWindow=nil; assert(!handleSwitcherWheel(r,e));
  e.fixtureWindow=r.host; e.point=NSMakePoint(200,160); assert(!handleSwitcherWheel(r,e));
  e.point=NSMakePoint(20,160); e.fixturePhase=NSEventPhaseChanged; assert(!handleSwitcherWheel(r,e));
  e.fixturePhase=NSEventPhaseBegan; e.inverted=YES; assert(handleSwitcherWheel(r,e));
  assert(r.wheel.wheelWindow==42 && r.wheel.wheelApp==7);
  assert(r.wheel.wheelMailbox->events[0].dy==20 && r.wheel.wheelMailbox->events[0].y==40);
  uint64_t gesture=r.wheel.wheelGesture;
  e.fixturePhase=NSEventPhaseChanged; e.point=NSMakePoint(200,100); e.at=1.1;
  assert(handleSwitcherWheel(r,e)); assert(r.wheel.wheelWindow==42);
  e.fixturePhase=NSEventPhaseEnded; e.at=1.2; assert(handleSwitcherWheel(r,e));
  assert(r.wheel.wheelValidGesture==gesture);
  acknowledgeWheel(r.wheel,9,1,gesture);
  e.fixturePhase=NSEventPhaseNone; e.fixtureMomentum=NSEventPhaseBegan; e.at=1.3;
  assert(handleSwitcherWheel(r,e)); assert(!r.wheel.wheelValidGesture);
  e.fixtureMomentum=NSEventPhaseEnded; e.at=1.4; assert(handleSwitcherWheel(r,e));
  e.fixtureMomentum=NSEventPhaseNone; e.fixturePhase=NSEventPhaseBegan; e.point=NSMakePoint(20,160); e.at=2;
  assert(handleSwitcherWheel(r,e));
  host.visible=NO; assert(!handleSwitcherWheel(r,e)); assert(!r.wheel.wheelValidGesture);
  host.visible=YES; r.wheel.wheelVisible=YES;
  assert(applyWheelPolicy(r.wheel,9,2,YES,regions)); assert(handleSwitcherWheel(r,e));
  r.content.frame=NSMakeRect(0,0,400,200);
  assert(!handleSwitcherWheel(r,e)); assert(!r.wheel.wheelValidGesture);
  r.bounds=r.content.bounds; r.wheel.wheelVisible=YES;
  assert(applyWheelPolicy(r.wheel,9,3,NO,regions)); assert(!handleSwitcherWheel(r,e));
  // A policy can arrive while the real overlay is still hidden/occluded.
  // Visibility recovery preserves its copied geometry, never a prior token.
  assert(applyWheelPolicy(r.wheel,9,4,YES,regions));
  host.visible=NO; refreshSwitcherWheelVisibility(r,NO);
  assert(!r.wheel.wheelVisible);
  host.visible=YES; refreshSwitcherWheelVisibility(r,NO);
  assert(r.wheel.wheelVisible);
  e.at=4; assert(handleSwitcherWheel(r,e));
  uint64_t retired=r.wheel.wheelValidGesture;
  host.occlusionState=0; refreshSwitcherWheelVisibility(r,NO);
  assert(!r.wheel.wheelVisible && !r.wheel.wheelValidGesture);
  host.occlusionState=NSWindowOcclusionStateVisible;
  refreshSwitcherWheelVisibility(r,NO);
  assert(r.wheel.wheelVisible && !r.wheel.wheelValidGesture);
  e.at=5; assert(handleSwitcherWheel(r,e));
  assert(r.wheel.wheelValidGesture>retired);
  refreshSwitcherWheelVisibility(r,YES);
  assert(r.wheel.wheelVisible && !r.wheel.wheelValidGesture);
  r.content.frame=NSMakeRect(0,0,500,200);
  refreshSwitcherWheelVisibility(r,YES);
  assert(!r.wheel.wheelVisible);
  puts("PASS overlay exact-host/precise/phase/region/sign/immutable-target/momentum/hide/resize/pass-through/occlusion-recovery seam");

  // Real, owned, hidden AppKit host: no application activation, orderFront,
  // OS input posting or interaction with another process's windows.
  [NSApplication sharedApplication];
  [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
  NSWindow *owned=[[NSWindow alloc] initWithContentRect:NSMakeRect(0,0,300,200) styleMask:NSWindowStyleMaskBorderless backing:NSBackingStoreBuffered defer:NO];
  owned.releasedWhenClosed=NO;
  assert(!owned.visible);
  assert(!ot_switcher_wheel_create((void *)1));
  uint64_t token=ot_switcher_wheel_create((__bridge void *)owned);
  assert(token && switcherWheelRecords()[@(token)].monitor);
  assert(!ot_switcher_wheel_create((__bridge void *)owned));
  const char *policy="{\"session\":20,\"revision\":1,\"enabled\":true,\"regions\":[{\"window\":42,\"app\":7,\"x\":10,\"y\":20,\"w\":80,\"h\":80}]}";
  assert(ot_switcher_wheel_policy(token,policy)==1);
  OTSwitcherWheelRecord *actual=switcherWheelRecords()[@(token)];
  assert(!actual.wheel.wheelVisible);
  // Seed the production mailbox without posting synthetic OS events.
  actual.wheel.wheelVisible=YES;
  OTPanelWheelInput input={.x=20,.y=40,.dy=90,.precise=YES,.phase=1,.timestamp=8,.unixTime=8};
  assert(admitWheel(actual.wheel,input));
  uint64_t pending=actual.wheel.wheelValidGesture;
  assert(pending);
  ot_switcher_wheel_close(token);
  assert(!switcherWheelRecords()[@(token)] && !actual.monitor && !actual.host);
  OTDockPanelWheelEvent output={0};
  assert(ot_switcher_wheel_next(token,&output)==1 && output.gesture==pending && output.phase==1);
  assert(ot_switcher_wheel_next(token,&output)==1 && output.gesture==pending && output.phase==4 && output.reason==3);
  assert(ot_switcher_wheel_next(token,&output)==-1);
  assert(!retiredSwitcherWheelMailboxes()[@(token)]);
  assert(!ot_switcher_wheel_valid(token,20,1,pending));
  uint64_t replacement=ot_switcher_wheel_create((__bridge void *)owned);
  assert(replacement>token);
  owned.contentView=[[NSView alloc] initWithFrame:NSMakeRect(0,0,300,200)];
  assert(ot_switcher_wheel_policy(replacement,policy)==-1);
  assert(!switcherWheelRecords()[@(replacement)]);
  uint64_t closing=ot_switcher_wheel_create((__bridge void *)owned);
  assert(closing>replacement);
  [owned close];
  assert(!switcherWheelRecords()[@(closing)]);
  puts("PASS owned hidden AppKit monitor attach/duplicate-host/content-retirement/close/mailbox-drain/token-lifetime");
 }
}
#endif

int ot_go_launcher_gesture_policy_current(uintptr_t handle) { return 0; }
#import "../../darwin_launcher_keyboard.m"
int ot_go_launcher_keyboard_current(uintptr_t handle) { return 0; }
