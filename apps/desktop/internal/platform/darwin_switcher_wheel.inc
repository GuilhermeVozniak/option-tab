// Included only by darwin_dock_panel.m: reuse its bounded classifier without
// replacing the existing Wails overlay or duplicating input ownership rules.
#import "darwin_switcher_wheel.h"
#define OT_SWITCHER_WHEEL_AVAILABLE 1
@interface OTSwitcherWheelRecord : NSObject
@property NSWindow *host;
@property NSView *content;
@property NSRect bounds;
@property OTDockPanelRecord *wheel;
@property id monitor, workspaceObserver;
@property NSMutableArray *observers;
@end
@implementation OTSwitcherWheelRecord
@end
static NSMutableDictionary<NSNumber *, OTSwitcherWheelRecord *> *switcherWheelRecords(void) {
 static NSMutableDictionary *records; static dispatch_once_t once;
 dispatch_once(&once, ^{ records=[NSMutableDictionary new]; }); return records;
}
static NSMutableDictionary<NSNumber *, OTPanelWheelMailbox *> *retiredSwitcherWheelMailboxes(void) {
 static NSMutableDictionary *records; static dispatch_once_t once;
 dispatch_once(&once, ^{ records=[NSMutableDictionary new]; }); return records;
}
static BOOL switcherWheelHostCurrent(OTSwitcherWheelRecord *r) {
 return r && r.host.contentView==r.content && r.host.visible && r.host.onActiveSpace &&
  !r.host.miniaturized && (r.host.occlusionState & NSWindowOcclusionStateVisible) &&
  NSEqualRects(r.bounds,r.content.bounds);
}
static void retireSwitcherWheel(OTSwitcherWheelRecord *r, int reason) {
 if (!r) return;
 cancelWheel(r.wheel,reason); r.wheel.wheelVisible=NO;
}
// A hidden/occluded host may become visible without changing DOM geometry.
// Restore only the exact content/bounds that were admitted by its latest policy;
// a resize requires a fresh geometry publication. Retirement never revives an
// old gesture token, even when the existing regions remain valid.
static void refreshSwitcherWheelVisibility(OTSwitcherWheelRecord *r, BOOL retire) {
 if (!r) return;
 if (retire || !switcherWheelHostCurrent(r)) retireSwitcherWheel(r,2);
 r.wheel.wheelVisible=switcherWheelHostCurrent(r);
}
static BOOL handleSwitcherWheel(OTSwitcherWheelRecord *r, NSEvent *event) {
 if (!r || event.type!=NSEventTypeScrollWheel || event.window!=r.host) return NO;
 if (!switcherWheelHostCurrent(r)) { retireSwitcherWheel(r,2); return NO; }
 if (!r.wheel.wheelEnabled || !event.hasPreciseScrollingDeltas) return NO;
 NSPoint p=[r.content convertPoint:event.locationInWindow fromView:nil];
 p.x-=NSMinX(r.content.bounds);
 p.y=r.content.flipped ? p.y-NSMinY(r.content.bounds) : NSMaxY(r.content.bounds)-p.y;
 OTPanelWheelInput i={.x=p.x,.y=p.y,.dx=event.scrollingDeltaX,.dy=event.scrollingDeltaY,
  .timestamp=event.timestamp,.unixTime=NSDate.date.timeIntervalSince1970-NSProcessInfo.processInfo.systemUptime+event.timestamp,
  .precise=event.hasPreciseScrollingDeltas,.inverted=event.isDirectionInvertedFromDevice,
  .phase=wheelPhase(event.phase),.momentum=wheelPhase(event.momentumPhase)};
 return admitWheel(r.wheel,i);
}
static void closeSwitcherWheel(uint64_t token) {
 OTSwitcherWheelRecord *r=switcherWheelRecords()[@(token)]; if(!r) return;
 retireSwitcherWheel(r,3);
 [switcherWheelRecords() removeObjectForKey:@(token)];
 if(r.monitor) { [NSEvent removeMonitor:r.monitor]; r.monitor=nil; }
 for(id observer in r.observers) [NSNotificationCenter.defaultCenter removeObserver:observer];
 [r.observers removeAllObjects];
 if(r.workspaceObserver) [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:r.workspaceObserver];
 r.workspaceObserver=nil;
 if(r.wheel.wheelMailbox && r.wheel.wheelMailbox->count) retiredSwitcherWheelMailboxes()[@(token)]=r.wheel.wheelMailbox;
 r.wheel.content=nil; r.content=nil; r.host=nil;
}
uint64_t ot_switcher_wheel_create(void *pointer) {
 if(!pointer) return 0;
 __block uint64_t token=0;
 panelMain(^{
  NSWindow *host=nil;
  for(NSWindow *candidate in NSApp.windows) if((__bridge void *)candidate==pointer) { host=candidate; break; }
  if(!host || !host.contentView || switcherWheelRecords().count>=8) return;
  for(OTSwitcherWheelRecord *other in switcherWheelRecords().allValues) if(other.host==host) return;
  static uint64_t next=0; if(next==UINT64_MAX) return;
  token=++next;
  OTSwitcherWheelRecord *r=[OTSwitcherWheelRecord new]; r.host=host; r.content=host.contentView; r.bounds=r.content.bounds;
  r.wheel=[OTDockPanelRecord new]; r.wheel.content=r.content; r.observers=[NSMutableArray new];
  switcherWheelRecords()[@(token)]=r;
  uint64_t owned=token;
  r.monitor=[NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskScrollWheel handler:^NSEvent *(NSEvent *event){
    return handleSwitcherWheel(switcherWheelRecords()[@(owned)],event) ? nil : event;
  }];
  if(!r.monitor) { closeSwitcherWheel(token); token=0; return; }
  for(NSNotificationName name in @[NSWindowWillCloseNotification,NSWindowDidResizeNotification,NSWindowDidMoveNotification,NSWindowDidChangeOcclusionStateNotification]) {
   id observer=[NSNotificationCenter.defaultCenter addObserverForName:name object:host queue:nil usingBlock:^(NSNotification *note){
    panelMain(^{
     OTSwitcherWheelRecord *current=switcherWheelRecords()[@(owned)];
     if([note.name isEqualToString:NSWindowWillCloseNotification]) closeSwitcherWheel(owned);
     else refreshSwitcherWheelVisibility(current,![note.name isEqualToString:NSWindowDidChangeOcclusionStateNotification]);
    });
   }];
   [r.observers addObject:observer];
  }
  id displayObserver=[NSNotificationCenter.defaultCenter addObserverForName:NSApplicationDidChangeScreenParametersNotification object:nil queue:nil usingBlock:^(NSNotification *note){
   panelMain(^{ refreshSwitcherWheelVisibility(switcherWheelRecords()[@(owned)],YES); });
  }]; [r.observers addObject:displayObserver];
  r.workspaceObserver=[NSWorkspace.sharedWorkspace.notificationCenter addObserverForName:NSWorkspaceActiveSpaceDidChangeNotification object:nil queue:nil usingBlock:^(NSNotification *note){
   panelMain(^{ refreshSwitcherWheelVisibility(switcherWheelRecords()[@(owned)],YES); });
  }];
 }); return token;
}
int ot_switcher_wheel_close(uint64_t token) { panelMain(^{closeSwitcherWheel(token);}); return 1; }
int ot_switcher_wheel_policy(uint64_t token,const char *json) {
 @autoreleasepool {
  NSData *data=json ? [[NSString stringWithUTF8String:json] dataUsingEncoding:NSUTF8StringEncoding] : nil;
  NSDictionary *p=data ? [NSJSONSerialization JSONObjectWithData:data options:0 error:nil] : nil;
  if(![p isKindOfClass:NSDictionary.class] || ![p[@"regions"] isKindOfClass:NSArray.class]) return 0;
  for(NSString *key in @[@"session",@"revision",@"enabled"]) if(![p[key] isKindOfClass:NSNumber.class]) return 0;
  __block int result=0;
  panelMain(^{
   OTSwitcherWheelRecord *r=switcherWheelRecords()[@(token)]; if(!r) {result=-1;return;}
   if(r.host.contentView!=r.content) {closeSwitcherWheel(token);result=-1;return;}
   result=applyWheelPolicy(r.wheel,[p[@"session"] unsignedLongLongValue],[p[@"revision"] unsignedLongLongValue],[p[@"enabled"] boolValue],p[@"regions"]);
   if(result) { r.bounds=r.content.bounds; r.wheel.wheelVisible=switcherWheelHostCurrent(r); }
  }); return result;
 }
}
int ot_switcher_wheel_next(uint64_t token,OTDockPanelWheelEvent *event) {
 __block int result=0;
 panelMain(^{
  OTSwitcherWheelRecord *r=switcherWheelRecords()[@(token)];
  OTPanelWheelMailbox *m=r ? r.wheel.wheelMailbox : retiredSwitcherWheelMailboxes()[@(token)];
  if(m && m->count) { *event=m->events[m->head];m->head=(m->head+1)%129;m->count--;result=1; }
  else if(!r) { [retiredSwitcherWheelMailboxes() removeObjectForKey:@(token)];result=-1; }
 }); return result;
}
int ot_switcher_wheel_valid(uint64_t token,uint64_t session,uint64_t revision,uint64_t gesture) {
 __block int result=0;
 panelMain(^{
  OTSwitcherWheelRecord *r=switcherWheelRecords()[@(token)];
  if(!switcherWheelHostCurrent(r)) {retireSwitcherWheel(r,2);return;}
  OTDockPanelRecord *w=r.wheel;
  result=w.wheelVisible && w.wheelEnabled && gesture && w.wheelSession==session && w.wheelRevision==revision && w.wheelValidGesture==gesture;
 }); return result;
}
void ot_switcher_wheel_complete(uint64_t token,uint64_t session,uint64_t revision,uint64_t gesture) {
 panelMain(^{ acknowledgeWheel(switcherWheelRecords()[@(token)].wheel,session,revision,gesture); });
}
