#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <unistd.h>
#import <stdlib.h>
static bool fixtureButtonDown=true;
static bool previewFixtureButtonState(CGEventSourceStateID state,CGMouseButton button){return button==kCGMouseButtonLeft&&fixtureButtonDown;}
// Translation-unit-only substitutions. No production admission bypass exists.
#define CGEventSourceButtonState previewFixtureButtonState
#define CGEventTapCreate(...) (abort(), (CFMachPortRef)NULL)
#define CGEventPost(...) abort()
#define CGEventTapPostEvent(...) abort()
#define CGWarpMouseCursorPosition(...) (abort(), kCGErrorFailure)
#include "../../darwin_preview_drag.m"
#undef CGEventSourceButtonState
#undef CGEventTapCreate

int ot_window_pid(uint32_t window){
 NSArray *list=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionIncludingWindow,window));
 for(NSDictionary *item in list)if([item[(id)kCGWindowNumber] unsignedIntValue]==window)return [item[(id)kCGWindowOwnerPID] intValue];return 0;
}
static CGRect windowBounds(uint32_t window){
 NSArray *list=CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionIncludingWindow,window));
 for(NSDictionary *item in list)if([item[(id)kCGWindowNumber] unsignedIntValue]==window){CGRect rect; if(CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)item[(id)kCGWindowBounds],&rect))return rect;}return CGRectNull;
}
static void require(BOOL ok,const char *message){if(!ok){fprintf(stderr,"FAIL %s\n",message);exit(1);}}
static BOOL near(double a,double b){return fabs(a-b)<0.6;}
static void expectPosition(uint32_t wid,double x,double y){
 for(int i=0;i<100;i++){CGRect b=windowBounds(wid);if(near(b.origin.x,x)&&near(b.origin.y,y))return;usleep(10000);}
 CGRect b=windowBounds(wid);fprintf(stderr,"wanted %.1f,%.1f actual %.1f,%.1f\n",x,y,b.origin.x,b.origin.y);require(NO,"exact position not observed");
}
int main(int argc,char **argv){@autoreleasepool{
 if(argc==2&&!strcmp(argv[1],"--foreground")){printf("%d\n",NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier);return 0;}
 require(argc==5,"expected PID, two explicit window IDs and original foreground");int pid=atoi(argv[1]);uint32_t first=(uint32_t)strtoul(argv[2],NULL,10),second=(uint32_t)strtoul(argv[3],NULL,10);
 require(pid>0&&pid!=getpid()&&first&&second&&first!=second,"invalid fixture identity");
 NSRunningApplication *fixture=[NSRunningApplication runningApplicationWithProcessIdentifier:pid];
 require([fixture.bundleIdentifier isEqualToString:@"com.optiontab.preview-drag-fixture"],"target is not the disposable fixture bundle");
 require(ot_window_pid(first)==pid&&ot_window_pid(second)==pid,"fixture window owner mismatch");
 pid_t foreground=atoi(argv[4]);require(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier==foreground,"fixture launch changed foreground");
 require(AXIsProcessTrusted(),"driver lacks Accessibility permission (no prompt requested)");
 CGRect original=windowBounds(first),other=windowBounds(second);require(!CGRectIsNull(original)&&!CGRectIsNull(other),"missing fixture CG bounds");
 printf("FIXTURE pid=%d first=%u second=%u foreground=%d original=%.1f,%.1f %.1fx%.1f\n",pid,first,second,foreground,original.origin.x,original.origin.y,original.size.width,original.size.height);
 void *owner=ot_preview_drag_create(0);require(owner!=NULL,"allocation");OTPreviewDragSize size={0};
 int prepared=-1;for(int attempt=0;attempt<20;attempt++){
  prepared=ot_preview_drag_prepare(owner,first,pid,&size);if(prepared==0)break;
  ot_preview_drag_destroy(owner);owner=ot_preview_drag_create(0);usleep(50000);
 }
 if(prepared!=0)fprintf(stderr,"prepare status=%d\n",prepared);require(prepared==0,"production exact AX prepare refused");
 double x=original.origin.x+80,y=original.origin.y+60;
 require(ot_preview_drag_position(owner,x,y)==0,"production AXPosition write refused");expectPosition(first,x,y);
 CGRect moved=windowBounds(first);require(CGSizeEqualToSize(moved.size,original.size),"window was resized");require(CGRectEqualToRect(windowBounds(second),other),"other fixture window changed");
 ot_preview_drag_cancel(owner);require(ot_preview_drag_position(owner,x+30,y+30)!=0,"explicit cancel allowed late write");expectPosition(first,x,y);ot_preview_drag_destroy(owner);
 owner=ot_preview_drag_create(0);require(ot_preview_drag_prepare(owner,first,pid,&size)==0,"second AX prepare refused");
 fixtureButtonDown=false;ot_preview_drag_check(owner);require(ot_preview_drag_position(owner,x+40,y+40)!=0,"button-up allowed late write");ot_preview_drag_destroy(owner);expectPosition(first,x,y);
 require(CGSizeEqualToSize(windowBounds(first).size,original.size)&&CGRectEqualToRect(windowBounds(second),other),"final size/other-window drift");
 require(NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier==foreground,"foreground changed");
 printf("PASS production AX prepare/position +80,+60; size/other window preserved; cancel/button-up refuse late writes; foreground preserved; no tap/input\n");return 0;
}}
