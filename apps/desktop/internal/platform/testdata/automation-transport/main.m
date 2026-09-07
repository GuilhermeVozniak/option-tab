#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <assert.h>
#include <unistd.h>

@interface OTTestManager : NSObject
@property NSUInteger suspends,resumes;
@property NSAppleEventDescriptor *current;
@property NSMutableDictionary *replies;
@property NSMutableDictionary *handlers;
- (NSAppleEventManagerSuspensionID)suspendCurrentAppleEvent;
- (NSAppleEventDescriptor *)replyAppleEventForSuspensionID:(NSAppleEventManagerSuspensionID)sid;
- (void)resumeWithSuspensionID:(NSAppleEventManagerSuspensionID)sid;
- (void)setEventHandler:(id)handler andSelector:(SEL)selector forEventClass:(AEEventClass)cls andEventID:(AEEventID)event;
- (void)removeEventHandlerForEventClass:(AEEventClass)cls andEventID:(AEEventID)event;
@end
@implementation OTTestManager
- (instancetype)init {if((self=[super init])){_replies=[NSMutableDictionary dictionary];_handlers=[NSMutableDictionary dictionary];}return self;}
- (NSAppleEventManagerSuspensionID)suspendCurrentAppleEvent {if(!self.current)return NULL;NSUInteger token=++self.suspends;self.replies[@(token)]=self.current;self.current=nil;return (void *)token;}
- (NSAppleEventDescriptor *)replyAppleEventForSuspensionID:(NSAppleEventManagerSuspensionID)sid {NSAppleEventDescriptor *reply=self.replies[@((uintptr_t)sid)];assert(reply);return reply;}
- (void)resumeWithSuspensionID:(NSAppleEventManagerSuspensionID)sid {assert(self.replies[@((uintptr_t)sid)]);[self.replies removeObjectForKey:@((uintptr_t)sid)];self.resumes++;}
- (void)setEventHandler:(id)handler andSelector:(SEL)selector forEventClass:(AEEventClass)cls andEventID:(AEEventID)event {self.handlers[@(event)]=handler;}
- (void)removeEventHandlerForEventClass:(AEEventClass)cls andEventID:(AEEventID)event {[self.handlers removeObjectForKey:@(event)];}
@end
static OTTestManager *testManager;
static BOOL testHandlerExists(AEEventID event){return testManager.handlers[@(event)]!=nil;}
static OSErr testUPP(const AppleEvent *event,AppleEvent *reply,SRefCon refcon){return noErr;}
static OSErr testGetHandler(AEEventClass cls,AEEventID event,AEEventHandlerUPP *handler,SRefCon *refcon,Boolean system){
 id current=testManager.handlers[@(event)];if(!current)return errAEHandlerNotFound;
 *handler=testUPP;*refcon=(__bridge void *)current;return noErr;
}
#define OT_AUTOMATION_GET_HANDLER testGetHandler
#define OT_AUTOMATION_MANAGER testManager
#define OT_AUTOMATION_HANDLER_EXISTS testHandlerExists
#include "../../darwin_automation.m"
static void pump(double seconds){NSDate *until=[NSDate dateWithTimeIntervalSinceNow:seconds];do{[[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.001]];}while(until.timeIntervalSinceNow>0);}
@interface OTTestEvent : NSAppleEventDescriptor
@property SInt16 fixtureSource;
@end
@implementation OTTestEvent
- (NSAppleEventDescriptor *)attributeDescriptorForKeyword:(AEKeyword)keyword {
 if(keyword==keyEventSourceAttr){SInt16 source=self.fixtureSource?:kAESameProcess;return [NSAppleEventDescriptor descriptorWithDescriptorType:typeSInt16 bytes:&source length:2];}
 if(keyword==keySenderPIDAttr)return [NSAppleEventDescriptor descriptorWithInt32:getpid()];
 if(keyword==keySenderEUIDAttr)return [NSAppleEventDescriptor descriptorWithInt32:geteuid()];
 return [super attributeDescriptorForKeyword:keyword];
}
@end
static NSAppleEventDescriptor *event(AEEventID code){
 NSAppleEventDescriptor *e=[NSAppleEventDescriptor appleEventWithEventClass:'OpTb' eventID:code targetDescriptor:[NSAppleEventDescriptor nullDescriptor] returnID:kAutoGenerateReturnID transactionID:kAnyTransactionID];
 SInt16 source=kAESameProcess;[e setAttributeDescriptor:[NSAppleEventDescriptor descriptorWithDescriptorType:typeSInt16 bytes:&source length:sizeof(source)] forKeyword:keyEventSourceAttr];
 [e setAttributeDescriptor:[NSAppleEventDescriptor descriptorWithInt32:getpid()] forKeyword:keySenderPIDAttr];
 [e setAttributeDescriptor:[NSAppleEventDescriptor descriptorWithInt32:geteuid()] forKeyword:keySenderEUIDAttr];AEDesc copy;assert(AEDuplicateDesc(e.aeDesc,&copy)==noErr);return [[OTTestEvent alloc] initWithAEDescNoCopy:&copy];
}
static NSAppleEventDescriptor *submit(OTAutomationOwner *owner,NSAppleEventDescriptor *e){NSAppleEventDescriptor *reply=[NSAppleEventDescriptor recordDescriptor];testManager.current=reply;[owner handle:e reply:reply];return reply;}
int main(void){@autoreleasepool{
 testManager=[OTTestManager new];
 // Dictionary wire types are independently exercised; unknown/ambiguous selectors refuse.
 NSAppleEventDescriptor *e=event('pvsh');[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:@"A Fixture"] forKeyword:'anam'];
 [e setParamDescriptor:[NSAppleEventDescriptor descriptorWithDouble:-120.5] forKeyword:'xpos'];[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithInt32:20] forKeyword:'ypos'];
 NSDictionary *decoded=OTAutomationDecode(e);assert([decoded[@"Name"] isEqual:@"A Fixture"]);assert([decoded[@"Position"][@"X"] doubleValue]==-120.5);
 [e setParamDescriptor:[NSAppleEventDescriptor descriptorWithInt32:123] forKeyword:'apid'];assert(!OTAutomationDecode(e));
 e=event('qapp');[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:@"no"] forKeyword:'xxxx'];assert(!OTAutomationDecode(e));
 e=event('wact');[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithEnumCode:'full'] forKeyword:'wopn'];[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:@"4294967295"] forKeyword:'widn'];assert(!OTAutomationDecode(e));[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithBoolean:NO] forKeyword:'full'];assert(OTAutomationDecode(e));[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:@"4294967296"] forKeyword:'widn'];assert(!OTAutomationDecode(e));
 e=event('qact');[e setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:@"true"] forKeyword:'imag'];assert(!OTAutomationDecode(e));
 e=event('qapp');SInt16 remote=kAERemoteProcess;((OTTestEvent *)e).fixtureSource=remote;assert(!OTAutomationLocal(e));
 // Existing private handler is left untouched on refused ownership.
 NSObject *prior=[NSObject new];testManager.handlers[@((uint32_t)'qapp')]=prior;uint64_t refused=ot_automation_start();pump(.01);assert(ot_automation_status(refused)==-2);assert(testManager.handlers[@((uint32_t)'qapp')]==prior);ot_automation_stop(refused);pump(.01);assert(testManager.handlers.count==1);[testManager.handlers removeAllObjects];
 uint64_t server=ot_automation_start();pump(.01);assert(ot_automation_status(server)==1);assert(testManager.handlers.count==7);OTAutomationOwner *owner=OTAutomationOwnerFor(server);
 NSAppleEventDescriptor *reply=submit(owner,event('qapp'));assert(testManager.suspends==1);assert(ot_automation_current(server,1));char *wire=ot_automation_pop(server);assert(wire&&strstr(wire,"queryApps"));free(wire);
 ot_automation_complete(server,1,"{\"ok\":true}","","");pump(.01);assert(testManager.resumes==1);assert([[reply paramDescriptorForKeyword:keyDirectObject].stringValue isEqual:@"{\"ok\":true}"]);ot_automation_complete(server,1,"{}","","");pump(.01);assert(testManager.resumes==1);
 // A busy queue must reply immediately even while another thread holds its mutex.
 dispatch_semaphore_t held=dispatch_semaphore_create(0),released=dispatch_semaphore_create(0);
 dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{pthread_mutex_lock(&owner->queueMutex);dispatch_semaphore_signal(held);usleep(100000);pthread_mutex_unlock(&owner->queueMutex);dispatch_semaphore_signal(released);});
 dispatch_semaphore_wait(held,DISPATCH_TIME_FOREVER);uint64_t beforeBusy=OTAutomationNow();reply=submit(owner,event('qapp'));assert(OTAutomationNow()-beforeBusy<20000000);assert([reply paramDescriptorForKeyword:keyErrorNumber].int32Value!=0);assert(testManager.resumes==2);dispatch_semaphore_wait(released,DISPATCH_TIME_FOREVER);
 // Native timeout invalidates admission and resumes even without a Go consumer.
 e=event('qapp');[e setAttributeDescriptor:[NSAppleEventDescriptor descriptorWithInt32:1] forKeyword:keyTimeoutAttr];reply=submit(owner,e);uint64_t timeoutRequest=owner.nextRequest;pump(.03);assert(!ot_automation_current(server,timeoutRequest));assert([reply paramDescriptorForKeyword:keyErrorNumber].int32Value==errAETimeout);assert(testManager.resumes==3);assert(!ot_automation_pop(server));
 // A no-reply sender still gets lifecycle retirement without reply mutation.
 NSAppleEventDescriptor *noReply=[NSAppleEventDescriptor nullDescriptor];testManager.current=noReply;[owner handle:event('qapp') reply:noReply];[owner finish:owner.nextRequest json:@"{}" code:nil message:nil];assert(noReply.descriptorType==typeNull);assert(testManager.resumes==4);
 // 17th request never suspends; shutdown resumes all sixteen exactly once.
 for(int i=0;i<16;i++)submit(owner,event('qapp'));NSUInteger count=testManager.suspends;reply=submit(owner,event('qapp'));assert(testManager.suspends==count);assert([reply paramDescriptorForKeyword:keyErrorNumber].int32Value!=0);
 NSObject *replacement=[NSObject new];testManager.handlers[@((uint32_t)'qapp')]=replacement;
 ot_automation_drain_main(server);assert(testManager.resumes==testManager.suspends);assert(testManager.handlers.count==1&&testManager.handlers[@((uint32_t)'qapp')]==replacement);[testManager.handlers removeAllObjects];assert(owner.pending.count==0);assert(!ot_automation_current(server,3));ot_automation_drain_main(server);assert(testManager.resumes==testManager.suspends);ot_automation_stop(server);pump(.01);
 uint64_t stoppedBeforeInstall=ot_automation_start();ot_automation_stop(stoppedBeforeInstall);pump(.01);assert(ot_automation_status(stoppedBeforeInstall)<0);assert(testManager.handlers.count==0);
 printf("PASS typed descriptors, local provenance, collision preservation, reply-once, native timeout, 16-slot overflow, shutdown drain; suspended=%lu resumed=%lu\n",(unsigned long)testManager.suspends,(unsigned long)testManager.resumes);
}}
