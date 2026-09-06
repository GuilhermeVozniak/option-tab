//go:build darwin
#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <stdatomic.h>
#import <pthread.h>
#import <time.h>
#import <unistd.h>
#import <libproc.h>
#import "darwin_dock_input.h"

#define INPUT_QUEUE 64
#define INPUT_REPLAY 0x4f5444495245504cLL
static atomic_uint_fast64_t nextGesture=0, nextSequence=0;
static atomic_bool sourceRunning=false;
static atomic_int testAlive=0;
static atomic_uint_fast64_t syntheticPasses=0,installations=0;
static pthread_mutex_t tokenMutex=PTHREAD_MUTEX_INITIALIZER;
static uint64_t currentGeneration=0,currentGesture=0;
static BOOL currentCompleted=NO;
static int currentAppPID=0,currentDockPID=0;
static uint64_t currentAppSeconds=0,currentAppMicros=0,currentDockSeconds=0,currentDockMicros=0;
static atomic_uint_fast64_t sourceEpochCounter=0,sourceEpochCurrent=0;
static BOOL processStart(int pid,uint64_t *seconds,uint64_t *micros) {
 struct proc_bsdinfo info={0};
 if(pid<=0||proc_pidinfo(pid,PROC_PIDTBSDINFO,0,&info,sizeof(info))!=sizeof(info))return NO;
 *seconds=info.pbi_start_tvsec;*micros=info.pbi_start_tvusec;return YES;
}

static void tokenSet(uint64_t generation,uint64_t gesture) {
 pthread_mutex_lock(&tokenMutex);currentGeneration=generation;currentGesture=gesture;currentCompleted=NO;currentAppPID=0;currentDockPID=0;pthread_mutex_unlock(&tokenMutex);
}
int ot_dock_input_owned(uint64_t generation,uint64_t gesture) {
 pthread_mutex_lock(&tokenMutex);
 int result=generation&&gesture&&generation==currentGeneration&&gesture==currentGesture&&!currentCompleted;
 pthread_mutex_unlock(&tokenMutex);return result;
}

int ot_dock_input_current(uint64_t generation,uint64_t gesture) {
 pthread_mutex_lock(&tokenMutex);
 BOOL match=generation&&gesture&&generation==currentGeneration&&gesture==currentGesture&&!currentCompleted;
 int app=currentAppPID,dock=currentDockPID;
 uint64_t appS=currentAppSeconds,appU=currentAppMicros,dockS=currentDockSeconds,dockU=currentDockMicros;
 pthread_mutex_unlock(&tokenMutex);
 uint64_t actualS=0,actualU=0;
 if(!match||!processStart(app,&actualS,&actualU)||actualS!=appS||actualU!=appU||!processStart(dock,&actualS,&actualU)||actualS!=dockS||actualU!=dockU)return 0;
 pthread_mutex_lock(&tokenMutex);
 match=generation==currentGeneration&&gesture==currentGesture&&!currentCompleted&&app==currentAppPID&&dock==currentDockPID&&appS==currentAppSeconds&&appU==currentAppMicros&&dockS==currentDockSeconds&&dockU==currentDockMicros;
 pthread_mutex_unlock(&tokenMutex);return match;
}
static BOOL recordProcessIdentity(uint64_t generation,uint64_t gesture,int app,int dock,uint64_t appS,uint64_t appU,uint64_t dockS,uint64_t dockU) {
 pthread_mutex_lock(&tokenMutex);
 BOOL match=generation==currentGeneration&&gesture==currentGesture&&!currentCompleted;
 if(match) {
  // Never silently replace a previously captured process lifetime.
  if(currentAppPID&&(currentAppPID!=app||currentDockPID!=dock||currentAppSeconds!=appS||currentAppMicros!=appU||currentDockSeconds!=dockS||currentDockMicros!=dockU))match=NO;
  else {currentAppPID=app;currentDockPID=dock;currentAppSeconds=appS;currentAppMicros=appU;currentDockSeconds=dockS;currentDockMicros=dockU;}
 }
 pthread_mutex_unlock(&tokenMutex);return match;
}

static BOOL captureProcessIdentity(uint64_t generation,uint64_t gesture,int app,int dock) {
 uint64_t appS=0,appU=0,dockS=0,dockU=0;
 if(!processStart(app,&appS,&appU)||!processStart(dock,&dockS,&dockU))return NO;
 return recordProcessIdentity(generation,gesture,app,dock,appS,appU,dockS,dockU);
}

typedef struct {
 int click,scroll,right,test,replays,permissionAllowed,health,testHealthMask;
 double nextPermissionCheck;
 uint64_t sourceEpoch;
 CFTypeRef workspaceTokens;
 CFMachPortRef tap;
 CFRunLoopSourceRef source;
 OTDockInputTarget target,owned;
 uint64_t gesture;
 CGEventRef down;
 CGPoint origin;
 CGEventFlags modifiers;
 int button,validated,scrolling,pending,completed,bypassScroll,scrollTerminal;
 double scrollAt;
 OTDockInputRaw queue[INPUT_QUEUE];
 size_t head,count;
} InputOwner;

double ot_dock_input_now(void) {struct timespec ts;clock_gettime(CLOCK_MONOTONIC,&ts);return ts.tv_sec+ts.tv_nsec/1e9;}
static BOOL sameTarget(OTDockInputTarget a,OTDockInputTarget b) {
 return a.generation==b.generation&&a.dock_pid==b.dock_pid&&a.app_pid==b.app_pid&&a.screen==b.screen&&a.edge==b.edge&&!strcmp(a.path,b.path)&&!strcmp(a.bundle,b.bundle);
}
static BOOL freshTarget(OTDockInputTarget t,CGPoint p,double now) {
 return t.generation&&t.dock_pid>0&&t.app_pid>0&&t.app_pid!=getpid()&&t.path[0]&&t.bundle[0]&&strcmp(t.bundle,"com.apple.finder")&&t.expiry>=now&&t.expiry<=now+0.151&&isfinite(p.x)&&isfinite(p.y)&&isfinite(t.x)&&isfinite(t.y)&&isfinite(t.w)&&isfinite(t.h)&&t.w>0&&t.h>0&&p.x>=t.x&&p.y>=t.y&&p.x<t.x+t.w&&p.y<t.y+t.h;
}
static void clearQueue(InputOwner *s) {
 for(size_t i=0;i<s->count;i++){OTDockInputRaw *r=&s->queue[(s->head+i)%INPUT_QUEUE];if(r->event)CFRelease(r->event);}
 s->head=0;s->count=0;
}
static BOOL enqueue(InputOwner *s,CGEventRef event,int type,BOOL cancelled) {
 if(s->count>=INPUT_QUEUE)return NO;
 OTDockInputRaw raw={0};raw.event=event?(void *)CFRetain(event):NULL;raw.target=s->owned;raw.gesture=s->gesture;raw.sequence=atomic_fetch_add(&nextSequence,1)+1;raw.at=ot_dock_input_now();raw.type=type;raw.cancelled=cancelled;raw.validated=s->validated;
 s->queue[(s->head+s->count)%INPUT_QUEUE]=raw;s->count++;return YES;
}
static void replayDown(InputOwner *s,CGEventTapProxy proxy) {
 if(!s->down)return;
 CGEventSetIntegerValueField(s->down,kCGEventSourceUserData,INPUT_REPLAY);
 if(s->test)s->replays++;
 else if(proxy)CGEventTapPostEvent(proxy,s->down);
 else CGEventPost(kCGSessionEventTap,s->down);
 CFRelease(s->down);s->down=NULL;
}
static void cancelOwned(InputOwner *s,CGEventTapProxy proxy) {
 if(!s->gesture)return;
 tokenSet(0,0);
 replayDown(s,proxy);
 if(!enqueue(s,NULL,0,YES)){clearQueue(s);enqueue(s,NULL,0,YES);}
 s->gesture=0;s->validated=0;s->scrolling=0;s->pending=0;s->completed=0;s->scrollTerminal=0;
}
static CGEventFlags inputMods(CGEventRef event) {
 return CGEventGetFlags(event)&(kCGEventFlagMaskCommand|kCGEventFlagMaskAlternate|kCGEventFlagMaskShift|kCGEventFlagMaskControl|kCGEventFlagMaskSecondaryFn);
}
static BOOL rightMods(CGEventFlags f) {return f==kCGEventFlagMaskCommand||f==(kCGEventFlagMaskCommand|kCGEventFlagMaskAlternate);}
static BOOL synthetic(CGEventRef event) {
 return CGEventGetIntegerValueField(event,kCGEventSourceUserData)==INPUT_REPLAY||CGEventGetIntegerValueField(event,kCGEventSourceUnixProcessID)!=0||CGEventGetIntegerValueField(event,kCGEventSourceStateID)!=kCGEventSourceStateHIDSystemState;
}
static void applyCompletion(InputOwner *s) {
 if(s->gesture&&ot_dock_input_completed(s->owned.generation,s->gesture)) {
  s->pending=0;s->completed=1;
  if(!s->scrolling&&!s->down){s->gesture=0;tokenSet(0,0);}
 }
}
static CGEventRef inputCallback(CGEventTapProxy proxy,CGEventType type,CGEventRef event,void *context) {
 InputOwner *s=context;applyCompletion(s);
 if(type==kCGEventTapDisabledByTimeout||type==kCGEventTapDisabledByUserInput){cancelOwned(s,proxy);if(s->tap&&s->permissionAllowed)CGEventTapEnable(s->tap,true);return event;}
 if(!event||!s->permissionAllowed)return event;
 if(synthetic(event)){if(!s->test)atomic_fetch_add(&syntheticPasses,1);return event;}
 CGPoint point=CGEventGetLocation(event);double now=ot_dock_input_now();CGEventFlags mods=inputMods(event);
 BOOL valid=freshTarget(s->target,point,now);
 // Pointer/cache changes can abandon a held down, but cannot revoke a
 // completed exact-app action awaiting execution/refusal acknowledgement.
 if(s->down&&(!valid||!sameTarget(s->target,s->owned)))cancelOwned(s,proxy);
 if(s->down) {
  BOOL release=type==(s->button==0?kCGEventLeftMouseUp:kCGEventRightMouseUp);
  BOOL movement=type==kCGEventMouseMoved||type==kCGEventLeftMouseDragged||type==kCGEventRightMouseDragged;
  double dx=point.x-s->origin.x,dy=point.y-s->origin.y;
  if(mods!=s->modifiers||hypot(dx,dy)>6.0||(!release&&!movement)) {cancelOwned(s,proxy);return event;}
  if(release&&!s->validated){cancelOwned(s,proxy);return event;}
  if(!enqueue(s,event,type,NO)){cancelOwned(s,proxy);return event;}
  if(release){CFRelease(s->down);s->down=NULL;s->pending=1;}
  return NULL;
 }
 if(type==kCGEventLeftMouseDown||type==kCGEventRightMouseDown) {
  if(s->pending)return event;
  if(!valid||(!(type==kCGEventLeftMouseDown?s->click&&mods==0:s->right&&rightMods(mods))))return event;
  cancelOwned(s,proxy);
  s->owned=s->target;s->gesture=atomic_fetch_add(&nextGesture,1)+1;s->button=type==kCGEventLeftMouseDown?0:1;s->origin=point;s->modifiers=mods;s->validated=0;s->completed=0;s->pending=0;s->down=(CGEventRef)CFRetain(event);
  tokenSet(s->owned.generation,s->gesture);
  // This down is still being forwarded on failure; never replay a duplicate.
  if(!enqueue(s,event,type,NO)){CFRelease(s->down);s->down=NULL;cancelOwned(s,proxy);return event;}
  return NULL;
 }
 if(type==kCGEventScrollWheel) {
  int phase=(int)CGEventGetIntegerValueField(event,kCGScrollWheelEventScrollPhase);
  int momentum=(int)CGEventGetIntegerValueField(event,kCGScrollWheelEventMomentumPhase);
  double previous=s->scrollAt;s->scrollAt=now;
  BOOL began=(phase&kCGScrollPhaseBegan)!=0;
  BOOL idle=previous==0||now-previous>0.25;
  BOOL terminal=(phase&(kCGScrollPhaseEnded|kCGScrollPhaseCancelled))||momentum==kCGMomentumScrollPhaseEnd;
  if(s->bypassScroll) {
   if(began||idle)s->bypassScroll=0;
   else {if(terminal)s->bypassScroll=0;return event;}
  }
  if(s->scrolling&&(began||idle)) {
   // A consumed prior stream still owns the pending action token. Do not
   // consume the new stream; remember bypass so its changed events pass too.
   if(s->pending){s->scrolling=0;s->bypassScroll=!terminal;return event;}
   cancelOwned(s,proxy);
  }
  if(!s->scrolling) {
   if(s->pending){s->bypassScroll=!terminal;return event;}
   if(!valid||!s->scroll||mods!=0||momentum||(!began&&!idle)||(phase&~(kCGScrollPhaseBegan|kCGScrollPhaseChanged)))return event;
   s->owned=s->target;s->gesture=atomic_fetch_add(&nextGesture,1)+1;s->scrolling=1;s->pending=1;s->validated=0;s->completed=0;tokenSet(s->owned.generation,s->gesture);
  }
  if(mods!=0||(phase&~(kCGScrollPhaseBegan|kCGScrollPhaseChanged|kCGScrollPhaseEnded|kCGScrollPhaseCancelled))){cancelOwned(s,proxy);return event;}
  if(!s->completed&&!enqueue(s,event,type,NO)){cancelOwned(s,proxy);return event;}
  if(phase&kCGScrollPhaseCancelled){cancelOwned(s,proxy);return NULL;}
  if(terminal)s->scrollTerminal=1;
  if(momentum==kCGMomentumScrollPhaseEnd) {
   s->scrolling=0;
   if(!s->pending){s->gesture=0;tokenSet(0,0);}
  }
  // Keep stream ownership across phase-ended and subsequent momentum. It
  // retires after acknowledgement plus idle, or at the next fresh began.
  return NULL;
 }
 return event;
}

void *ot_dock_input_create(int click,int scroll,int right,int test) {
 if(!click&&!scroll&&!right)return NULL;
 if(!test&&atomic_exchange(&sourceRunning,true))return NULL;
 InputOwner *s=calloc(1,sizeof(*s));if(!s){if(!test)atomic_store(&sourceRunning,false);return NULL;}
 s->testHealthMask=15;s->click=click;s->scroll=scroll;s->right=right;s->test=test;s->permissionAllowed=test||AXIsProcessTrusted();
 if(test){atomic_fetch_add(&testAlive,1);return s;}
 if(!s->permissionAllowed){free(s);atomic_store(&sourceRunning,false);return NULL;}
 CGEventMask mask=CGEventMaskBit(kCGEventLeftMouseDown)|CGEventMaskBit(kCGEventLeftMouseUp)|CGEventMaskBit(kCGEventRightMouseDown)|CGEventMaskBit(kCGEventRightMouseUp)|CGEventMaskBit(kCGEventMouseMoved)|CGEventMaskBit(kCGEventLeftMouseDragged)|CGEventMaskBit(kCGEventRightMouseDragged)|CGEventMaskBit(kCGEventScrollWheel);
 s->tap=CGEventTapCreate(kCGSessionEventTap,kCGHeadInsertEventTap,kCGEventTapOptionDefault,mask,inputCallback,s);
 if(!s->tap){free(s);atomic_store(&sourceRunning,false);return NULL;}
 s->source=CFMachPortCreateRunLoopSource(kCFAllocatorDefault,s->tap,0);
 if(!s->source){CFRelease(s->tap);free(s);atomic_store(&sourceRunning,false);return NULL;}
 CFRunLoopAddSource(CFRunLoopGetCurrent(),s->source,kCFRunLoopCommonModes);CGEventTapEnable(s->tap,true);
 @autoreleasepool {
  uint64_t epoch=atomic_fetch_add(&sourceEpochCounter,1)+1;s->sourceEpoch=epoch;atomic_store(&sourceEpochCurrent,epoch);
  NSNotificationCenter *center=NSWorkspace.sharedWorkspace.notificationCenter;
  NSMutableArray *tokens=[NSMutableArray array];
  for(NSString *name in @[NSWorkspaceDidLaunchApplicationNotification,NSWorkspaceDidTerminateApplicationNotification]) {
   id token=[center addObserverForName:name object:nil queue:nil usingBlock:^(NSNotification *note){
    if(atomic_load(&sourceEpochCurrent)!=epoch)return;
    NSRunningApplication *app=note.userInfo[NSWorkspaceApplicationKey];
    BOOL dockChanged=[app.bundleIdentifier isEqualToString:@"com.apple.dock"];pid_t pid=app.processIdentifier;
    pthread_mutex_lock(&tokenMutex);
    if(dockChanged||pid==currentAppPID){currentGeneration=0;currentGesture=0;}
    pthread_mutex_unlock(&tokenMutex);
   }];[tokens addObject:token];
  }
  s->workspaceTokens=CFBridgingRetain(tokens);
 }
 atomic_fetch_add(&installations,1);return s;
}
void ot_dock_input_target(void *owner,OTDockInputTarget target) {
 InputOwner *s=owner;
 applyCompletion(s);
 if(s->gesture&&(!target.generation||target.generation!=s->owned.generation||target.dock_pid!=s->owned.dock_pid))cancelOwned(s,NULL);
 else if(s->down&&(!sameTarget(s->owned,target)||target.expiry<ot_dock_input_now()))cancelOwned(s,NULL);
 s->target=target;
}
void ot_dock_input_pump(void *owner) {
 InputOwner *s=owner;
 CFRunLoopRunInMode(kCFRunLoopDefaultMode,0.005,true);
 if(ot_dock_input_health(owner)!=0)return;
 if(s->gesture&&!ot_dock_input_owned(s->owned.generation,s->gesture)&&!ot_dock_input_completed(s->owned.generation,s->gesture))cancelOwned(s,NULL);

 applyCompletion(s);
 if(s->down&&s->target.expiry<ot_dock_input_now())cancelOwned(s,NULL);
 if(s->scrolling&&ot_dock_input_now()-s->scrollAt>0.25) {
  if(s->completed)cancelOwned(s,NULL);
  else {
   // A phase-less subthreshold stream still needs an observable terminal so
   // the owner can acknowledge no action. Do not revoke a pending action.
   if(!s->scrollTerminal) {
    if(enqueue(s,NULL,kCGEventScrollWheel,NO))s->queue[(s->head+s->count-1)%INPUT_QUEUE].terminal=1;
    else {cancelOwned(s,NULL);return;}
   }
   s->scrolling=0;s->scrollTerminal=1;
  }
 }
}
int ot_dock_input_pop(void *owner,OTDockInputRaw *out) {
 InputOwner *s=owner;if(!s->count)return 0;
 *out=s->queue[s->head];s->head=(s->head+1)%INPUT_QUEUE;s->count--;return 1;
}
void ot_dock_input_ack(void *owner,uint64_t generation,uint64_t gesture,int valid) {
 InputOwner *s=owner;
 if(s->gesture!=gesture||s->owned.generation!=generation)return;
 if(valid){if(s->test)captureProcessIdentity(generation,gesture,getpid(),getpid());s->validated=1;}else cancelOwned(s,NULL);
}
void ot_dock_input_stop(void *owner) {
 InputOwner *s=owner;if(!s)return;
 cancelOwned(s,NULL);
 if(!s->test)atomic_store(&sourceEpochCurrent,0);
 if(s->workspaceTokens) {
  @autoreleasepool {NSArray *tokens=CFBridgingRelease(s->workspaceTokens);for(id token in tokens)[NSWorkspace.sharedWorkspace.notificationCenter removeObserver:token];}
  s->workspaceTokens=NULL;
 }
 if(s->source){CFRunLoopRemoveSource(CFRunLoopGetCurrent(),s->source,kCFRunLoopCommonModes);CFRelease(s->source);}
 if(s->tap){CGEventTapEnable(s->tap,false);CFMachPortInvalidate(s->tap);CFRelease(s->tap);}
 clearQueue(s);if(!s->test)atomic_store(&sourceRunning,false);else atomic_fetch_sub(&testAlive,1);free(s);
}
void ot_dock_input_release(void *event){if(event)CFRelease(event);}

static int phaseCode(NSEventPhase phase) {
 if(phase&NSEventPhaseCancelled)return 4;if(phase&NSEventPhaseEnded)return 3;
 if(phase&NSEventPhaseBegan)return 1;if(phase&NSEventPhaseChanged||phase&NSEventPhaseStationary)return 2;return 0;
}
OTDockInputFields ot_dock_input_fields(void *event) {
 OTDockInputFields out={0};if(!event)return out;
 @autoreleasepool {
  CGEventRef cg=event;CGPoint point=CGEventGetLocation(cg);out.x=point.x;out.y=point.y;
  CGEventFlags flags=inputMods(cg);
  if(flags&kCGEventFlagMaskControl)out.mods|=1;if(flags&kCGEventFlagMaskAlternate)out.mods|=2;
  if(flags&kCGEventFlagMaskShift)out.mods|=4;if(flags&kCGEventFlagMaskCommand)out.mods|=8;if(flags&kCGEventFlagMaskSecondaryFn)out.mods|=128;
  out.button=(int)CGEventGetIntegerValueField(cg,kCGMouseEventButtonNumber);
  if(CGEventGetType(cg)==kCGEventScrollWheel) {
   NSEvent *e=[NSEvent eventWithCGEvent:cg];
   out.precise=e.hasPreciseScrollingDeltas;out.inverted=e.isDirectionInvertedFromDevice;
   double direction=out.inverted?-1:1;
   out.dx=e.scrollingDeltaX*direction;out.dy=e.scrollingDeltaY*direction;
   out.phase=phaseCode(e.phase);out.momentum=phaseCode(e.momentumPhase);
  }
 }
 return out;
}

static CFTypeRef inputAttribute(AXUIElementRef element,CFStringRef name,CFAbsoluteTime deadline) {
 double remaining=deadline-CFAbsoluteTimeGetCurrent();if(remaining<=0)return NULL;
 AXUIElementSetMessagingTimeout(element,MIN(0.03,remaining));CFTypeRef value=NULL;
 if(AXUIElementCopyAttributeValue(element,name,&value)!=kAXErrorSuccess){if(value)CFRelease(value);return NULL;}return value;
}
static BOOL inputStringEquals(AXUIElementRef node,CFStringRef key,CFStringRef expected,CFAbsoluteTime deadline) {
 CFTypeRef value=inputAttribute(node,key,deadline);BOOL result=value&&CFGetTypeID(value)==CFStringGetTypeID()&&CFEqual(value,expected);if(value)CFRelease(value);return result;
}
int ot_dock_input_validate_app(OTDockInputRaw raw) {
 @autoreleasepool {
  if(!AXIsProcessTrusted())return 0;
  OTDockInputTarget t=raw.target;
  // Capture before reading app metadata. A PID reused during those reads must
  // still fail the final process-start guard rather than becoming the new token.
  uint64_t appS=0,appU=0,dockS=0,dockU=0;
  if(!processStart(t.app_pid,&appS,&appU)||!processStart(t.dock_pid,&dockS,&dockU))return 0;
  NSRunningApplication *dock=[NSRunningApplication runningApplicationWithProcessIdentifier:t.dock_pid];
  NSRunningApplication *app=[NSRunningApplication runningApplicationWithProcessIdentifier:t.app_pid];
  NSString *path=[NSString stringWithUTF8String:t.path],*bundle=[NSString stringWithUTF8String:t.bundle];
  if(!dock||dock.terminated||![dock.bundleIdentifier isEqualToString:@"com.apple.dock"]||!app||app.terminated||app.activationPolicy!=NSApplicationActivationPolicyRegular||![app.bundleIdentifier isEqualToString:bundle]||![app.bundleURL.URLByStandardizingPath.URLByResolvingSymlinksInPath.path isEqualToString:path])return 0;
  NSUInteger matches=0;
  for(NSRunningApplication *candidate in [NSRunningApplication runningApplicationsWithBundleIdentifier:bundle]) {
   if(!candidate.terminated&&candidate.activationPolicy==NSApplicationActivationPolicyRegular&&[candidate.bundleURL.URLByStandardizingPath.URLByResolvingSymlinksInPath.path isEqualToString:path])matches++;
  }
  if(matches!=1)return 0;
  return recordProcessIdentity(raw.target.generation,raw.gesture,t.app_pid,t.dock_pid,appS,appU,dockS,dockU);
 }
}
int ot_dock_input_validate(OTDockInputRaw raw) {
 @autoreleasepool {
  if(!ot_dock_input_validate_app(raw))return 0;
  OTDockInputTarget t=raw.target;
  NSString *path=[NSString stringWithUTF8String:t.path];
  CGPoint point=CGEventGetLocation(raw.event);
  CFAbsoluteTime deadline=CFAbsoluteTimeGetCurrent()+0.15;
  AXUIElementRef system=AXUIElementCreateSystemWide(),node=NULL;AXUIElementSetMessagingTimeout(system,0.03);
  AXError hit=AXUIElementCopyElementAtPosition(system,point.x,point.y,&node);CFRelease(system);
  if(hit!=kAXErrorSuccess||!node){if(node)CFRelease(node);return 0;}
  BOOL valid=NO;
  for(int i=0;i<6&&node&&CFAbsoluteTimeGetCurrent()<deadline;i++) {
   pid_t pid=0;if(AXUIElementGetPid(node,&pid)!=kAXErrorSuccess||pid!=t.dock_pid)break;
   if(inputStringEquals(node,kAXRoleAttribute,kAXDockItemRole,deadline)) {
    if(!inputStringEquals(node,kAXSubroleAttribute,kAXApplicationDockItemSubrole,deadline))break;
    CFTypeRef url=inputAttribute(node,kAXURLAttribute,deadline);
    BOOL identity=url&&CFGetTypeID(url)==CFURLGetTypeID()&&[(__bridge NSURL *)url isFileURL]&&[[(__bridge NSURL *)url URLByStandardizingPath].URLByResolvingSymlinksInPath.path isEqualToString:path];
    if(url)CFRelease(url);
    CFTypeRef pos=inputAttribute(node,kAXPositionAttribute,deadline),size=inputAttribute(node,kAXSizeAttribute,deadline);
    CGPoint p=CGPointZero;CGSize z=CGSizeZero;
    BOOL bounds=pos&&size&&CFGetTypeID(pos)==AXValueGetTypeID()&&CFGetTypeID(size)==AXValueGetTypeID()&&AXValueGetType(pos)==kAXValueCGPointType&&AXValueGetType(size)==kAXValueCGSizeType&&AXValueGetValue(pos,kAXValueCGPointType,&p)&&AXValueGetValue(size,kAXValueCGSizeType,&z);
    if(pos)CFRelease(pos);if(size)CFRelease(size);
    valid=identity&&bounds&&z.width>0&&z.height>0&&CGRectContainsPoint(CGRectMake(p.x,p.y,z.width,z.height),point)&&CGRectContainsPoint(CGRectMake(t.x,t.y,t.w,t.h),point)&&CFAbsoluteTimeGetCurrent()<deadline;
    break;
   }
   CFTypeRef parent=inputAttribute(node,kAXParentAttribute,deadline);CFRelease(node);node=NULL;
   if(parent&&CFGetTypeID(parent)==AXUIElementGetTypeID())node=(AXUIElementRef)parent;else if(parent)CFRelease(parent);
  }
  if(node)CFRelease(node);return valid;
 }
}

int ot_dock_input_test_event(void *owner,int type,double x,double y,int mods,int isSynthetic,int phase,int momentum) {
 InputOwner *s=owner;if(!s||!s->test)return -1;
 if((uint32_t)type==kCGEventTapDisabledByTimeout||(uint32_t)type==kCGEventTapDisabledByUserInput){inputCallback(NULL,(CGEventType)type,NULL,s);return 0;}
 CGEventRef event=type==kCGEventScrollWheel?CGEventCreateScrollWheelEvent(NULL,kCGScrollEventUnitPixel,2,20,0):CGEventCreateMouseEvent(NULL,type,CGPointMake(x,y),type==kCGEventRightMouseDown||type==kCGEventRightMouseUp?kCGMouseButtonRight:kCGMouseButtonLeft);
 CGEventSetLocation(event,CGPointMake(x,y));CGEventSetIntegerValueField(event,kCGEventSourceUnixProcessID,isSynthetic?getpid():0);CGEventSetIntegerValueField(event,kCGEventSourceStateID,kCGEventSourceStateHIDSystemState);
 CGEventFlags f=0;if(mods&1)f|=kCGEventFlagMaskControl;if(mods&2)f|=kCGEventFlagMaskAlternate;if(mods&4)f|=kCGEventFlagMaskShift;if(mods&8)f|=kCGEventFlagMaskCommand;CGEventSetFlags(event,f);
 if(type==kCGEventScrollWheel){CGEventSetIntegerValueField(event,kCGScrollWheelEventScrollPhase,phase);CGEventSetIntegerValueField(event,kCGScrollWheelEventMomentumPhase,momentum);}
 CGEventRef result=inputCallback(NULL,type,event,s);CFRelease(event);return result==NULL;
}
int ot_dock_input_test_replays(void *owner){return ((InputOwner *)owner)->replays;}

int ot_dock_input_test_alive(void){return atomic_load(&testAlive);}
uint64_t ot_dock_input_synthetic_passes(void){return atomic_load(&syntheticPasses);}
uint64_t ot_dock_input_installations(void){return atomic_load(&installations);}

void ot_dock_input_complete(uint64_t generation,uint64_t gesture) {
 pthread_mutex_lock(&tokenMutex);
 if(generation==currentGeneration&&gesture==currentGesture)currentCompleted=YES;
 pthread_mutex_unlock(&tokenMutex);
}
int ot_dock_input_completed(uint64_t generation,uint64_t gesture) {
 pthread_mutex_lock(&tokenMutex);int result=generation==currentGeneration&&gesture==currentGesture&&currentCompleted;
 pthread_mutex_unlock(&tokenMutex);return result;
}

void ot_dock_input_test_idle(void *owner) {
 InputOwner *s=owner;if(!s||!s->test)return;s->scrollAt=ot_dock_input_now()-0.251;ot_dock_input_pump(owner);
}

void ot_dock_input_test_changed_process(void *owner) {
 InputOwner *s=owner;if(!s||!s->test)return;
 pthread_mutex_lock(&tokenMutex);currentAppMicros++;pthread_mutex_unlock(&tokenMutex);
}

int ot_dock_input_health(void *owner){
 InputOwner *s=owner;if(s->health)return s->health;
 double now=ot_dock_input_now();
 if(s->test||now>=s->nextPermissionCheck){
  s->nextPermissionCheck=now+0.1;
  BOOL ax=s->test?(s->testHealthMask&1)!=0:AXIsProcessTrusted();
  BOOL listen=s->test?(s->testHealthMask&2)!=0:CGPreflightListenEventAccess();
  if(!ax)s->health=1;else if(!listen)s->health=2;
 }
 BOOL valid=s->test?(s->testHealthMask&4)!=0:s->tap&&CFMachPortIsValid(s->tap);
 BOOL enabled=s->test?(s->testHealthMask&8)!=0:valid&&CGEventTapIsEnabled(s->tap);
 if(!s->health&&!valid)s->health=3;
 if(!s->health&&!enabled)s->health=4;
 if(s->health){s->permissionAllowed=0;cancelOwned(s,NULL);s->target.expiry=0;if(s->tap)CGEventTapEnable(s->tap,false);}
 return s->health;
}
void ot_dock_input_test_health(void *owner,int mask){InputOwner *s=owner;if(s->test)s->testHealthMask=mask;}
