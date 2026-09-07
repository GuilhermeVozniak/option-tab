// Compiled sender and separate disposable windowless receiver. No osascript or UI.
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <unistd.h>
#include "../../darwin_automation.m"
static int receiver(NSString *readyFile){
 [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];[NSApp finishLaunching];
 uint64_t server=ot_automation_start();NSDate *limit=[NSDate dateWithTimeIntervalSinceNow:8];BOOL ready=NO,answered=NO;
 while(limit.timeIntervalSinceNow>0){
  NSEvent *next=[NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate dateWithTimeIntervalSinceNow:.005] inMode:NSDefaultRunLoopMode dequeue:YES];if(next)[NSApp sendEvent:next];
  if(!ready&&ot_automation_status(server)==1){printf("READY %d\n",getpid());fflush(stdout);if(readyFile)[[@(getpid()) stringValue] writeToFile:readyFile atomically:YES encoding:NSUTF8StringEncoding error:NULL];ready=YES;}
  char *wire=ot_automation_pop(server);if(wire){NSData *data=[NSData dataWithBytes:wire length:strlen(wire)];NSDictionary *request=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];free(wire);ot_automation_complete(server,[request[@"ID"] unsignedLongLongValue],"{\"fixture\":true}","","");answered=YES;limit=[NSDate dateWithTimeIntervalSinceNow:.1];}
 }
 ot_automation_drain_main(server);return answered?0:2;
}
int main(int argc,const char *argv[]){@autoreleasepool{
 if(argc>=2&&strcmp(argv[1],"--receiver")==0)return receiver(argc==3?@(argv[2]):nil);
 if(argc==4&&strcmp(argv[1],"--send")==0){
  pid_t pid=atoi(argv[2]);NSRunningApplication *app=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];
  if(!app || ![app.bundleURL.URLByResolvingSymlinksInPath.path isEqualToString:[NSURL fileURLWithPath:@(argv[3])].URLByResolvingSymlinksInPath.path]){fprintf(stderr,"refused fixture process/bundle mismatch\n");return 1;}
  pid_t foreground=NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
  NSAppleEventDescriptor *target=[NSAppleEventDescriptor descriptorWithDescriptorType:typeKernelProcessID bytes:&pid length:sizeof(pid)];
  printf("No-prompt preflight receiverPID=%d\n",pid);fflush(stdout);OSStatus permission=AEDeterminePermissionToAutomateTarget(target.aeDesc,'OpTb','qapp',false);printf("Preflight result=%d\n",(int)permission);fflush(stdout);
  if(permission!=noErr){printf("UNAVAILABLE LaunchServices fixture no-prompt preflight=%d receiverPID=%d\n",(int)permission,pid);return 0;}
  NSAppleEventDescriptor *e=[NSAppleEventDescriptor appleEventWithEventClass:'OpTb' eventID:'qapp' targetDescriptor:target returnID:kAutoGenerateReturnID transactionID:kAnyTransactionID];
  AppleEvent reply={typeNull,NULL};OSStatus sent=AESendMessage(e.aeDesc,&reply,kAEWaitReply|kAENeverInteract|kAEDontRecord,180);
  NSAppleEventDescriptor *response=[[NSAppleEventDescriptor alloc] initWithAEDescNoCopy:&reply];int failure=[response paramDescriptorForKeyword:keyErrorNumber].int32Value;
  BOOL ok=sent==0&&failure==0&&[[response paramDescriptorForKeyword:keyDirectObject].stringValue isEqualToString:@"{\"fixture\":true}"];
  printf("%s LaunchServices fixture receiverPID=%d send=%d error=%d foregroundUnchanged=%d reply=%s\n",ok?"PASS":"FAIL",pid,(int)sent,failure,foreground==NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier,response.description.UTF8String);
  return ok?0:1;
 }
 NSTask *child=[NSTask new];child.executableURL=[NSURL fileURLWithPath:@(argv[0])];child.arguments=@[@"--receiver"];NSPipe *output=[NSPipe pipe];child.standardOutput=output;
 NSError *error=nil;if(![child launchAndReturnError:&error]){fprintf(stderr,"receiver launch failed: %s\n",error.description.UTF8String);return 1;}
 NSData *readyData=[output.fileHandleForReading availableData];NSString *ready=[[NSString alloc] initWithData:readyData encoding:NSUTF8StringEncoding];
 if(![ready hasPrefix:@"READY "]){if(child.running)[child terminate];[child waitUntilExit];fprintf(stderr,"receiver did not become ready\n");return 1;}
 pid_t pid=child.processIdentifier;NSAppleEventDescriptor *target=[NSAppleEventDescriptor descriptorWithDescriptorType:typeKernelProcessID bytes:&pid length:sizeof(pid)];
 printf("No-prompt preflight receiverPID=%d\n",pid);fflush(stdout);OSStatus permission=AEDeterminePermissionToAutomateTarget(target.aeDesc,'OpTb','qapp',false);printf("Preflight result=%d\n",(int)permission);fflush(stdout);
 if(permission!=noErr){printf("UNAVAILABLE cross-process no-prompt preflight=%d receiverPID=%d\n",(int)permission,pid);if(child.running)[child terminate];[child waitUntilExit];return 0;}
 NSAppleEventDescriptor *event=[NSAppleEventDescriptor appleEventWithEventClass:'OpTb' eventID:'qapp' targetDescriptor:target returnID:kAutoGenerateReturnID transactionID:kAnyTransactionID];
 AppleEvent reply={typeNull,NULL};OSStatus sent=AESendMessage(event.aeDesc,&reply,kAEWaitReply|kAENeverInteract|kAEDontRecord,180);NSAppleEventDescriptor *response=[[NSAppleEventDescriptor alloc] initWithAEDescNoCopy:&reply];
 NSString *json=[response paramDescriptorForKeyword:keyDirectObject].stringValue;int nativeError=[response paramDescriptorForKeyword:keyErrorNumber].int32Value;
 BOOL success=sent==noErr&&nativeError==0&&[json isEqualToString:@"{\"fixture\":true}"];
 if(success)printf("PASS actual cross-process private Apple event receiverPID=%d suspended reply JSON; no windows/no prompt\n",pid);
 else fprintf(stderr,"FAIL cross-process send=%d error=%d reply=%s\n",(int)sent,nativeError,response.description.UTF8String);
 if(child.running)[child terminate];[child waitUntilExit];return success?0:1;
}}
