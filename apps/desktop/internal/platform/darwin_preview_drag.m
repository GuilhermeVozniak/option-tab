//go:build darwin
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <libproc.h>
#import <pthread.h>
#import <stdatomic.h>
#import <unistd.h>
#import "darwin.h"
#import "darwin_preview_drag.h"

extern AXError _AXUIElementGetWindow(AXUIElementRef,CGWindowID *);
typedef struct {
 atomic_bool cancelled;
 pthread_mutex_t mutex;
 OTPreviewDragSample sample;
 int test,button,writes;
 uint64_t testScreens,testRegistrationScreens;
 double finalX,finalY;
 int pid;
 uint32_t window;
 uint64_t startSec,startUsec;
 atomic_uint_fast64_t displayToken;
 AXUIElementRef target;
 AXObserverRef observer;
 CFMachPortRef tap;
 CFRunLoopSourceRef tapSource;
 uint64_t screens;
 CFAbsoluteTime lastCheck;
} PreviewDragOwner;
static atomic_uint_fast64_t displayTokens=0,activeDisplayToken=0,invalidDisplayToken=0;
static BOOL dragProcess(PreviewDragOwner *s) {
 struct proc_bsdinfo info={0};
 return proc_pidinfo(s->pid,PROC_PIDTBSDINFO,0,&info,sizeof(info))==sizeof(info)&&info.pbi_start_tvsec==s->startSec&&info.pbi_start_tvusec==s->startUsec;
}
static uint64_t dragScreens(void) {
 CGDirectDisplayID ids[32];uint32_t count=0;
 if(CGGetActiveDisplayList(32,ids,&count)!=kCGErrorSuccess||count==0)return 0;
 uint64_t hash=1469598103934665603ULL;
 for(uint32_t i=0;i<count;i++) {
  CGRect b=CGDisplayBounds(ids[i]);double values[]={b.origin.x,b.origin.y,b.size.width,b.size.height};
  hash=(hash^ids[i])*1099511628211ULL;
  for(int j=0;j<4;j++){uint64_t bits=0;memcpy(&bits,&values[j],sizeof(bits));hash=(hash^bits)*1099511628211ULL;}
 }
 return hash;
}
static BOOL dragTopologyCurrent(PreviewDragOwner *s) {
 uint64_t current=s->test?s->testScreens:dragScreens();
 if(!current||current!=s->screens){atomic_store(&s->cancelled,true);return NO;}return YES;
}
static void dragCheckRelease(PreviewDragOwner *s) {
 BOOL down=s->test?s->button:CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState,kCGMouseButtonLeft);
 if(down)return;
 // An already captured up owns its final destination. A missing/passed up
 // cancels conservatively, without inventing a terminal event or click.
 pthread_mutex_lock(&s->mutex);
 if(!s->sample.released)atomic_store(&s->cancelled,true);
 pthread_mutex_unlock(&s->mutex);
}
static void dragDisplayChanged(CGDirectDisplayID display,CGDisplayChangeSummaryFlags flags,void *context) {
 uint64_t token=(uint64_t)(uintptr_t)context;
 if(token==atomic_load(&activeDisplayToken))atomic_store(&invalidDisplayToken,token);
}
static BOOL dragCancelled(PreviewDragOwner *s) {
 return atomic_load(&s->cancelled)||(s->displayToken&&atomic_load(&invalidDisplayToken)==s->displayToken);
}
static CFTypeRef dragAttribute(PreviewDragOwner *s,AXUIElementRef node,CFStringRef name,CFAbsoluteTime deadline) {
 if(dragCancelled(s))return NULL;double remaining=deadline-CFAbsoluteTimeGetCurrent();if(remaining<=0)return NULL;
 AXUIElementSetMessagingTimeout(node,MIN(0.03,remaining));CFTypeRef value=NULL;
 if(AXUIElementCopyAttributeValue(node,name,&value)!=kAXErrorSuccess){if(value)CFRelease(value);return NULL;}return value;
}
static BOOL dragBoolean(PreviewDragOwner *s,CFStringRef name,BOOL *out,CFAbsoluteTime deadline) {
 CFTypeRef value=dragAttribute(s,s->target,name,deadline);
 BOOL valid=value&&CFGetTypeID(value)==CFBooleanGetTypeID();if(valid)*out=CFBooleanGetValue(value);if(value)CFRelease(value);return valid;
}
static BOOL dragRoot(PreviewDragOwner *s,AXUIElementRef node,CFAbsoluteTime deadline) {
 if(!node||dragCancelled(s))return NO;
 pid_t pid=0;CGWindowID wid=0;AXUIElementSetMessagingTimeout(node,0.03);
 if(AXUIElementGetPid(node,&pid)!=kAXErrorSuccess||pid!=s->pid||_AXUIElementGetWindow(node,&wid)!=kAXErrorSuccess||wid!=s->window)return NO;
 CFTypeRef role=dragAttribute(s,node,kAXRoleAttribute,deadline);
 BOOL root=role&&CFGetTypeID(role)==CFStringGetTypeID()&&CFEqual(role,kAXWindowRole);if(role)CFRelease(role);return root;
}
static BOOL dragWritable(PreviewDragOwner *s,CFAbsoluteTime deadline) {
 if(dragCancelled(s)||!dragProcess(s)||ot_window_pid(s->window)!=s->pid||!dragRoot(s,s->target,deadline))return NO;
 BOOL minimized=NO,fullscreen=NO;Boolean writable=false;
 if(!dragBoolean(s,kAXMinimizedAttribute,&minimized,deadline)||!dragBoolean(s,CFSTR("AXFullScreen"),&fullscreen,deadline)||minimized||fullscreen)return NO;
 if(dragCancelled(s)||CFAbsoluteTimeGetCurrent()>=deadline)return NO;
 AXUIElementSetMessagingTimeout(s->target,MIN(0.03,deadline-CFAbsoluteTimeGetCurrent()));
 return AXUIElementIsAttributeSettable(s->target,kAXPositionAttribute,&writable)==kAXErrorSuccess&&writable;
}
static void dragDestroyed(AXObserverRef observer,AXUIElementRef element,CFStringRef notification,void *context) {
 PreviewDragOwner *s=context;atomic_store(&s->cancelled,true);
}
void *ot_preview_drag_create(int test) {
 PreviewDragOwner *s=calloc(1,sizeof(*s));if(!s)return NULL;
 atomic_init(&s->cancelled,false);atomic_init(&s->displayToken,0);pthread_mutex_init(&s->mutex,NULL);s->test=test;s->button=1;s->screens=1;s->testScreens=1;return s;
}
int ot_preview_drag_prepare(void *owner,uint32_t window,int pid,OTPreviewDragSize *size) {
 PreviewDragOwner *s=owner;
 if(dragCancelled(s))return -1;
 if(s->test){s->window=window;s->pid=pid;size->width=800;size->height=600;return 0;}
 if(pid<=0||pid==getpid()||!window||!AXIsProcessTrusted())return -2;
 s->pid=pid;s->window=window;
 struct proc_bsdinfo info={0};
 if(proc_pidinfo(pid,PROC_PIDTBSDINFO,0,&info,sizeof(info))!=sizeof(info)||ot_window_pid(window)!=pid)return -3;
 s->startSec=info.pbi_start_tvsec;s->startUsec=info.pbi_start_tvusec;
 s->screens=dragScreens();if(!s->screens)return -8;
 CFAbsoluteTime deadline=CFAbsoluteTimeGetCurrent()+0.25;
 AXUIElementRef app=AXUIElementCreateApplication(pid);if(!app)return -4;
 CFTypeRef windows=dragAttribute(s,app,kAXWindowsAttribute,deadline);CFRelease(app);
 if(windows&&CFGetTypeID(windows)==CFArrayGetTypeID()) {
  CFArrayRef roots=windows;
  for(CFIndex i=0;i<CFArrayGetCount(roots)&&i<128&&CFAbsoluteTimeGetCurrent()<deadline&&!dragCancelled(s);i++) {
   CFTypeRef item=CFArrayGetValueAtIndex(roots,i);
   if(CFGetTypeID(item)==AXUIElementGetTypeID()&&dragRoot(s,(AXUIElementRef)item,deadline)){s->target=(AXUIElementRef)CFRetain(item);break;}
  }
 }
 if(windows)CFRelease(windows);
 if(dragCancelled(s))return -1;if(!s->target)return -4;
 if(!dragWritable(s,deadline))return -5;
 CFTypeRef position=dragAttribute(s,s->target,kAXPositionAttribute,deadline),extent=dragAttribute(s,s->target,kAXSizeAttribute,deadline);
 CGPoint point;CGSize dimensions;
 BOOL valid=position&&extent&&CFGetTypeID(position)==AXValueGetTypeID()&&CFGetTypeID(extent)==AXValueGetTypeID()&&AXValueGetType(position)==kAXValueCGPointType&&AXValueGetType(extent)==kAXValueCGSizeType&&AXValueGetValue(position,kAXValueCGPointType,&point)&&AXValueGetValue(extent,kAXValueCGSizeType,&dimensions)&&isfinite(point.x)&&isfinite(point.y)&&isfinite(dimensions.width)&&isfinite(dimensions.height)&&dimensions.width>0&&dimensions.height>0;
 if(position)CFRelease(position);if(extent)CFRelease(extent);
 if(!valid)return -4;size->width=dimensions.width;size->height=dimensions.height;
 if(AXObserverCreate(pid,dragDestroyed,&s->observer)==kAXErrorSuccess&&s->observer) {
  if(AXObserverAddNotification(s->observer,s->target,kAXUIElementDestroyedNotification,s)==kAXErrorSuccess)CFRunLoopAddSource(CFRunLoopGetCurrent(),AXObserverGetRunLoopSource(s->observer),kCFRunLoopDefaultMode);
  else {CFRelease(s->observer);s->observer=NULL;}
 }
 return dragCancelled(s)?-1:0;
}
static CGEventRef previewDragCallback(CGEventTapProxy proxy,CGEventType type,CGEventRef event,void *context) {
 PreviewDragOwner *s=context;
 if(type==kCGEventTapDisabledByTimeout||type==kCGEventTapDisabledByUserInput){atomic_store(&s->cancelled,true);return event;}
 if(!event||dragCancelled(s))return event;
 if(CGEventGetIntegerValueField(event,kCGEventSourceUnixProcessID)!=0)return event;
 if(type==kCGEventKeyDown&&CGEventGetIntegerValueField(event,kCGKeyboardEventKeycode)==53){atomic_store(&s->cancelled,true);return NULL;}
 if(type!=kCGEventLeftMouseDragged&&type!=kCGEventLeftMouseUp)return event;
 CGPoint point=CGEventGetLocation(event);
 if(!isfinite(point.x)||!isfinite(point.y)){atomic_store(&s->cancelled,true);return event;}
 pthread_mutex_lock(&s->mutex);
 if(s->sample.released){pthread_mutex_unlock(&s->mutex);return event;}
 s->sample.sequence++;s->sample.x=point.x;s->sample.y=point.y;s->sample.released=type==kCGEventLeftMouseUp;
 pthread_mutex_unlock(&s->mutex);return NULL;
}
int ot_preview_drag_start(void *owner) {
 PreviewDragOwner *s=owner;
 if(dragCancelled(s))return -1;
 if(s->test){
  if(!s->button)return -6;if(!dragTopologyCurrent(s))return -8;
  if(s->testRegistrationScreens)s->testScreens=s->testRegistrationScreens;
  return dragTopologyCurrent(s)?0:-8;
 }
 if(!dragProcess(s)||!CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState,kCGMouseButtonLeft))return -6;
 if(!dragTopologyCurrent(s))return -8;
 s->displayToken=atomic_fetch_add(&displayTokens,1)+1;atomic_store(&activeDisplayToken,s->displayToken);
 if(CGDisplayRegisterReconfigurationCallback(dragDisplayChanged,(void *)(uintptr_t)s->displayToken)!=kCGErrorSuccess)return -8;
 if(dragCancelled(s))return -1;
 if(!dragTopologyCurrent(s))return -8;
 CGEventMask mask=CGEventMaskBit(kCGEventLeftMouseDragged)|CGEventMaskBit(kCGEventLeftMouseUp)|CGEventMaskBit(kCGEventKeyDown);
 s->tap=CGEventTapCreate(kCGSessionEventTap,kCGHeadInsertEventTap,kCGEventTapOptionDefault,mask,previewDragCallback,s);
 if(!s->tap)return -7;
 s->tapSource=CFMachPortCreateRunLoopSource(kCFAllocatorDefault,s->tap,0);if(!s->tapSource)return -7;
 CFRunLoopAddSource(CFRunLoopGetCurrent(),s->tapSource,kCFRunLoopCommonModes);CGEventTapEnable(s->tap,true);
 if(!dragTopologyCurrent(s)||dragCancelled(s)||!CGEventSourceButtonState(kCGEventSourceStateCombinedSessionState,kCGMouseButtonLeft))return -1;
 CGEventRef current=CGEventCreate(NULL);if(!current)return -7;
 CGPoint point=CGEventGetLocation(current);CFRelease(current);
 pthread_mutex_lock(&s->mutex);s->sample.sequence++;s->sample.x=point.x;s->sample.y=point.y;pthread_mutex_unlock(&s->mutex);
 return 0;
}
void ot_preview_drag_check(void *owner) {
 PreviewDragOwner *s=owner;
 dragCheckRelease(s);if(!dragTopologyCurrent(s)||dragCancelled(s)||s->test)return;
 CFRunLoopRunInMode(kCFRunLoopDefaultMode,0,true);
 CFAbsoluteTime now=CFAbsoluteTimeGetCurrent();
 if(now-s->lastCheck>=0.1){
  s->lastCheck=now;
  if(!AXIsProcessTrusted()||!dragWritable(s,now+0.15))atomic_store(&s->cancelled,true);
 }

}
void ot_preview_drag_pump(void *owner) {CFRunLoopRunInMode(kCFRunLoopDefaultMode,0.005,true);}
void ot_preview_drag_stop_tap(void *owner) {
 PreviewDragOwner *s=owner;
 if(s->tapSource){CFRunLoopRemoveSource(CFRunLoopGetCurrent(),s->tapSource,kCFRunLoopCommonModes);CFRelease(s->tapSource);s->tapSource=NULL;}
 if(s->tap){CGEventTapEnable(s->tap,false);CFMachPortInvalidate(s->tap);CFRelease(s->tap);s->tap=NULL;}
 if(s->displayToken){atomic_store(&activeDisplayToken,0);CGDisplayRemoveReconfigurationCallback(dragDisplayChanged,(void *)(uintptr_t)s->displayToken);}
}
OTPreviewDragSample ot_preview_drag_sample(void *owner) {
 PreviewDragOwner *s=owner;
 pthread_mutex_lock(&s->mutex);OTPreviewDragSample sample=s->sample;pthread_mutex_unlock(&s->mutex);
 sample.cancelled=dragCancelled(s);return sample;
}
int ot_preview_drag_position(void *owner,double x,double y) {
 PreviewDragOwner *s=owner;
 if(!isfinite(x)||!isfinite(y))return -5;
 dragCheckRelease(s);
 if(!dragTopologyCurrent(s)||dragCancelled(s))return -1;
 if(s->test){s->finalX=x;s->finalY=y;s->writes++;return 0;}
 CFRunLoopRunInMode(kCFRunLoopDefaultMode,0,true);
 if(!dragWritable(s,CFAbsoluteTimeGetCurrent()+0.15))return dragCancelled(s)?-1:-3;
 CGPoint destination=CGPointMake(x,y);AXValueRef value=AXValueCreate(kAXValueCGPointType,&destination);if(!value)return -9;
 // The only mutation in this source. No size/fullscreen/Space writes, focus,
 // activation, event replay, snapping, or implicit restoration are performed.
 dragCheckRelease(s);
 if(!dragTopologyCurrent(s)||dragCancelled(s)){CFRelease(value);return -1;}
 AXUIElementSetMessagingTimeout(s->target,0.03);
 AXError result=AXUIElementSetAttributeValue(s->target,kAXPositionAttribute,value);CFRelease(value);
 return result==kAXErrorSuccess?0:-9;
}
void ot_preview_drag_cancel(void *owner){PreviewDragOwner *s=owner;atomic_store(&s->cancelled,true);}
void ot_preview_drag_destroy(void *owner) {
 PreviewDragOwner *s=owner;if(!s)return;
 if(s->observer){CFRunLoopRemoveSource(CFRunLoopGetCurrent(),AXObserverGetRunLoopSource(s->observer),kCFRunLoopDefaultMode);if(s->target)AXObserverRemoveNotification(s->observer,s->target,kAXUIElementDestroyedNotification);CFRelease(s->observer);}
 if(s->target)CFRelease(s->target);pthread_mutex_destroy(&s->mutex);free(s);
}
int ot_preview_drag_test_event(void *owner,int type,double x,double y,int key) {
 PreviewDragOwner *s=owner;if(!s||!s->test)return -1;
 if((uint32_t)type==kCGEventTapDisabledByTimeout){previewDragCallback(NULL,(CGEventType)type,NULL,s);return 0;}
 CGEventRef event=type==kCGEventKeyDown?CGEventCreateKeyboardEvent(NULL,key,true):CGEventCreateMouseEvent(NULL,type,CGPointMake(x,y),kCGMouseButtonLeft);
 CGEventSetIntegerValueField(event,kCGEventSourceUnixProcessID,0);
 CGEventRef result=previewDragCallback(NULL,type,event,s);CFRelease(event);return result==NULL;
}
void ot_preview_drag_test_button(void *owner,int down){PreviewDragOwner *s=owner;if(s->test)s->button=down;}
int ot_preview_drag_test_writes(void *owner){return ((PreviewDragOwner *)owner)->writes;}

void ot_preview_drag_test_screens(void *owner,uint64_t current,uint64_t registration){PreviewDragOwner *s=owner;if(s->test){s->testScreens=current;s->testRegistrationScreens=registration;}}

int ot_preview_drag_test_synthetic_up(void *owner) {
 PreviewDragOwner *s=owner;if(!s||!s->test)return -1;
 CGEventRef event=CGEventCreateMouseEvent(NULL,kCGEventLeftMouseUp,CGPointMake(20,30),kCGMouseButtonLeft);
 CGEventSetIntegerValueField(event,kCGEventSourceUnixProcessID,123);
 CGEventRef result=previewDragCallback(NULL,kCGEventLeftMouseUp,event,s);CFRelease(event);return result==NULL;
}
