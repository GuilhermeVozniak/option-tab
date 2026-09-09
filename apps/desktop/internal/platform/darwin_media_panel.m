//go:build darwin

#import "darwin_media_panel.h"
#import <CoreGraphics/CoreGraphics.h>

@interface OTMediaPin : NSObject
@property(weak) NSPanel *panel;
@property uint64_t session,sequence;
@property BOOL visible,dragging,reported;
@property NSRect lastFrame;
@property NSString *display;
@property NSDictionary *pending;
@property id moveObserver,topologyObserver;
@end
@implementation OTMediaPin
@end
static NSMutableDictionary<NSNumber *,OTMediaPin *> *pins(void){
 static NSMutableDictionary *records;static dispatch_once_t once;
 dispatch_once(&once,^{records=[NSMutableDictionary new];});return records;
}
static NSMutableDictionary<NSNumber *,NSDictionary *> *terminalPins(void){
 static NSMutableDictionary *records;static dispatch_once_t once;
 dispatch_once(&once,^{records=[NSMutableDictionary new];});return records;
}
static void mediaMain(void (^work)(void)){
 if(NSThread.isMainThread){work();}else{dispatch_sync(dispatch_get_main_queue(),work);}
}
static NSString *displayUUID(NSScreen *screen){
 NSNumber *number=screen.deviceDescription[@"NSScreenNumber"];
 if(!number)return @"";
 CFUUIDRef uuid=CGDisplayCreateUUIDFromDisplayID(number.unsignedIntValue);
 if(!uuid)return @"";
 NSString *value=CFBridgingRelease(CFUUIDCreateString(NULL,uuid));CFRelease(uuid);return value;
}
static NSRect clampMediaFrame(NSRect frame,NSRect visible){
 frame.size.width=MIN(frame.size.width,visible.size.width);frame.size.height=MIN(frame.size.height,visible.size.height);
 frame.origin.x=MAX(NSMinX(visible),MIN(frame.origin.x,NSMaxX(visible)-frame.size.width));
 frame.origin.y=MAX(NSMinY(visible),MIN(frame.origin.y,NSMaxY(visible)-frame.size.height));return frame;
}
static NSUInteger mediaScreenIndex(NSRect frame,NSString *preferred,NSArray<NSDictionary *> *screens,NSUInteger mainIndex){
 if(!screens.count)return NSNotFound;
 mainIndex=MIN(mainIndex,screens.count-1);
 if(preferred.length){for(NSUInteger index=0;index<screens.count;index++)if([screens[index][@"uuid"]isEqual:preferred])return index;return mainIndex;}
 for(NSUInteger index=0;index<screens.count;index++)if(NSPointInRect(NSMakePoint(NSMidX(frame),NSMidY(frame)),[screens[index][@"frame"]rectValue]))return index;
 return mainIndex;
}
static NSScreen *chooseScreen(NSRect frame,NSString *preferred){
 NSArray<NSScreen *> *screens=NSScreen.screens;NSMutableArray *snapshot=[NSMutableArray new];NSUInteger mainIndex=0;
 for(NSUInteger index=0;index<screens.count;index++){NSScreen *screen=screens[index];if(screen==NSScreen.mainScreen)mainIndex=index;[snapshot addObject:@{@"uuid":displayUUID(screen),@"frame":[NSValue valueWithRect:screen.frame]}];}
 NSUInteger index=mediaScreenIndex(frame,preferred,snapshot,mainIndex);return index==NSNotFound?nil:screens[index];
}
static NSDictionary *pinEvent(OTMediaPin *record,NSString *reason){
 NSRect frame=record.panel.frame;
 CGFloat top=NSMaxY(NSScreen.screens.firstObject.frame);
 return @{@"session":@(record.session),@"sequence":@(++record.sequence),@"bounds":@{@"X":@(frame.origin.x),@"Y":@(top-NSMaxY(frame)),@"W":@(frame.size.width),@"H":@(frame.size.height)},@"displayUUID":record.display?:@"",@"reason":reason};
}
static void publishPin(OTMediaPin *record,NSString *reason){
 if(!record.visible)return;
 if(record.reported&&NSEqualRects(record.lastFrame,record.panel.frame)&&![reason isEqual:@"topology"])return;
 record.lastFrame=record.panel.frame;record.reported=YES;record.pending=pinEvent(record,reason);
}
static void resizePinContent(NSPanel *panel){panel.contentView.frame=NSMakeRect(0,0,panel.frame.size.width,panel.frame.size.height);for(NSView *view in panel.contentView.subviews)view.frame=panel.contentView.bounds;}
void OTMediaAttach(uint64_t token,uint64_t session,NSPanel *panel){
 OTMediaPin *r=[OTMediaPin new];r.panel=panel;r.session=session;
 r.moveObserver=[NSNotificationCenter.defaultCenter addObserverForName:NSWindowDidMoveNotification object:panel queue:nil usingBlock:^(NSNotification *note){
  (void)note;OTMediaPin *live=pins()[@(token)];if(!live||!live.visible)return;
  NSScreen *screen=chooseScreen(live.panel.frame,nil);live.display=displayUUID(screen);publishPin(live,@"moved");
 }];
 r.topologyObserver=[NSNotificationCenter.defaultCenter addObserverForName:NSApplicationDidChangeScreenParametersNotification object:nil queue:nil usingBlock:^(NSNotification *note){
  (void)note;OTMediaPin *live=pins()[@(token)];if(!live)return;live.dragging=NO;
  NSScreen *screen=chooseScreen(live.panel.frame,live.display);if(!screen)return;
  live.display=displayUUID(screen);[live.panel setFrame:clampMediaFrame(live.panel.frame,screen.visibleFrame) display:live.visible];resizePinContent(live.panel);publishPin(live,@"topology");
 }];pins()[@(token)]=r;
}
NSRect OTMediaPrepare(uint64_t token,NSRect frame){
 OTMediaPin *r=pins()[@(token)];if(!r)return frame;
 NSScreen *screen=chooseScreen(frame,r.display);if(screen){r.display=displayUUID(screen);frame=clampMediaFrame(frame,screen.visibleFrame);}r.visible=YES;return frame;
}
static BOOL mediaHeaderHit(NSPoint point,NSSize size){return point.x>=0&&point.x<size.width-48&&point.y>=size.height-32&&point.y<=size.height;}
BOOL OTMediaHeaderEvent(uint64_t token,NSEvent *event){
 OTMediaPin *r=pins()[@(token)];if(!r||!r.visible||!r.panel.visible||r.dragging||event.window!=r.panel||event.type!=NSEventTypeLeftMouseDown||!mediaHeaderHit(event.locationInWindow,r.panel.contentView.bounds.size))return NO;
 r.dragging=YES;[r.panel performWindowDragWithEvent:event];
 if(pins()[@(token)]==r&&r.visible){r.dragging=NO;NSScreen *screen=chooseScreen(r.panel.frame,nil);r.display=displayUUID(screen);[r.panel setFrame:clampMediaFrame(r.panel.frame,screen.visibleFrame) display:YES];publishPin(r,@"moved");}
 return YES;
}
void OTMediaShown(uint64_t token){OTMediaPin *r=pins()[@(token)];if(r)publishPin(r,@"moved");}
void OTMediaHide(uint64_t token){OTMediaPin *r=pins()[@(token)];if(r){r.visible=NO;r.dragging=NO;r.pending=nil;r.reported=NO;}}
void OTMediaDestroy(uint64_t token,BOOL hostClosed){
 OTMediaPin *r=pins()[@(token)];if(!r)return;
 if(hostClosed)terminalPins()[@(token)]=pinEvent(r,@"hostClosed");else[terminalPins()removeObjectForKey:@(token)];
 r.visible=NO;r.dragging=NO;r.pending=nil;
 if(r.moveObserver)[NSNotificationCenter.defaultCenter removeObserver:r.moveObserver];if(r.topologyObserver)[NSNotificationCenter.defaultCenter removeObserver:r.topologyObserver];
 [pins()removeObjectForKey:@(token)];
}
char *ot_media_panel_next(uint64_t token){
 __block char *result=NULL;mediaMain(^{
  OTMediaPin *r=pins()[@(token)];NSDictionary *event=r.pending?:terminalPins()[@(token)];r.pending=nil;[terminalPins()removeObjectForKey:@(token)];
  if(event){NSData *data=[NSJSONSerialization dataWithJSONObject:event options:0 error:nil];result=strdup([[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding].UTF8String);}
 });return result;
}

void ot_media_panel_forget(uint64_t token){mediaMain(^{[terminalPins()removeObjectForKey:@(token)];});}
