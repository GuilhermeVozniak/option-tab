#import <AppKit/AppKit.h>
#import <Carbon/Carbon.h>
#import <unistd.h>
static NSRunningApplication *ot_fixture_running(NSString *);
static OSStatus ot_fixture_permission(NSRunningApplication *,BOOL);
static NSAppleEventDescriptor *ot_fixture_send(NSAppleEventDescriptor *,double,NSError **);
#define OT_MEDIA_FIXTURE 1
#include "../../darwin_media.m"

@interface FixtureProcess:NSObject
@property(nonatomic,copy) NSString *provider;
@property(nonatomic) NSTimeInterval started;
@end
@implementation FixtureProcess
-(pid_t)processIdentifier{return getpid();}
-(NSDate *)launchDate{return [NSDate dateWithTimeIntervalSince1970:self.started?:123];}
-(BOOL)isTerminated{return NO;}
@end
static NSString *mode=@"ready";
static NSString *failedProvider=nil;
static int reads,mutations,prompts,guards,identityReads,runningReads,lyricReads;
static double firstLyricDeadline;
static BOOL allowGuard=YES;
static BOOL realDelivery=NO;
static dispatch_semaphore_t sendEntered;
static NSAppleEventDescriptor *fixtureReply(NSAppleEventDescriptor *,double,NSError **);
int ot_media_guard(uintptr_t token){guards++;if([mode isEqual:@"lyricsGuardRelaunch"])mode=@"relaunched";return token==42&&allowGuard;}
static NSRunningApplication *ot_fixture_running(NSString *p){
 runningReads++;
 if([mode isEqual:@"notRunning"]||([mode isEqual:@"disappeared"]&&runningReads>1)||([mode isEqual:@"lyricsPostExit"]&&runningReads>2))return nil;
 FixtureProcess *process=[FixtureProcess new];process.provider=p;process.started=(([mode isEqual:@"relaunched"]&&runningReads>1)||([mode isEqual:@"lyricsPostRelaunch"]&&runningReads>2))?124:123;
 return (NSRunningApplication *)process;
}
static OSStatus ot_fixture_permission(NSRunningApplication *a,BOOL ask){if(ask)prompts++;FixtureProcess *process=(FixtureProcess *)a;return [mode isEqual:@"denied"]||[failedProvider isEqual:process.provider]?errAEEventNotPermitted:noErr;}
static NSAppleEventDescriptor *ot_fixture_send(NSAppleEventDescriptor *e,double timeout,NSError **error){
 if(realDelivery)return [e sendEventWithOptions:kAEWaitReply|kAENeverInteract|kAEDontRecord timeout:timeout error:error];
 return fixtureReply(e,timeout,error);
}
static NSAppleEventDescriptor *fixtureReply(NSAppleEventDescriptor *e,double timeout,NSError **error){
 NSCAssert(timeout>0&&timeout<=2,@"deadline");
 NSAppleEventDescriptor *target=[e attributeDescriptorForKeyword:keyAddressAttr];pid_t pid=0;NSData *addressData=target.data;if(addressData.length==sizeof(pid))memcpy(&pid,addressData.bytes,sizeof(pid));NSCAssert(target.descriptorType==typeKernelProcessID&&pid==getpid(),@"only fixture PID");
 OSType suite=[e attributeDescriptorForKeyword:keyEventClassAttr].typeCodeValue,code=[e attributeDescriptorForKeyword:keyEventIDAttr].typeCodeValue;
 NSAppleEventDescriptor *reply=[NSAppleEventDescriptor appleEventWithEventClass:kCoreEventClass eventID:kAEAnswer targetDescriptor:[NSAppleEventDescriptor nullDescriptor] returnID:0 transactionID:0];
 if([mode isEqual:@"inFlightTimeout"]){if(sendEntered)dispatch_semaphore_signal(sendEntered);usleep((useconds_t)(timeout*1000000));*error=[NSError errorWithDomain:@"Fixture" code:errAETimeout userInfo:nil];return nil;}
 if(code!=kAEGetData){NSCAssert((suite=='hook'||suite=='spfy')&&(code=='Play'||code=='Paus'||code=='Prev'||code=='Next') ||(suite==kAECoreSuite&&code==kAESetData),@"fixed command allowlist");mutations++;return reply;}
 NSCAssert(suite==kAECoreSuite,@"fixed read suite");reads++;
 if([mode isEqual:@"timeout"]){*error=[NSError errorWithDomain:@"Fixture" code:errAETimeout userInfo:nil];return nil;}
 OSType prop=[[e paramDescriptorForKeyword:keyDirectObject] descriptorForKeyword:keyAEKeyData].typeCodeValue;
 NSAppleEventDescriptor *value=nil;
 switch(prop){
 case 'pPIS':case 'ID  ':identityReads++;if([mode isEqual:@"lyricsDeadline"]&&identityReads==1){firstLyricDeadline=timeout;usleep(50000);}if([mode isEqual:@"lyricsPostError"]&&identityReads==2){*error=[NSError errorWithDomain:@"Fixture" code:errAETimeout userInfo:nil];return nil;}value=[NSAppleEventDescriptor descriptorWithString:([mode isEqual:@"changed"]&&identityReads%2==0)?@"B":@"A"];break;
 case 'pLyr':{
   lyricReads++;
   NSAppleEventDescriptor *object=[e paramDescriptorForKeyword:keyDirectObject],*container=[object descriptorForKeyword:keyAEContainer];
   NSCAssert([object descriptorForKeyword:keyAEDesiredClass].typeCodeValue==typeProperty&&[object descriptorForKeyword:keyAEKeyForm].enumCodeValue==formPropertyID&&[container descriptorForKeyword:keyAEKeyData].typeCodeValue=='pTrk',@"lyrics must read current track property");
   NSCAssert(guards>0&&allowGuard,@"lyrics getter before final admission");
   if([mode isEqual:@"lyricsDeadline"])NSCAssert(timeout<firstLyricDeadline-0.03,@"lyrics reset total deadline");
   if([mode isEqual:@"lyricsWrongType"])value=[NSAppleEventDescriptor descriptorWithInt32:7];
   else if([mode isEqual:@"lyricsInvalidUTF8"]){const unsigned char byte=0xff;value=[NSAppleEventDescriptor descriptorWithDescriptorType:typeUTF8Text bytes:&byte length:1];}
   else if([mode isEqual:@"lyricsOversize"])value=[NSAppleEventDescriptor descriptorWithString:[@"é" stringByPaddingToLength:524289 withString:@"é" startingAtIndex:0]];
   else value=[NSAppleEventDescriptor descriptorWithString:[mode isEqual:@"lyricsMissing"]?@"":[mode isEqual:@"lyricsPlain"]?@"Original plain fixture":@"[00:01]Original café fixture"];
   break;
 }
 case 'pnam':value=[NSAppleEventDescriptor descriptorWithString:[mode isEqual:@"oversize"]?[@"x" stringByPaddingToLength:4097 withString:@"x" startingAtIndex:0]:@"Original fixture track"];break;
 case 'pArt':case 'pAlb':value=[NSAppleEventDescriptor descriptorWithString:@"Original fixture"];break;
 case 'aUrl':value=[NSAppleEventDescriptor descriptorWithString:@""];break;
 case 'pPlS':value=[NSAppleEventDescriptor descriptorWithEnumCode:'kPSP'];break;
 case 'pPos':case 'pDur':{double n=prop=='pPos'?1:30;value=[mode isEqual:@"wrongType"]?[NSAppleEventDescriptor descriptorWithString:@"wrong"]:[NSAppleEventDescriptor descriptorWithDescriptorType:typeIEEE64BitFloatingPoint bytes:&n length:sizeof(n)];break;}
 default:NSCAssert(NO,@"unexpected read property");
 }
 [reply setParamDescriptor:value forKeyword:keyDirectObject];return reply;
}
static NSDictionary *providerReadReply(const char *provider){char *r=ot_media_read(provider);NSData *d=[NSData dataWithBytes:r length:strlen(r)];free(r);return [NSJSONSerialization JSONObjectWithData:d options:0 error:nil];}
static NSDictionary *readReply(void){return providerReadReply("music");}
static NSDictionary *lyricsReply(const char *provider,int pid,const char *birth,const char *track){
 char *r=ot_media_lyrics(provider,pid,birth,track,42);NSCAssert(r,@"lyrics reply absent");NSData *d=[NSData dataWithBytes:r length:strlen(r)];free(r);return [NSJSONSerialization JSONObjectWithData:d options:0 error:nil];
}
int main(void){@autoreleasepool{
 for(NSString *scenario in @[@"ready",@"lyricsPlain",@"lyricsMissing",@"lyricsWrongType",@"lyricsInvalidUTF8",@"lyricsOversize",@"notRunning",@"denied",@"changed",@"timeout",@"disappeared",@"relaunched",@"lyricsGuardRelaunch",@"lyricsPostError",@"lyricsPostExit",@"lyricsPostRelaunch",@"lyricsDeadline"]){
   mode=scenario;reads=identityReads=runningReads=lyricReads=guards=0;allowGuard=YES;
   NSDictionary *r=lyricsReply("music",getpid(),"123.000000","A");
   NSString *expected=[@[@"ready",@"lyricsPlain",@"lyricsDeadline"]containsObject:scenario]?@"ready":[scenario isEqual:@"lyricsMissing"]?@"missing":@"unavailable";
   NSCAssert([r[@"status"]isEqual:expected],@"lyrics scenario %@ result %@",scenario,r);
   if([expected isEqual:@"ready"])NSCAssert([r[@"text"]isEqual:[scenario isEqual:@"lyricsPlain"]?@"Original plain fixture":@"[00:01]Original café fixture"],@"provider text changed");
   else NSCAssert(!r[@"text"],@"failed lyrics leaked text");
   if([@[@"notRunning",@"denied",@"timeout",@"disappeared",@"relaunched",@"lyricsGuardRelaunch"]containsObject:scenario])NSCAssert(lyricReads==0,@"retired request reached lyrics getter");
 }
 mode=@"ready";runningReads=identityReads=lyricReads=guards=0;allowGuard=NO;NSDictionary *refusedLyrics=lyricsReply("music",getpid(),"123.000000","A");NSCAssert([refusedLyrics[@"status"]isEqual:@"unavailable"]&&lyricReads==0&&guards==1,@"lyrics final guard refusal");allowGuard=YES;
 for(NSArray *target in @[@[@"spotify",@(getpid()),@"123.000000",@"A"],@[@"music",@(getpid()+1),@"123.000000",@"A"],@[@"music",@(getpid()),@"124.000000",@"A"],@[@"music",@(getpid()),@"123.000000",@"B"]]){runningReads=identityReads=lyricReads=guards=0;NSDictionary *r=lyricsReply([target[0]UTF8String],[target[1]intValue],[target[2]UTF8String],[target[3]UTF8String]);NSCAssert(![r[@"status"]isEqual:@"ready"]&&lyricReads==0,@"wrong lyric target admitted");}
 NSCAssert(prompts==0&&mutations==0,@"lyrics requested consent or mutated player");
 printf("PASS provider lyrics: 17 descriptor scenarios, wrong targets, final guard, original UTF8/plain text, no prompt/mutation, shared deadline\n");
 guards=0;
 for(NSString *scenario in @[@"notRunning",@"denied",@"changed",@"wrongType",@"oversize",@"timeout",@"disappeared",@"relaunched",@"ready"]){mode=scenario;reads=identityReads=runningReads=0;NSDictionary *r=readReply();NSString *expected=[scenario isEqual:@"notRunning"]||[scenario isEqual:@"disappeared"]||[scenario isEqual:@"relaunched"]?@"notRunning":[scenario isEqual:@"denied"]?@"denied":[scenario isEqual:@"ready"]?@"ready":@"unavailable";NSCAssert([r[@"status"] isEqual:expected],@"scenario %@ result %@",scenario,r);if([scenario isEqual:@"notRunning"]||[scenario isEqual:@"denied"])NSCAssert(reads==0,@"preflight must prevent reads");}
 NSCAssert(prompts==0,@"sampling requested consent");
 mode=@"ready";failedProvider=@"spotify";runningReads=reads=identityReads=0;NSDictionary *music=providerReadReply("music");runningReads=reads=identityReads=0;NSDictionary *spotify=providerReadReply("spotify");NSCAssert([music[@"status"]isEqual:@"ready"]&&[spotify[@"status"]isEqual:@"denied"],@"provider failure isolation music=%@ spotify=%@",music,spotify);failedProvider=nil;
 mode=@"ready";allowGuard=NO;char *err=ot_media_command("music",getpid(),"123.000000","A","play",0,42);NSCAssert(err&&mutations==0&&guards==1,@"final refusal");free(err);
 allowGuard=YES;err=ot_media_command("music",getpid(),"123.000000","A","pause",0,42);NSCAssert(!err&&mutations==1,@"exact command");
 err=ot_media_command("spotify",getpid(),"123.000000","A","seek",1000,42);NSCAssert(err&&mutations==1,@"Spotify seek refused");free(err);
 mode=@"notRunning";err=ot_media_command("music",getpid(),"123.000000","A","play",0,42);NSCAssert(err&&mutations==1,@"disappeared process");free(err);
 mode=@"inFlightTimeout";runningReads=0;sendEntered=dispatch_semaphore_create(0);__block char *slowError=nil;CFAbsoluteTime started=CFAbsoluteTimeGetCurrent();dispatch_group_t group=dispatch_group_create();dispatch_group_async(group,dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{slowError=ot_media_command("music",getpid(),"123.000000","A","play",0,42);});NSCAssert(dispatch_semaphore_wait(sendEntered,dispatch_time(DISPATCH_TIME_NOW,500*NSEC_PER_MSEC))==0,@"send never entered");BOOL ownerCancelled=YES;(void)ownerCancelled;NSCAssert(dispatch_group_wait(group,dispatch_time(DISPATCH_TIME_NOW,2500*NSEC_PER_MSEC))==0,@"cancelled owner did not join bounded send");double elapsed=CFAbsoluteTimeGetCurrent()-started;NSCAssert(slowError&&elapsed>=1.8&&elapsed<2.5&&mutations==1,@"bounded in-flight join error=%s elapsed=%f mutations=%d",slowError?:"none",elapsed,mutations);free(slowError);
 printf("PASS native descriptors: 9 metadata scenarios, mid-read process retirement, provider-isolated denial, bounded in-flight join, no sampling prompts, PID-only target, final guard refusal, exact pause, unverified seek refusal\n");
return 0;
}}
