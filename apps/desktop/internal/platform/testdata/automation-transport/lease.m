// No windows or events sent: inspect public handler ownership metadata.
#import <Cocoa/Cocoa.h>
#import <Carbon/Carbon.h>
#include <assert.h>
#include "../../darwin_automation.m"
static OSErr OTLeaseReplacement(const AppleEvent *event,AppleEvent *reply,SRefCon refcon){return noErr;}
@interface OTLeaseTarget:NSObject
- (void)handle:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply;
@end
@implementation OTLeaseTarget
- (void)handle:(NSAppleEventDescriptor *)event reply:(NSAppleEventDescriptor *)reply {}
@end
int main(void){@autoreleasepool{
 [NSApplication sharedApplication];[NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
 NSAppleEventManager *manager=NSAppleEventManager.sharedAppleEventManager;OTLeaseTarget *first=[OTLeaseTarget new],*second=[OTLeaseTarget new];
 [manager setEventHandler:first andSelector:@selector(handle:reply:) forEventClass:'OpTb' andEventID:'qapp'];
 AEEventHandlerUPP a=NULL,b=NULL;SRefCon ar=0,br=0;OSStatus as=AEGetEventHandler('OpTb','qapp',&a,&ar,false);
 [manager setEventHandler:second andSelector:@selector(handle:reply:) forEventClass:'OpTb' andEventID:'qapp'];OSStatus bs=AEGetEventHandler('OpTb','qapp',&b,&br,false);
 printf("NSAppleEventManager replacement status=%d/%d UPPsame=%d refconSame=%d first=%p/%ld second=%p/%ld\n",(int)as,(int)bs,a==b,ar==br,a,(long)ar,b,(long)br);
 [manager removeEventHandlerForEventClass:'OpTb' andEventID:'qapp'];
 uint64_t server=ot_automation_start();NSDate *limit=[NSDate dateWithTimeIntervalSinceNow:1];
 while(ot_automation_status(server)==0&&limit.timeIntervalSinceNow>0)[[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:.001]];
 assert(ot_automation_status(server)==1);
 assert(AEInstallEventHandler('OpTb','qapp',OTLeaseReplacement,(SRefCon)(uintptr_t)1234,false)==noErr);
 ot_automation_drain_main(server);AEEventHandlerUPP replacement=NULL;SRefCon replacementRef=0;
 assert(AEGetEventHandler('OpTb','qapp',&replacement,&replacementRef,false)==noErr);
 assert(replacement==OTLeaseReplacement&&replacementRef==(SRefCon)(uintptr_t)1234);
 assert(AERemoveEventHandler('OpTb','qapp',OTLeaseReplacement,false)==noErr);
 printf("PASS later lower-level Apple event handler replacement preserved on drain\n");
}}
