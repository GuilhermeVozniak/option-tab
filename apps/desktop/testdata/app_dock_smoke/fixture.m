#import <Cocoa/Cocoa.h>
@interface SmokeWindow:NSWindow
@end
@implementation SmokeWindow
-(void)dealloc {printf("DEALLOC %d %s\n",getpid(),self.title.UTF8String);fflush(stdout);}
@end
@interface FixtureDelegate:NSObject<NSApplicationDelegate,NSWindowDelegate>
@property(nonatomic,strong) NSMutableArray *windows;
@property(nonatomic) BOOL empty;
@end
@implementation FixtureDelegate
-(void)applicationDidFinishLaunching:(NSNotification*)n {
 self.windows=[NSMutableArray new];
 if(!self.empty)for(int i=0;i<2;i++){
  NSWindow*w=[[SmokeWindow alloc]initWithContentRect:NSMakeRect(180+i*390,500,360,240) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
  w.delegate=self;w.releasedWhenClosed=NO;w.title=[NSString stringWithFormat:@"Smoke document %d",i+1];
  NSTextField*v=[NSTextField labelWithString:w.title];v.frame=NSMakeRect(0,0,360,240);w.contentView=v;[w makeKeyAndOrderFront:nil];[self.windows addObject:w];printf("WINDOW %d %ld %s\n",getpid(),(long)w.windowNumber,w.title.UTF8String);
 }
 [NSApp activateIgnoringOtherApps:YES];printf("READY %d %d\n",getpid(),self.empty);fflush(stdout);
 __block int tick=0;[NSTimer scheduledTimerWithTimeInterval:0.2 repeats:YES block:^(NSTimer*t){tick++;for(NSWindow*w in self.windows)if(w.visible){NSTextField*v=(NSTextField*)w.contentView;v.stringValue=[NSString stringWithFormat:@"%@\nAnimated frame %d",w.title,tick];}}];
}
-(void)windowWillClose:(NSNotification*)n {NSWindow*w=n.object;printf("CLOSED %d %ld %s\n",getpid(),(long)w.windowNumber,w.title.UTF8String);fflush(stdout);if(getenv("SMOKE_RETAIN_CLOSED") && strcmp(getenv("SMOKE_RETAIN_CLOSED"),"0")==0)dispatch_async(dispatch_get_main_queue(),^{[self.windows removeObjectIdenticalTo:w];printf("OWNER RELEASED %d count=%lu\n",getpid(),(unsigned long)self.windows.count);fflush(stdout);});}
-(BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication*)s{return NO;}
@end
int main(int argc,char**argv){@autoreleasepool{NSApplication*app=NSApplication.sharedApplication;app.activationPolicy=NSApplicationActivationPolicyRegular;FixtureDelegate*d=[FixtureDelegate new];d.empty=argc>1;app.delegate=d;[app run];}return 0;}
