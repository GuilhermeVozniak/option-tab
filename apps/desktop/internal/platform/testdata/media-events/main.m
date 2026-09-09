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
static int reads,mutations,prompts,guards,identityReads,runningReads;
static BOOL allowGuard=YES;
static BOOL realDelivery=NO;
static dispatch_semaphore_t sendEntered;
static NSAppleEventDescriptor *fixtureReply(NSAppleEventDescriptor *,double,NSError **);
int ot_media_guard(uintptr_t token){guards++;return token==42&&allowGuard;}
static NSRunningApplication *ot_fixture_running(NSString *p){
 runningReads++;
 if([mode isEqual:@"notRunning"]||([mode isEqual:@"disappeared"]&&runningReads>1))return nil;
 FixtureProcess *process=[FixtureProcess new];process.provider=p;process.started=([mode isEqual:@"relaunched"]&&runningReads>1)?124:123;
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
 case 'pPIS':case 'ID  ':identityReads++;value=[NSAppleEventDescriptor descriptorWithString:([mode isEqual:@"changed"]&&identityReads%2==0)?@"B":@"A"];break;
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
int main(void){@autoreleasepool{
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
