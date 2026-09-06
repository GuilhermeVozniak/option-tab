#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import "native.h"
#include <unistd.h>
extern AXError _AXUIElementGetWindow(AXUIElementRef element,CGWindowID *window);
static void onMain(void(^block)(void)){if(NSThread.isMainThread){@autoreleasepool{block();}}else dispatch_sync(dispatch_get_main_queue(),^{@autoreleasepool{block();}});}
int smoke_activate(int pid){__block int ok=0;onMain(^{NSRunningApplication *app=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];ok=[app activateWithOptions:NSApplicationActivateIgnoringOtherApps];});return ok;}
int smoke_foreground(void){__block int pid=0;onMain(^{pid=NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;});return pid;}
uint32_t smoke_focused_window(int pid){AXUIElementRef app=AXUIElementCreateApplication(pid);CFTypeRef value=NULL;CGWindowID wid=0;if(AXUIElementCopyAttributeValue(app,kAXFocusedWindowAttribute,&value)==kAXErrorSuccess&&value){_AXUIElementGetWindow((AXUIElementRef)value,&wid);CFRelease(value);}CFRelease(app);return wid;}
void smoke_click(double x,double y){CGEventRef down=CGEventCreateMouseEvent(NULL,kCGEventLeftMouseDown,CGPointMake(x,y),kCGMouseButtonLeft);CGEventRef up=CGEventCreateMouseEvent(NULL,kCGEventLeftMouseUp,CGPointMake(x,y),kCGMouseButtonLeft);CGEventSetIntegerValueField(down,kCGMouseEventClickState,1);CGEventSetIntegerValueField(up,kCGMouseEventClickState,1);CGEventPost(kCGHIDEventTap,down);usleep(50000);CGEventPost(kCGHIDEventTap,up);CFRelease(down);CFRelease(up);}
void smoke_key(void){CGEventRef down=CGEventCreateKeyboardEvent(NULL,7,true);CGEventRef up=CGEventCreateKeyboardEvent(NULL,7,false);CGEventPost(kCGHIDEventTap,down);CGEventPost(kCGHIDEventTap,up);CFRelease(down);CFRelease(up);}
static NSArray *smoke_views(NSView *view){NSMutableArray *out=[NSMutableArray new];NSMutableArray *q=[NSMutableArray arrayWithObject:view];while(q.count&&out.count<40){NSView*v=q[0];[q removeObjectAtIndex:0];NSMutableArray*areas=[NSMutableArray new];for(NSTrackingArea*a in v.trackingAreas)[areas addObject:@{@"options":@(a.options),@"owner":NSStringFromClass([a.owner class])?:@"nil"}];[out addObject:@{@"class":NSStringFromClass(v.class),@"tracking":areas}];[q addObjectsFromArray:v.subviews];}return out;}
char *smoke_state(void *pointer){__block char *result=NULL;onMain(^{NSWindow *host=(__bridge NSWindow *)pointer;NSMutableArray *panels=[NSMutableArray new];NSMutableArray *screens=[NSMutableArray new];for(NSScreen *screen in NSScreen.screens){[screens addObject:@{@"scale":@(screen.backingScaleFactor),@"name":screen.localizedName}];}for(NSWindow *window in NSApp.windows){if([NSStringFromClass(window.class) isEqualToString:@"OTDockPanel"]){NSRect f=window.frame;[panels addObject:@{@"visible":@(window.visible),@"key":@(window.keyWindow),@"main":@(window.mainWindow),@"x":@(f.origin.x),@"y":@(f.origin.y),@"width":@(f.size.width),@"height":@(f.size.height),@"scale":@(window.backingScaleFactor),@"mouseMoved":@(window.acceptsMouseMovedEvents),@"views":smoke_views(window.contentView)}];}}NSData *json=[NSJSONSerialization dataWithJSONObject:@{@"hostVisible":@(host.visible),@"hostContentWidth":@(host.contentView.frame.size.width),@"hostContentHeight":@(host.contentView.frame.size.height),@"panels":panels,@"screenCount":@(NSScreen.screens.count),@"screens":screens} options:0 error:nil];result=strdup([[NSString alloc]initWithData:json encoding:NSUTF8StringEncoding].UTF8String);});return result;}

int smoke_screen(int index,double *x,double *y){__block int ok=0;onMain(^{if(index>=0&&index<NSScreen.screens.count){NSRect f=NSScreen.screens[index].frame;*x=NSMinX(f);*y=NSMaxY(NSScreen.screens.firstObject.frame)-NSMaxY(f);ok=1;}});return ok;}
int smoke_position_fixture(int pid,double x,double y){AXUIElementRef app=AXUIElementCreateApplication(pid);CFTypeRef window=NULL;int ok=0;if(AXUIElementCopyAttributeValue(app,kAXFocusedWindowAttribute,&window)==kAXErrorSuccess&&window){CGPoint point=CGPointMake(x,y);AXValueRef value=AXValueCreate(kAXValueCGPointType,&point);ok=AXUIElementSetAttributeValue((AXUIElementRef)window,kAXPositionAttribute,value)==kAXErrorSuccess;CFRelease(value);CFRelease(window);}CFRelease(app);return ok;}
void smoke_pointer(double *x,double *y){CGEventRef event=CGEventCreate(NULL);CGPoint point=CGEventGetLocation(event);*x=point.x;*y=point.y;CFRelease(event);}
void smoke_warp(double x,double y){CGPoint point=CGPointMake(x,y);CGEventSourceRef source=CGEventSourceCreate(kCGEventSourceStateHIDSystemState);CGEventSourceSetLocalEventsSuppressionInterval(source,0);CGEventRef moved=CGEventCreateMouseEvent(source,kCGEventMouseMoved,point,kCGMouseButtonLeft);CGEventPost(kCGSessionEventTap,moved);CFRelease(moved);CFRelease(source);}
static id smoke_ax_attr(AXUIElementRef element,CFStringRef key){CFTypeRef value=NULL;if(AXUIElementCopyAttributeValue(element,key,&value)!=kAXErrorSuccess||!value)return @"";return CFBridgingRelease(value);}
char *smoke_fixture_surfaces(int pid){@autoreleasepool{
 NSMutableArray *cg=[NSMutableArray new],*ax=[NSMutableArray new];NSArray *info=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionAll,kCGNullWindowID));
 for(NSDictionary*w in info)if([w[(__bridge NSString*)kCGWindowOwnerPID] intValue]==pid)[cg addObject:@{@"id":w[(__bridge NSString*)kCGWindowNumber]?:@0,@"title":w[(__bridge NSString*)kCGWindowName]?:@"",@"layer":w[(__bridge NSString*)kCGWindowLayer]?:@0,@"alpha":w[(__bridge NSString*)kCGWindowAlpha]?:@0,@"onscreen":w[(__bridge NSString*)kCGWindowIsOnscreen]?:@NO,@"bounds":w[(__bridge NSString*)kCGWindowBounds]?:@{}}];
 AXUIElementRef app=AXUIElementCreateApplication(pid);AXUIElementSetMessagingTimeout(app,.2);id windows=smoke_ax_attr(app,kAXWindowsAttribute);if([windows isKindOfClass:NSArray.class])for(id value in windows){AXUIElementRef w=(__bridge AXUIElementRef)value;AXUIElementSetMessagingTimeout(w,.1);CGWindowID wid=0;_AXUIElementGetWindow(w,&wid);[ax addObject:@{@"id":@(wid),@"role":smoke_ax_attr(w,kAXRoleAttribute),@"subrole":smoke_ax_attr(w,kAXSubroleAttribute),@"title":smoke_ax_attr(w,kAXTitleAttribute)}];}CFRelease(app);
 NSData*data=[NSJSONSerialization dataWithJSONObject:@{@"pid":@(pid),@"cg":cg,@"axWindows":ax} options:0 error:nil];return strdup([[NSString alloc]initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
}}

void smoke_tracking_diagnostic(void *host) {onMain(^{
 for(NSWindow *panel in NSApp.windows) if([NSStringFromClass(panel.class) isEqualToString:@"OTDockPanel"] && panel.visible){
 NSPoint point=[panel convertPointFromScreen:NSEvent.mouseLocation];
 NSView *root=panel.contentView;NSView *hit=[root hitTest:[root.superview convertPoint:point fromView:nil]];
 NSLog(@"TRACKING DIAG panel=%@ root=%@ super=%@ hit=%@ point=%@",panel,root,root.superview,hit,NSStringFromPoint(point));
 NSMutableArray *queue=[NSMutableArray arrayWithObject:root];
 while(queue.count){NSView *v=queue[0];[queue removeObjectAtIndex:0];
 for(NSTrackingArea *area in v.trackingAreas)if((area.options&NSTrackingMouseMoved) && [area.owner respondsToSelector:@selector(mouseMoved:)]){
 NSEvent *event=[NSEvent mouseEventWithType:NSEventTypeMouseMoved location:point modifierFlags:0 timestamp:NSProcessInfo.processInfo.systemUptime windowNumber:panel.windowNumber context:nil eventNumber:0 clickCount:0 pressure:0];
 NSLog(@"TRACKING OWNER %@ view=%@ window=%@ descendant=%d",area.owner,v,v.window,[hit isDescendantOf:v]);
 (void)event;
 }[queue addObjectsFromArray:v.subviews];}
 }
 });}
