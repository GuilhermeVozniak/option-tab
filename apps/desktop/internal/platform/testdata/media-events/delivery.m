#define main descriptor_main
#include "main.m"
#undef main

@interface MediaReceiver:NSObject
-(void)receive:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply;
@end
@implementation MediaReceiver
-(void)receive:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply {
 NSError *error=nil;
 NSAppleEventDescriptor *response=fixtureReply(event,2,&error);
 NSAppleEventDescriptor *value=[response paramDescriptorForKeyword:keyDirectObject];
 if(value)[reply setParamDescriptor:value forKeyword:keyDirectObject];
 if(error)[reply setParamDescriptor:[NSAppleEventDescriptor descriptorWithInt32:(int32_t)error.code] forKeyword:keyErrorNumber];
}
@end
int main(void){@autoreleasepool{
 [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
 MediaReceiver *receiver=[MediaReceiver new];
 NSAppleEventManager *manager=NSAppleEventManager.sharedAppleEventManager;
 [manager setEventHandler:receiver andSelector:@selector(receive:reply:) forEventClass:kAECoreSuite andEventID:kAEGetData];
 [manager setEventHandler:receiver andSelector:@selector(receive:reply:) forEventClass:'hook' andEventID:'Paus'];
 realDelivery=YES;
 dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{
  @autoreleasepool{
   OSStatus access=AEDeterminePermissionToAutomateTarget(address((NSRunningApplication *)[FixtureProcess new]).aeDesc,typeWildCard,typeWildCard,false);
   if(access){printf("UNAVAILABLE actual delivery no-prompt preflight=%d\n",(int)access);fflush(stdout);_Exit(3);}
   NSDictionary *result=readReply();
   if(![result[@"status"]isEqual:@"ready"]){printf("UNAVAILABLE actual delivery reply=%s\n",result.description.UTF8String);fflush(stdout);_Exit(3);}
   char *error=ot_media_command("music",getpid(),"123.000000","A","pause",0,42);
   if(error||mutations!=1){printf("FAIL actual delivery command=%s mutations=%d\n",error?:"none",mutations);fflush(stdout);_Exit(1);}
   printf("PASS actual OS AppleEvent delivery: own PID %d, metadata reads=%d, pause mutations=%d, prompts=%d\n",getpid(),reads,mutations,prompts);fflush(stdout);_Exit(0);
  }
 });
 [NSApp run];
}}
