#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
static CFTypeRef attr(AXUIElementRef node,CFStringRef name){AXUIElementSetMessagingTimeout(node,0.05);CFTypeRef value=NULL;if(AXUIElementCopyAttributeValue(node,name,&value)!=kAXErrorSuccess){if(value)CFRelease(value);return NULL;}return value;}
int main(int argc,char **argv){@autoreleasepool {
 if(argc==4&&strcmp(argv[1],"--move")==0){CGPoint point=CGPointMake(atof(argv[2]),atof(argv[3]));CGWarpMouseCursorPosition(point);CGEventRef moved=CGEventCreateMouseEvent(NULL,kCGEventMouseMoved,point,kCGMouseButtonLeft);CGEventPost(kCGHIDEventTap,moved);CFRelease(moved);return 0;}
 CGEventRef event=CGEventCreate(NULL);CGPoint pointer=CGEventGetLocation(event);CFRelease(event);
 if(argc==2&&strcmp(argv[1],"--pointer")==0){printf("{\"x\":%f,\"y\":%f}\n",pointer.x,pointer.y);return 0;}
 if(argc!=2)return 2;
 NSString *wanted=[NSString stringWithUTF8String:argv[1]];NSArray *docks=[NSRunningApplication runningApplicationsWithBundleIdentifier:@"com.apple.dock"];if(docks.count!=1)return 3;
 pid_t dockPID=[docks[0] processIdentifier];AXUIElementRef app=AXUIElementCreateApplication(dockPID);
 NSMutableArray *queue=[NSMutableArray arrayWithObject:@{ @"node":CFBridgingRelease(app),@"depth":@0 }];CFAbsoluteTime deadline=CFAbsoluteTimeGetCurrent()+4;int visited=0;
 while(queue.count&&visited++<512&&CFAbsoluteTimeGetCurrent()<deadline){
  NSDictionary *entry=queue[0];[queue removeObjectAtIndex:0];AXUIElementRef node=(__bridge AXUIElementRef)entry[@"node"];int depth=[entry[@"depth"] intValue];
  CFTypeRef role=attr(node,kAXRoleAttribute);CFTypeRef subrole=attr(node,kAXSubroleAttribute);CFTypeRef value=attr(node,kAXURLAttribute);
  NSURL *url=value&&CFGetTypeID(value)==CFURLGetTypeID()?(__bridge NSURL *)value:nil;
  BOOL match=role&&CFGetTypeID(role)==CFStringGetTypeID()&&CFEqual(role,kAXDockItemRole)&&subrole&&CFGetTypeID(subrole)==CFStringGetTypeID()&&CFEqual(subrole,kAXApplicationDockItemSubrole)&&url.isFileURL&&[url.URLByStandardizingPath.URLByResolvingSymlinksInPath.path isEqualToString:wanted];
  if(role)CFRelease(role);if(subrole)CFRelease(subrole);if(value)CFRelease(value);
  if(match){CFTypeRef pos=attr(node,kAXPositionAttribute),size=attr(node,kAXSizeAttribute);CGPoint p;CGSize s;
   BOOL valid=pos&&size&&CFGetTypeID(pos)==AXValueGetTypeID()&&CFGetTypeID(size)==AXValueGetTypeID()&&AXValueGetValue((AXValueRef)pos,kAXValueCGPointType,&p)&&AXValueGetValue((AXValueRef)size,kAXValueCGSizeType,&s);
   if(pos)CFRelease(pos);if(size)CFRelease(size);if(!valid)return 4;
   CGDirectDisplayID displays[32];uint32_t count=0;CGGetActiveDisplayList(32,displays,&count);CGRect screen=CGRectZero;double distance=DBL_MAX;
   CGPoint center=CGPointMake(p.x+s.width/2,p.y+s.height/2);
   for(uint32_t i=0;i<count;i++){CGRect r=CGDisplayBounds(displays[i]);double dx=MAX(MAX(CGRectGetMinX(r)-center.x,0),center.x-CGRectGetMaxX(r));double dy=MAX(MAX(CGRectGetMinY(r)-center.y,0),center.y-CGRectGetMaxY(r));double d=dx*dx+dy*dy;if(d<distance){distance=d;screen=r;}}
   printf("{\"dockPID\":%d,\"x\":%f,\"y\":%f,\"w\":%f,\"h\":%f,\"pointerX\":%f,\"pointerY\":%f,\"screenX\":%f,\"screenY\":%f,\"screenW\":%f,\"screenH\":%f}\n",dockPID,p.x,p.y,s.width,s.height,pointer.x,pointer.y,screen.origin.x,screen.origin.y,screen.size.width,screen.size.height);return 0;
  }
  if(depth>=6)continue;
  CFTypeRef children=attr(node,kAXChildrenAttribute);if(children&&CFGetTypeID(children)==CFArrayGetTypeID())for(id child in (__bridge NSArray *)children){if(CFGetTypeID((__bridge CFTypeRef)child)==AXUIElementGetTypeID())[queue addObject:@{@"node":child,@"depth":@(depth+1)}];}
  if(children)CFRelease(children);
 }
 fprintf(stderr,"disposable Dock app icon was not AX-discoverable\n");return 5;
}}
