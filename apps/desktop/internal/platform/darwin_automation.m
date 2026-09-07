//go:build darwin

#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <pthread.h>
#include <stdatomic.h>
#include <time.h>
#include <unistd.h>
#include "darwin_automation.h"

#ifndef OT_AUTOMATION_MANAGER
#define OT_AUTOMATION_MANAGER [NSAppleEventManager sharedAppleEventManager]
#endif
#ifndef OT_AUTOMATION_GET_HANDLER
#define OT_AUTOMATION_GET_HANDLER AEGetEventHandler
#endif
#ifndef OT_AUTOMATION_HANDLER_EXISTS
#define OT_AUTOMATION_HANDLER_EXISTS OTAutomationHandlerExists
#endif

static uint64_t OTAutomationNow(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + ts.tv_nsec;
}
static const AEEventID OTAutomationEvents[] = {'swop','pvsh','pvhi','wact','qapp','qwin','qact'};
static BOOL OTAutomationHandlerExists(AEEventID event) {
    AEEventHandlerUPP handler = NULL;
    SRefCon refcon = 0;
    return AEGetEventHandler('OpTb', event, &handler, &refcon, false) == noErr;
}
@interface OTAutomationRecord : NSObject
@property NSAppleEventManagerSuspensionID suspension;
@property uint64_t deadline;
@property NSDictionary *wire;
@property BOOL queued;
@end
@implementation OTAutomationRecord
@end

@interface OTAutomationOwner : NSObject {
@public
    atomic_bool stopped;
    atomic_int status;
    pthread_mutex_t queueMutex;
    AEEventHandlerUPP installedHandlers[7];
    SRefCon installedRefcons[7];
    BOOL installedLeases[7];
}
@property uint64_t token;
@property uint64_t nextRequest;
@property BOOL installed;
@property NSMutableDictionary<NSNumber *, OTAutomationRecord *> *pending; // main only
@property NSMutableDictionary<NSNumber *, NSNumber *> *live; // queueMutex only
@property NSMutableArray<NSDictionary *> *queue; // queueMutex only
- (void)handle:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply;
- (void)finish:(uint64_t)request json:(NSString *)json code:(NSString *)code message:(NSString *)message;
- (void)drain;
@end
static pthread_mutex_t OTAutomationOwnersMutex = PTHREAD_MUTEX_INITIALIZER;
static NSMutableDictionary<NSNumber *, OTAutomationOwner *> *OTAutomationOwners;
static uint64_t OTAutomationNextOwner;
static OTAutomationOwner *OTAutomationOwnerFor(uint64_t token) {
    pthread_mutex_lock(&OTAutomationOwnersMutex);
    OTAutomationOwner *owner = OTAutomationOwners[@(token)];
    pthread_mutex_unlock(&OTAutomationOwnersMutex);
    return owner;
}
static void OTAutomationError(NSAppleEventDescriptor *reply, NSString *code, NSString *message) {
    if (!reply || reply.descriptorType == typeNull) return;
    int number = [code isEqualToString:@"invalidArgument"] ? errAEWrongDataType :
        [code isEqualToString:@"timeout"] ? errAETimeout :
        [code isEqualToString:@"permissionDenied"] ? errAEEventNotPermitted :
        [code isEqualToString:@"notFound"] ? errAENoSuchObject : -2700;
    NSString *text = [NSString stringWithFormat:@"%@: %@", code, message];
    [reply setParamDescriptor:[NSAppleEventDescriptor descriptorWithInt32:number] forKeyword:keyErrorNumber];
    [reply setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:text] forKeyword:keyErrorString];
}
static NSString *OTAutomationText(NSAppleEventDescriptor *value, NSUInteger limit) {
    if (!value || (value.descriptorType != typeUnicodeText && value.descriptorType != typeUTF8Text && value.descriptorType != typeChar)) return nil;
    NSString *text = value.stringValue;
    if (!text.length || [text lengthOfBytesUsingEncoding:NSUTF8StringEncoding] > limit || [text rangeOfString:[NSString stringWithCharacters:(unichar[]){0} length:1]].location != NSNotFound) return nil;
    return text;
}
static BOOL OTAutomationDecimal(NSString *text, uint64_t maximum) {
    if (!text.length || text.length > 20) return NO;
    uint64_t value = 0;
    for (NSUInteger i=0; i<text.length; i++) {
        unichar c = [text characterAtIndex:i];
        if (c<'0' || c>'9' || value > (maximum-(c-'0'))/10) return NO;
        value=value*10+c-'0';
    }
    return value != 0;
}
static NSNumber *OTAutomationBool(NSAppleEventDescriptor *value) {
    if (value.descriptorType == typeTrue) return @YES;
    if (value.descriptorType == typeFalse) return @NO;
    return value.descriptorType == typeBoolean ? @(value.booleanValue) : nil;
}
static NSNumber *OTAutomationNumber(NSAppleEventDescriptor *value) {
    double number;
    if (value.descriptorType == typeIEEE64BitFloatingPoint) number=value.doubleValue;
    else if (value.descriptorType == typeSInt32) number=value.int32Value;
    else return nil;
    return isfinite(number) && fabs(number)<=10000000 ? @(number) : nil;
}
static NSString *OTAutomationEnum(NSAppleEventDescriptor *value, NSDictionary *names) {
    return value.descriptorType == typeEnumerated ? names[@(value.enumCodeValue)] : nil;
}
static NSDictionary *OTAutomationDecode(NSAppleEventDescriptor *event) {
    if (AEGetDescDataSize(event.aeDesc)>16384) return nil;
    AEEventID code=event.eventID;
    NSDictionary *operations=@{@((uint32_t)'swop'):@"openSwitcher",@((uint32_t)'pvsh'):@"showPreviews",@((uint32_t)'pvhi'):@"hidePreviews",@((uint32_t)'wact'):@"windowAction",@((uint32_t)'qapp'):@"queryApps",@((uint32_t)'qwin'):@"queryWindows",@((uint32_t)'qact'):@"queryActiveWindow"};
    NSString *operation=operations[@(code)];if(!operation)return nil;
    NSMutableSet *allowed=[NSMutableSet set];
    if(code=='swop')[allowed addObject:@((uint32_t)'mode')];
    if(code=='pvsh'||code=='qwin'||code=='wact')[allowed addObjectsFromArray:@[@((uint32_t)'anam'),@((uint32_t)'bund'),@((uint32_t)'apid')]];
    if(code=='pvsh')[allowed addObjectsFromArray:@[@((uint32_t)'xpos'),@((uint32_t)'ypos')]];
    if(code=='pvhi')[allowed addObject:@((uint32_t)'ptok')];
    if(code=='wact')[allowed addObjectsFromArray:@[@((uint32_t)'widn'),@((uint32_t)'actv'),@((uint32_t)'wopn'),@((uint32_t)'full')]];
    if(code=='qwin'||code=='qact')[allowed addObject:@((uint32_t)'imag')];
    for(NSInteger i=1;i<=event.numberOfItems;i++)if(![allowed containsObject:@([event keywordForDescriptorAtIndex:i])])return nil;
    NSMutableDictionary *wire=[@{@"Operation":operation} mutableCopy];
    NSDictionary *texts=@{@((uint32_t)'anam'):@"Name",@((uint32_t)'bund'):@"BundleID",@((uint32_t)'widn'):@"WindowID",@((uint32_t)'ptok'):@"PresentationToken"};
    for(NSNumber *key in texts){NSAppleEventDescriptor *v=[event paramDescriptorForKeyword:key.unsignedIntValue];if(v){NSString *text=OTAutomationText(v,1024);if(!text)return nil;wire[texts[key]]=text;}}
    NSAppleEventDescriptor *pid=[event paramDescriptorForKeyword:'apid'];
    if(pid){if(pid.descriptorType!=typeSInt32||pid.int32Value<=0)return nil;wire[@"PID"]=@(pid.int32Value);}
    NSUInteger selectors=(wire[@"Name"]!=nil)+(wire[@"BundleID"]!=nil)+(wire[@"PID"]!=nil);
    if(selectors>1||(code=='pvsh'&&selectors!=1))return nil;
    NSDictionary *booleans=@{@((uint32_t)'actv'):@"ActiveWindow",@((uint32_t)'full'):@"Fullscreen",@((uint32_t)'imag'):@"IncludeImages"};
    for(NSNumber *key in booleans){NSAppleEventDescriptor *v=[event paramDescriptorForKeyword:key.unsignedIntValue];if(v){NSNumber *b=OTAutomationBool(v);if(!b)return nil;wire[booleans[key]]=b;}}
    NSAppleEventDescriptor *mode=[event paramDescriptorForKeyword:'mode'];
    if(mode){NSString *name=OTAutomationEnum(mode,@{@((uint32_t)'wind'):@"windows",@((uint32_t)'apps'):@"apps"});if(!name)return nil;wire[@"Mode"]=name;}
    NSAppleEventDescriptor *x=[event paramDescriptorForKeyword:'xpos'],*y=[event paramDescriptorForKeyword:'ypos'];
    if(x||y){NSNumber *nx=OTAutomationNumber(x),*ny=OTAutomationNumber(y);if(!nx||!ny)return nil;wire[@"Position"]=@{@"X":nx,@"Y":ny};}
    if(code=='pvhi'&&!OTAutomationDecimal(wire[@"PresentationToken"],UINT64_MAX))return nil;
    if(code=='wact'){
        NSString *action=OTAutomationEnum([event paramDescriptorForKeyword:'wopn'],@{@((uint32_t)'focu'):@"focus",@((uint32_t)'clos'):@"close",@((uint32_t)'mini'):@"minimize",@((uint32_t)'hide'):@"hide",@((uint32_t)'full'):@"fullscreen"});
        if(!action)return nil;wire[@"Action"]=action;
        BOOL active=[wire[@"ActiveWindow"] boolValue],window=wire[@"WindowID"]!=nil;
        if(active==window || (window&&!OTAutomationDecimal(wire[@"WindowID"],UINT32_MAX)))return nil;
        if([action isEqualToString:@"fullscreen"] != (wire[@"Fullscreen"]!=nil))return nil;
    }
    return wire;
}
static BOOL OTAutomationLocal(NSAppleEventDescriptor *event) {
    NSAppleEventDescriptor *source=[event attributeDescriptorForKeyword:keyEventSourceAttr];
    if(source.descriptorType!=typeSInt16)return NO;
    int s=source.int32Value;
    NSAppleEventDescriptor *pid=[event attributeDescriptorForKeyword:keySenderPIDAttr],*uid=[event attributeDescriptorForKeyword:keySenderEUIDAttr];
    return (s==kAELocalProcess||s==kAESameProcess)&&pid.descriptorType==typeSInt32&&pid.int32Value>0&&uid.descriptorType==typeSInt32&&(uid_t)uid.int32Value==geteuid();
}
@implementation OTAutomationOwner
- (instancetype)init {
    if((self=[super init])){atomic_init(&stopped,false);atomic_init(&status,0);pthread_mutex_init(&queueMutex,NULL);_pending=[NSMutableDictionary dictionary];_live=[NSMutableDictionary dictionary];_queue=[NSMutableArray array];}
    return self;
}
- (void)dealloc {pthread_mutex_destroy(&queueMutex);}
- (void)handle:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply {
    uint64_t received=OTAutomationNow();
    if(atomic_load(&stopped)){OTAutomationError(reply,@"cancelled",@"Automation stopped");return;}
    if(!OTAutomationLocal(event)){OTAutomationError(reply,@"permissionDenied",@"Only local current-user Apple events are accepted");return;}
    NSDictionary *wire=OTAutomationDecode(event);
    if(!wire){OTAutomationError(reply,@"invalidArgument",@"Invalid typed automation parameters");return;}
    if(self.pending.count>=16){OTAutomationError(reply,@"busy",@"Automation request capacity reached");return;}
    uint64_t budget=5000000000ULL;
    NSAppleEventDescriptor *timeout=[event attributeDescriptorForKeyword:keyTimeoutAttr];
    if(timeout.descriptorType==typeSInt32&&timeout.int32Value>0){uint64_t requested=(uint64_t)timeout.int32Value*1000000000ULL/60;budget=MIN(budget,requested>10000000?requested-10000000:1);}
    NSAppleEventManagerSuspensionID sid=[OT_AUTOMATION_MANAGER suspendCurrentAppleEvent];
    if(!sid){OTAutomationError(reply,@"internal",@"Could not suspend Apple event");return;}
    uint64_t request=++self.nextRequest;
    OTAutomationRecord *record=[OTAutomationRecord new];record.suspension=sid;record.deadline=received+budget;record.wire=wire;
    self.pending[@(request)]=record;
    if(pthread_mutex_trylock(&queueMutex)!=0){[self finish:request json:nil code:@"busy" message:@"Automation queue is occupied"];return;}
    record.queued=YES;
    self.live[@(request)]=@(record.deadline);
    [self.queue addObject:@{@"ID":@(request),@"Deadline":@(record.deadline),@"Wire":wire}];
    pthread_mutex_unlock(&queueMutex);
    __weak OTAutomationOwner *weakSelf=self;
    uint64_t timerNow=OTAutomationNow();
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW,(int64_t)(record.deadline>timerNow?record.deadline-timerNow:0)),dispatch_get_main_queue(),^{[weakSelf finish:request json:nil code:@"timeout" message:@"Automation request expired"];});
}
- (void)finish:(uint64_t)request json:(NSString *)json code:(NSString *)code message:(NSString *)message {
    OTAutomationRecord *record=self.pending[@(request)];if(!record)return;
    [self.pending removeObjectForKey:@(request)];
    // A failed trylock never published this record: its busy reply must not wait for that mutex.
    if(record.queued){
    pthread_mutex_lock(&queueMutex);
    [self.live removeObjectForKey:@(request)];
    NSIndexSet *removed=[self.queue indexesOfObjectsPassingTest:^BOOL(NSDictionary *item,NSUInteger index,BOOL *stop){return [item[@"ID"] unsignedLongLongValue]==request;}];
    [self.queue removeObjectsAtIndexes:removed];pthread_mutex_unlock(&queueMutex);
    }
    if(OTAutomationNow()>=record.deadline){code=@"timeout";message=@"Automation request expired";}
    else if(!code.length && atomic_load(&stopped)){code=@"cancelled";message=@"Automation request retired";}
    NSAppleEventDescriptor *reply=[OT_AUTOMATION_MANAGER replyAppleEventForSuspensionID:record.suspension];
    if(code.length)OTAutomationError(reply,code,message?:@"");
    else if(reply.descriptorType!=typeNull)[reply setParamDescriptor:[NSAppleEventDescriptor descriptorWithString:json?:@"{}"] forKeyword:keyDirectObject];
    [OT_AUTOMATION_MANAGER resumeWithSuspensionID:record.suspension];
}
- (void)drain {
    atomic_store(&stopped,true);
    // Foundation exposes the same generic pair for NSAppleEventManager target swaps.
    // This lease detects observable lower-level replacements; the private suite has one app owner.
    if(self.installed){
        for(NSUInteger i=0;i<7;i++){
            AEEventHandlerUPP current=NULL;SRefCon refcon=0;
            if(installedLeases[i]&&OT_AUTOMATION_GET_HANDLER('OpTb',OTAutomationEvents[i],&current,&refcon,false)==noErr&&current==installedHandlers[i]&&refcon==installedRefcons[i])
                [OT_AUTOMATION_MANAGER removeEventHandlerForEventClass:'OpTb' andEventID:OTAutomationEvents[i]];
            installedLeases[i]=NO;
        }
        self.installed=NO;
    }
    for(NSNumber *request in [self.pending.allKeys copy])[self finish:request.unsignedLongLongValue json:nil code:@"cancelled" message:@"Automation stopped"];
    atomic_store(&status,-1);
}
@end

uint64_t ot_automation_start(void) {
    @autoreleasepool {
        OTAutomationOwner *owner=[OTAutomationOwner new];
        pthread_mutex_lock(&OTAutomationOwnersMutex);
        if(!OTAutomationOwners)OTAutomationOwners=[NSMutableDictionary dictionary];
        owner.token=++OTAutomationNextOwner;OTAutomationOwners[@(owner.token)]=owner;
        pthread_mutex_unlock(&OTAutomationOwnersMutex);
        dispatch_async(dispatch_get_main_queue(),^{
            if(atomic_load(&owner->stopped)){[owner drain];return;}
            for(NSUInteger i=0;i<7;i++)if(OT_AUTOMATION_HANDLER_EXISTS(OTAutomationEvents[i])){atomic_store(&owner->status,-2);return;}
            owner.installed=YES;
            for(NSUInteger i=0;i<7;i++){
                [OT_AUTOMATION_MANAGER setEventHandler:owner andSelector:@selector(handle:reply:) forEventClass:'OpTb' andEventID:OTAutomationEvents[i]];
                if(OT_AUTOMATION_GET_HANDLER('OpTb',OTAutomationEvents[i],&owner->installedHandlers[i],&owner->installedRefcons[i],false)!=noErr){[owner drain];return;}
                owner->installedLeases[i]=YES;
            }
            atomic_store(&owner->status,1);
        });
        return owner.token;
    }
}
int ot_automation_status(uint64_t server){@autoreleasepool{OTAutomationOwner *owner=OTAutomationOwnerFor(server);return owner?atomic_load(&owner->status):-1;}}
int ot_automation_current(uint64_t server,uint64_t request){
    @autoreleasepool{OTAutomationOwner *owner=OTAutomationOwnerFor(server);if(!owner||atomic_load(&owner->stopped))return 0;
        pthread_mutex_lock(&owner->queueMutex);uint64_t deadline=[owner.live[@(request)] unsignedLongLongValue];pthread_mutex_unlock(&owner->queueMutex);
        return deadline && OTAutomationNow()<deadline && !atomic_load(&owner->stopped);
    }
}
char *ot_automation_pop(uint64_t server){
    @autoreleasepool{OTAutomationOwner *owner=OTAutomationOwnerFor(server);if(!owner||atomic_load(&owner->stopped))return NULL;
        pthread_mutex_lock(&owner->queueMutex);NSDictionary *item=owner.queue.firstObject;if(item)[owner.queue removeObjectAtIndex:0];pthread_mutex_unlock(&owner->queueMutex);
        if(!item)return NULL;uint64_t deadline=[item[@"Deadline"] unsignedLongLongValue],now=OTAutomationNow();if(now>=deadline)return NULL;
        NSMutableDictionary *wire=[item[@"Wire"] mutableCopy];wire[@"ID"]=item[@"ID"];wire[@"RemainingMS"]=@((deadline-now)/1000000);
        NSData *data=[NSJSONSerialization dataWithJSONObject:wire options:0 error:NULL];return data?strndup(data.bytes,data.length):NULL;
    }
}
void ot_automation_complete(uint64_t server,uint64_t request,const char *json,const char *code,const char *message){
    @autoreleasepool{OTAutomationOwner *owner=OTAutomationOwnerFor(server);if(!owner)return;NSString *j=json?@(json):nil,*c=code?@(code):nil,*m=message?@(message):nil;
        dispatch_async(dispatch_get_main_queue(),^{[owner finish:request json:j code:c message:m];});}
}
void ot_automation_stop(uint64_t server){@autoreleasepool{OTAutomationOwner *owner=OTAutomationOwnerFor(server);if(!owner||atomic_exchange(&owner->stopped,true))return;dispatch_async(dispatch_get_main_queue(),^{[owner drain];pthread_mutex_lock(&OTAutomationOwnersMutex);[OTAutomationOwners removeObjectForKey:@(server)];pthread_mutex_unlock(&OTAutomationOwnersMutex);});}}
void ot_automation_drain_main(uint64_t server){@autoreleasepool{if(![NSThread isMainThread]){ot_automation_stop(server);return;}OTAutomationOwner *owner=OTAutomationOwnerFor(server);if(owner)[owner drain];pthread_mutex_lock(&OTAutomationOwnersMutex);[OTAutomationOwners removeObjectForKey:@(server)];pthread_mutex_unlock(&OTAutomationOwnersMutex);}}
