//go:build darwin

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <stdatomic.h>
#import "darwin_dock.h"

extern uint64_t ot_active_space(void);
enum { DockInvalidate = 1, DockGeometry = 2, DockInventory = 4 };

static NSDictionary *DockRect(CGRect r) { return @{@"X":@(r.origin.x),@"Y":@(r.origin.y),@"W":@(r.size.width),@"H":@(r.size.height)}; }
static BOOL DockValidRect(CGRect r) { return isfinite(r.origin.x)&&isfinite(r.origin.y)&&isfinite(r.size.width)&&isfinite(r.size.height)&&r.size.width>0&&r.size.height>0; }
static NSString *DockCanonicalPath(NSURL *url) {
  if (!url.isFileURL) return nil;
  NSURL *normalized = url.URLByStandardizingPath.URLByResolvingSymlinksInPath;
  return normalized.path.stringByStandardizingPath;
}
static CFTypeRef DockCopyAttribute(AXUIElementRef node, CFStringRef name, CFAbsoluteTime deadline, BOOL *invalid) {
  double remaining = deadline-CFAbsoluteTimeGetCurrent();
  if (remaining <= 0) return NULL;
  AXUIElementSetMessagingTimeout(node, (float)MIN(0.05,remaining));
  CFTypeRef value = NULL;
  AXError error = AXUIElementCopyAttributeValue(node,name,&value);
  if (error == kAXErrorInvalidUIElement) *invalid=YES;
  if (error != kAXErrorSuccess) { if(value)CFRelease(value);return NULL; }
  return value;
}
static NSString *DockStringAttribute(AXUIElementRef node, CFStringRef name, CFAbsoluteTime deadline, BOOL *invalid) {
  CFTypeRef value=DockCopyAttribute(node,name,deadline,invalid);
  NSString *result=value&&CFGetTypeID(value)==CFStringGetTypeID()?[(__bridge NSString *)value copy]:nil;
  if(value)CFRelease(value);return result;
}
static CGRect DockElementBounds(AXUIElementRef node,CFAbsoluteTime deadline,BOOL *invalid) {
  CFTypeRef position=DockCopyAttribute(node,kAXPositionAttribute,deadline,invalid);
  CFTypeRef size=DockCopyAttribute(node,kAXSizeAttribute,deadline,invalid);
  CGPoint point=CGPointZero;CGSize extent=CGSizeZero;
  BOOL good=position&&size&&CFGetTypeID(position)==AXValueGetTypeID()&&CFGetTypeID(size)==AXValueGetTypeID()&&
    AXValueGetType((AXValueRef)position)==kAXValueCGPointType&&AXValueGetType((AXValueRef)size)==kAXValueCGSizeType&&
    AXValueGetValue((AXValueRef)position,kAXValueCGPointType,&point)&&AXValueGetValue((AXValueRef)size,kAXValueCGSizeType,&extent);
  if(position)CFRelease(position);if(size)CFRelease(size);
  return good?CGRectMake(point.x,point.y,extent.width,extent.height):CGRectZero;
}

@interface OTDockObservation : NSObject {
@public
  atomic_bool alive;
  atomic_uint dirty;
  AXUIElementRef dockAX;
  AXUIElementRef systemAX;
  AXObserverRef axObserver;
  CFRunLoopRef workerLoop;
  pid_t dockPID;
  uint64_t generation;
  uint64_t space;
  CFAbsoluteTime nextProbe;
  CFAbsoluteTime nextAttach;
  CGRect knownBounds;
}
@property(nonatomic) NSMutableArray *tokens;
@property(nonatomic) NSArray *screens;
@property(nonatomic) NSArray *apps;
@property(nonatomic) NSString *status;
@property(nonatomic) NSString *orientation;
@property(nonatomic) NSString *lastPath;
@property(nonatomic) NSString *lastBundle;
@property(nonatomic) NSString *diagnostic;
- (void)detachAX;
- (void)invalidate:(unsigned)reason;
- (void)stop;
- (NSDictionary *)poll;
@end

static void DockAXChanged(AXObserverRef observer,AXUIElementRef element,CFStringRef notification,void *context) {
  OTDockObservation *owner=(__bridge OTDockObservation *)context;
  [owner invalidate:CFEqual(notification,kAXUIElementDestroyedNotification)?DockInvalidate:DockGeometry];
}
@implementation OTDockObservation
- (instancetype)init {
  if((self=[super init])) {
    atomic_init(&alive,true);atomic_init(&dirty,DockInvalidate|DockInventory);
    workerLoop=CFRunLoopGetCurrent();CFRetain(workerLoop);
    systemAX=AXUIElementCreateSystemWide();AXUIElementSetMessagingTimeout(systemAX,0.05);
    self.tokens=[NSMutableArray array];self.screens=@[];self.apps=@[];self.status=@"dockUnavailable";self.orientation=@"bottom";
    __weak OTDockObservation *weakSelf=self;
    NSNotificationCenter *workspace=NSWorkspace.sharedWorkspace.notificationCenter;
    for(NSString *name in @[NSWorkspaceDidLaunchApplicationNotification,NSWorkspaceDidTerminateApplicationNotification]) {
      id token=[workspace addObserverForName:name object:nil queue:nil usingBlock:^(NSNotification *note){
        OTDockObservation *owner=weakSelf;if(!owner)return;
        NSRunningApplication *app=note.userInfo[NSWorkspaceApplicationKey];
        [owner invalidate:[app.bundleIdentifier isEqualToString:@"com.apple.dock"]?DockInvalidate|DockInventory:DockInventory];
      }];[self.tokens addObject:@[workspace,token]];
    }
    for(NSString *name in @[NSWorkspaceActiveSpaceDidChangeNotification,NSWorkspaceDidWakeNotification]) {
      id token=[workspace addObserverForName:name object:nil queue:nil usingBlock:^(NSNotification *note){[weakSelf invalidate:DockInvalidate|DockInventory];}];
      [self.tokens addObject:@[workspace,token]];
    }
    NSNotificationCenter *center=NSNotificationCenter.defaultCenter;
    id token=[center addObserverForName:NSApplicationDidChangeScreenParametersNotification object:nil queue:nil usingBlock:^(NSNotification *note){[weakSelf invalidate:DockInvalidate];}];
    [self.tokens addObject:@[center,token]];
  }
  return self;
}
- (void)invalidate:(unsigned)reason {if(atomic_load(&alive))atomic_fetch_or(&dirty,reason);}
- (void)detachAX {
  if(axObserver){CFRunLoopRemoveSource(workerLoop,AXObserverGetRunLoopSource(axObserver),kCFRunLoopDefaultMode);CFRelease(axObserver);axObserver=NULL;}
  if(dockAX){CFRelease(dockAX);dockAX=NULL;}
  knownBounds=CGRectZero;
}
- (void)attachAX {
  if(dockPID<=0)return;
  dockAX=AXUIElementCreateApplication(dockPID);AXUIElementSetMessagingTimeout(dockAX,0.05);
  if(AXObserverCreate(dockPID,DockAXChanged,&axObserver)==kAXErrorSuccess&&axObserver){
    CFRunLoopAddSource(workerLoop,AXObserverGetRunLoopSource(axObserver),kCFRunLoopDefaultMode);
    // Dock versions need not expose every notification; the timer remains authoritative.
    for(NSString *notification in @[@"AXUIElementDestroyed",@"AXMoved",@"AXResized",@"AXChildrenChanged"])
      AXObserverAddNotification(axObserver,dockAX,(__bridge CFStringRef)notification,(__bridge void *)self);
  }
}
- (NSArray *)readScreens {
  CGDirectDisplayID displays[32];uint32_t count=0;
  if(CGGetActiveDisplayList(32,displays,&count)!=kCGErrorSuccess)return @[];
  NSMutableArray *result=[NSMutableArray array];
  for(uint32_t i=0;i<count;i++) {
    CGRect bounds=CGDisplayBounds(displays[i]);
    if(!DockValidRect(bounds))continue;
    double scale=(double)CGDisplayPixelsWide(displays[i])/bounds.size.width;
    [result addObject:@{@"id":@(displays[i]),@"bounds":DockRect(bounds),@"scale":@(scale)}];
  }
  return result;
}
- (void)refreshApps {
  NSMutableArray *result=[NSMutableArray array];
  for(NSRunningApplication *app in NSWorkspace.sharedWorkspace.runningApplications) {
    if(app.terminated||app.processIdentifier==getpid()||app.activationPolicy!=NSApplicationActivationPolicyRegular)continue;
    NSString *path=DockCanonicalPath(app.bundleURL);if(!path)continue;
    [result addObject:@{@"pid":@(app.processIdentifier),@"path":path,@"bundleId":app.bundleIdentifier?:@""}];
    if(result.count>=512)break;
  }
  self.apps=result;
}
- (BOOL)nearDockEdge:(CGPoint)point {
  if(DockValidRect(knownBounds)&&CGRectContainsPoint(CGRectInset(knownBounds,-8,-8),point))return YES;
  for(NSDictionary *screen in self.screens) {
    NSDictionary *b=screen[@"bounds"];CGRect r=CGRectMake([b[@"X"] doubleValue],[b[@"Y"] doubleValue],[b[@"W"] doubleValue],[b[@"H"] doubleValue]);
    if(CGRectContainsPoint(r,point)&&
       (point.x<=CGRectGetMinX(r)+160||point.x>=CGRectGetMaxX(r)-160||point.y>=CGRectGetMaxY(r)-160))return YES;
  }
  return NO;
}
- (NSDictionary *)hitItem:(CGPoint)point {
  CFAbsoluteTime deadline=CFAbsoluteTimeGetCurrent()+0.15;BOOL invalid=NO;self.diagnostic=@"AX classification yielded no application icon";
  AXUIElementRef node=NULL;
  AXUIElementSetMessagingTimeout(systemAX,0.05);
  AXError hit=AXUIElementCopyElementAtPosition(systemAX,point.x,point.y,&node);
  if(hit!=kAXErrorSuccess||!node){self.diagnostic=[NSString stringWithFormat:@"AX hit test error %d",hit];if(node)CFRelease(node);if(hit==kAXErrorInvalidUIElement)[self invalidate:DockInvalidate];return nil;}
  NSDictionary *result=nil;
  for(int parents=0;parents<6&&node&&CFAbsoluteTimeGetCurrent()<deadline;parents++) {
    pid_t owner=0;if(AXUIElementGetPid(node,&owner)!=kAXErrorSuccess||owner!=dockPID){self.diagnostic=@"AX hit owner is not Dock";break;}
    NSString *role=DockStringAttribute(node,kAXRoleAttribute,deadline,&invalid);
    NSString *subrole=DockStringAttribute(node,kAXSubroleAttribute,deadline,&invalid);
    if([role isEqualToString:(__bridge NSString *)kAXDockItemRole]) {
      if(![subrole isEqualToString:(__bridge NSString *)kAXApplicationDockItemSubrole]){self.diagnostic=@"Dock item is not an application subrole";break;}
      CFTypeRef rawURL=DockCopyAttribute(node,kAXURLAttribute,deadline,&invalid);
      NSURL *url=rawURL&&CFGetTypeID(rawURL)==CFURLGetTypeID()?[(__bridge NSURL *)rawURL copy]:nil;
      if(rawURL)CFRelease(rawURL);
      CGRect bounds=DockElementBounds(node,deadline,&invalid);
      NSString *title=DockStringAttribute(node,kAXTitleAttribute,deadline,&invalid)?:@"";
      CFTypeRef parent=DockCopyAttribute(node,kAXParentAttribute,deadline,&invalid);
      CGRect container=CGRectZero;
      if(parent&&CFGetTypeID(parent)==AXUIElementGetTypeID())container=DockElementBounds((AXUIElementRef)parent,deadline,&invalid);
      if(parent)CFRelease(parent);
      if(!url.isFileURL||!DockValidRect(bounds)||invalid||CFAbsoluteTimeGetCurrent()>=deadline){self.diagnostic=[NSString stringWithFormat:@"AX app attributes rejected: url=%d bounds=%d invalid=%d deadline=%d",url.isFileURL,DockValidRect(bounds),invalid,CFAbsoluteTimeGetCurrent()>=deadline];break;}
      NSString *path=DockCanonicalPath(url);
      if(!path||![path.pathExtension.lowercaseString isEqualToString:@"app"]){self.diagnostic=@"Dock URL is not an app bundle";break;}
      NSString *bundle=nil;
      for(NSDictionary *app in self.apps)if([app[@"path"] isEqualToString:path]){bundle=app[@"bundleId"];break;}
      if(!bundle.length) {
        if(![self.lastPath isEqualToString:path]){self.lastPath=path;self.lastBundle=[NSBundle bundleWithURL:[NSURL fileURLWithPath:path]].bundleIdentifier?:@"";}
        bundle=self.lastBundle?:@"";
      }
      NSMutableArray *matches=[NSMutableArray array];
      // Resolve the hovered bundle against live processes, never a stale PID
      // cache or a localized Dock title. Duplicate launches remain ambiguous.
      NSArray *candidates=bundle.length?[NSRunningApplication runningApplicationsWithBundleIdentifier:bundle]:@[];
      for(NSRunningApplication *app in candidates) {
        if(app.terminated||app.processIdentifier==getpid()||app.activationPolicy!=NSApplicationActivationPolicyRegular)continue;
        NSString *livePath=DockCanonicalPath(app.bundleURL);if(!livePath)continue;
        [matches addObject:@{@"pid":@(app.processIdentifier),@"path":livePath,@"bundleId":app.bundleIdentifier?:@""}];
      }
      if(CFAbsoluteTimeGetCurrent()>=deadline){self.diagnostic=@"identity resolution exhausted classification deadline";break;}
      if(DockValidRect(container))knownBounds=container;
      self.diagnostic=@"application icon classified";
      result=@{@"pid":@(dockPID),@"role":role,@"subrole":subrole,@"url":url.absoluteString,@"path":path,@"bundleId":bundle?:@"",@"title":title,@"bounds":DockRect(bounds),@"container":DockRect(container),@"apps":matches};
      break;
    }
    CFTypeRef parent=DockCopyAttribute(node,kAXParentAttribute,deadline,&invalid);
    CFRelease(node);node=NULL;
    if(parent&&CFGetTypeID(parent)==AXUIElementGetTypeID())node=(AXUIElementRef)parent;else if(parent)CFRelease(parent);
  }
  if(node)CFRelease(node);
  if(invalid)[self invalidate:DockInvalidate];
  return result;
}
- (NSDictionary *)poll {
  if(!atomic_load(&alive))return nil;
  // Deliver supported AX notifications only on the owning worker run loop.
  CFRunLoopRunInMode(kCFRunLoopDefaultMode,0,true);
  CGEventRef event=CGEventCreate(NULL);CGPoint point=event?CGEventGetLocation(event):CGPointZero;if(event)CFRelease(event);
  CFAbsoluteTime now=CFAbsoluteTimeGetCurrent();unsigned changes=atomic_exchange(&dirty,0);
  if(now>=nextProbe) {
    nextProbe=now+1.0;
    BOOL permission=AXIsProcessTrusted();pid_t currentPID=0;
    if(permission)for(NSRunningApplication *app in [NSRunningApplication runningApplicationsWithBundleIdentifier:@"com.apple.dock"])if(!app.terminated){currentPID=app.processIdentifier;break;}
    NSString *status=!permission?@"permissionDenied":currentPID?@"ready":@"dockUnavailable";
    NSArray *screens=[self readScreens];uint64_t currentSpace=ot_active_space();
    CFTypeRef setting=CFPreferencesCopyAppValue(CFSTR("orientation"),CFSTR("com.apple.dock"));
    NSString *orientation=setting&&CFGetTypeID(setting)==CFStringGetTypeID()?[(__bridge NSString *)setting copy]:@"bottom";if(setting)CFRelease(setting);
    if(currentPID!=dockPID||![status isEqualToString:self.status]||![screens isEqual:self.screens]||space!=currentSpace||![orientation isEqualToString:self.orientation])changes|=DockInvalidate;
    dockPID=currentPID;self.status=status;self.screens=screens;space=currentSpace;self.orientation=orientation;
    changes|=DockInventory;
  }
  if(changes&DockInventory)[self refreshApps];
  BOOL invalidated=(changes&DockInvalidate)!=0;
  if(invalidated){generation++;[self detachAX];if(systemAX)CFRelease(systemAX);systemAX=AXUIElementCreateSystemWide();AXUIElementSetMessagingTimeout(systemAX,0.05);}
  if(!dockAX&&[self.status isEqualToString:@"ready"]&&now>=nextAttach){[self attachAX];nextAttach=now+1.0;}
  else if(changes&DockGeometry)knownBounds=CGRectZero;
  NSMutableDictionary *out=[@{@"generation":@(generation),@"dockPid":@(dockPID),@"pointerX":@(point.x),@"pointerY":@(point.y),@"status":self.status,@"screens":self.screens,@"orientation":self.orientation} mutableCopy];
  if(invalidated){if([self.status isEqualToString:@"ready"])out[@"status"]=@"dockUnavailable";return out;}
  if([self.status isEqualToString:@"ready"]&&!dockAX){out[@"status"]=@"dockUnavailable";return out;}
  if([self.status isEqualToString:@"ready"]&&dockAX&&[self nearDockEdge:point]) {
    NSDictionary *item=[self hitItem:point];if(item)out[@"item"]=item;out[@"diagnostic"]=self.diagnostic?:@"";
  }
  if(!out[@"diagnostic"])out[@"diagnostic"]=@"pointer outside Dock edge gate";
  return out;
}
- (void)stop {
  if(!atomic_exchange(&alive,false))return;
  for(NSArray *entry in self.tokens)[entry[0] removeObserver:entry[1]];
  [self.tokens removeAllObjects];[self detachAX];
  if(systemAX){CFRelease(systemAX);systemAX=NULL;}
  if(workerLoop){CFRelease(workerLoop);workerLoop=NULL;}
  self.apps=@[];self.screens=@[];
}
@end

void *ot_dock_observer_create(void) {@autoreleasepool{return (__bridge_retained void *)[OTDockObservation new];}}
char *ot_dock_observer_poll(void *observer) {
  @autoreleasepool {
    OTDockObservation *owner=(__bridge OTDockObservation *)observer;
    NSDictionary *observation=[owner poll];if(!observation)return NULL;
    NSData *data=[NSJSONSerialization dataWithJSONObject:observation options:0 error:nil];if(!data)return NULL;
    char *json=calloc(data.length+1,1);if(json)memcpy(json,data.bytes,data.length);return json;
  }
}
void ot_dock_observer_destroy(void *observer) {@autoreleasepool{OTDockObservation *owner=(__bridge_transfer OTDockObservation *)observer;[owner stop];}}
