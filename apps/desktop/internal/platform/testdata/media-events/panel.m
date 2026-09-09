#import <Cocoa/Cocoa.h>
#include "../../darwin_media_panel.m"
@interface FakePinPanel:NSObject
@property(getter=isVisible) BOOL visible;
@property NSRect frame;
@property NSView *contentView;
@property int drags;
@end
@implementation FakePinPanel
-(void)performWindowDragWithEvent:(NSEvent *)event{(void)event;self.drags++;OTMediaHide(77);}
@end
@interface FakePinEvent:NSObject
@property id window;
@property NSEventType type;
@property NSPoint locationInWindow;
@end
@implementation FakePinEvent
@end
int main(void){@autoreleasepool{
 NSRect display=NSMakeRect(-1920,-200,1920,1080);
 NSRect initial=clampMediaFrame(NSMakeRect(-2100,-500,360,360),display);
 NSCAssert(NSEqualRects(initial,NSMakeRect(-1920,-200,360,360)),@"negative-origin clamp");
 NSRect resize=clampMediaFrame(NSMakeRect(-100,-100,2400,1500),display);
 NSCAssert(NSEqualRects(resize,display),@"oversized logical clamp");
 NSRect retina=clampMediaFrame(NSMakeRect(1400,700,360,360),NSMakeRect(0,0,1512,944));
 NSCAssert(NSEqualRects(retina,NSMakeRect(1152,584,360,360)),@"logical points unaffected by scale");
 NSRect disconnect=clampMediaFrame(initial,NSMakeRect(0,0,1440,900));NSCAssert(disconnect.origin.x==0&&disconnect.origin.y==0,@"disconnect clamp");
 NSArray *screens=@[@{@"uuid":@"left",@"frame":[NSValue valueWithRect:display]},@{@"uuid":@"main",@"frame":[NSValue valueWithRect:NSMakeRect(0,0,1440,900)]}];
 NSCAssert(mediaScreenIndex(initial,@"left",screens,1)==0,@"connected display not retained");
 NSCAssert(mediaScreenIndex(initial,@"disconnected",screens,1)==1,@"disconnected display not mapped to main");
 NSCAssert(mediaScreenIndex(initial,nil,screens,1)==0,@"negative display initial selection");
 NSCAssert(mediaScreenIndex(initial,nil,@[],0)==NSNotFound,@"empty topology accepted");
 NSSize size=NSMakeSize(360,400);NSCAssert(mediaHeaderHit(NSMakePoint(10,390),size),@"header refused");NSCAssert(!mediaHeaderHit(NSMakePoint(340,390),size),@"close region admitted");NSCAssert(!mediaHeaderHit(NSMakePoint(10,300),size),@"body admitted");
 FakePinPanel *panel=[FakePinPanel new];panel.visible=YES;panel.frame=NSMakeRect(0,0,360,400);panel.contentView=[[NSView alloc]initWithFrame:panel.frame];
 OTMediaPin *record=[OTMediaPin new];record.panel=(NSPanel *)panel;record.session=99;record.visible=YES;record.pending=@{@"old":@YES};pins()[@77]=record;
 FakePinEvent *event=[FakePinEvent new];event.type=NSEventTypeLeftMouseDown;event.window=[NSObject new];event.locationInWindow=NSMakePoint(10,390);
 NSCAssert(!OTMediaHeaderEvent(77,(NSEvent *)event)&&panel.drags==0,@"other window admitted");
 event.window=panel;event.locationInWindow=NSMakePoint(340,390);NSCAssert(!OTMediaHeaderEvent(77,(NSEvent *)event),@"close control starts drag");
 event.locationInWindow=NSMakePoint(10,390);NSCAssert(OTMediaHeaderEvent(77,(NSEvent *)event)&&panel.drags==1,@"own header refused");
 NSCAssert(!record.visible&&!record.dragging&&!record.pending,@"hide during drag did not retire ownership and queued event");
 OTMediaDestroy(77,YES);NSCAssert(!pins()[@77],@"host destruction retained owner");char *terminal=ot_media_panel_next(77);NSCAssert(terminal&&strstr(terminal,"hostClosed"),@"host closed missing");free(terminal);NSCAssert(!ot_media_panel_next(77),@"terminal delivered twice");
 printf("PASS native media panel seam: negative origins, resize, Retina logical bounds, disconnect clamp, header/body/close/other-window exclusion, hide during simulated drag, terminal host destruction; no visible panel or input\n");return 0;
}}
