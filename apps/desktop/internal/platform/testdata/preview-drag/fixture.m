#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
@interface PreviewFixture:NSObject<NSApplicationDelegate>
@property(strong) NSMutableArray *windows;
@property(copy) NSString *statePath;
@end
@implementation PreviewFixture
-(void)applicationDidFinishLaunching:(NSNotification *)notification {
 self.windows=[NSMutableArray array];
 for(int i=0;i<2;i++){
  NSWindow *w=[[NSWindow alloc]initWithContentRect:NSMakeRect(120+360*i,180+80*i,320,240) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
  w.title=[NSString stringWithFormat:@"Preview AX fixture %d",i+1];w.releasedWhenClosed=NO;
  [w orderFrontRegardless];[self.windows addObject:w];
 }
 dispatch_after(dispatch_time(DISPATCH_TIME_NOW,200*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
  NSString *state=[NSString stringWithFormat:@"%d %ld %ld\n",getpid(),(long)[self.windows[0] windowNumber],(long)[self.windows[1] windowNumber]];
  [state writeToFile:self.statePath atomically:YES encoding:NSUTF8StringEncoding error:NULL];
 });
}
@end
int main(int argc,char **argv){@autoreleasepool{
 if(argc!=2)return 2;NSApplication *app=NSApplication.sharedApplication;[app setActivationPolicy:NSApplicationActivationPolicyAccessory];
 PreviewFixture *delegate=[PreviewFixture new];delegate.statePath=@(argv[1]);app.delegate=delegate;[app run];return 0;
}}
