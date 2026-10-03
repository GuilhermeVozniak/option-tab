// Windowless, own-process Apple event delivery. No target app launch or permission prompt.
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <stdatomic.h>
#include <unistd.h>
#include "../../darwin_automation.m"
static atomic_bool finished;
static atomic_int resultCode;
int main(void){@autoreleasepool{
 [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];[NSApp finishLaunching];
 uint64_t server=ot_automation_start();
 dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED,0),^{@autoreleasepool{
  for(int i=0;i<500 && ot_automation_status(server)==0;i++)usleep(1000);
  pid_t pid=getpid();NSAppleEventDescriptor *target=[NSAppleEventDescriptor descriptorWithDescriptorType:typeKernelProcessID bytes:&pid length:sizeof(pid)];
  OSStatus permission=AEDeterminePermissionToAutomateTarget(target.aeDesc,'OpTb','qapp',false);
  if(permission!=noErr){printf("UNAVAILABLE own-process no-prompt preflight=%d\n",(int)permission);atomic_store(&finished,true);return;}
  NSAppleEventDescriptor *event=[NSAppleEventDescriptor appleEventWithEventClass:'OpTb' eventID:'qapp' targetDescriptor:target returnID:kAutoGenerateReturnID transactionID:kAnyTransactionID];
  AppleEvent reply={typeNull,NULL};OSStatus sent=AESendMessage(event.aeDesc,&reply,kAEWaitReply|kAENeverInteract|kAEDontRecord,180);
  NSAppleEventDescriptor *response=[[NSAppleEventDescriptor alloc] initWithAEDescNoCopy:&reply];
  NSString *json=[response paramDescriptorForKeyword:keyDirectObject].stringValue;
  int error=[response paramDescriptorForKeyword:keyErrorNumber].int32Value;
  if(sent==noErr&&error==errAEEventNotPermitted){printf("REFUSED actual own-process event by production local-source policy; no windows/no prompt\n");}
  else if(sent!=noErr||error||![json isEqualToString:@"{\"fixture\":true}"]){fprintf(stderr,"FAIL actual self delivery send=%d error=%d reply=%s\n",(int)sent,error,response.description.UTF8String);atomic_store(&resultCode,1);}
  else printf("PASS actual private Apple event to own PID %d, suspended reply JSON, no windows/no prompt\n",pid);
  atomic_store(&finished,true);
 }});
 NSDate *limit=[NSDate dateWithTimeIntervalSinceNow:5];
 while(!atomic_load(&finished)&&limit.timeIntervalSinceNow>0){
  [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:.005]];
  char *wire=ot_automation_pop(server);if(wire){NSData *data=[NSData dataWithBytes:wire length:strlen(wire)];NSDictionary *request=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];free(wire);ot_automation_complete(server,[request[@"ID"] unsignedLongLongValue],"{\"fixture\":true}","","");}
 }
 ot_automation_drain_main(server);ot_automation_stop(server);
 if(!atomic_load(&finished))return 2;
 return atomic_load(&resultCode);
}}
