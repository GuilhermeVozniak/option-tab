#import <Cocoa/Cocoa.h>
#include <stdio.h>
int main(void) {
 @autoreleasepool {
  NSApplication *app=[NSApplication sharedApplication];
  [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
  __block NSWindow *window=[[NSWindow alloc] initWithContentRect:NSMakeRect(100,100,320,200) styleMask:(NSWindowStyleMaskTitled|NSWindowStyleMaskMiniaturizable) backing:NSBackingStoreBuffered defer:NO];
  window.title=@"Option Tab disposable capture fixture";
  window.releasedWhenClosed=NO;
  [window orderFrontRegardless];
  printf("%ld\n",(long)window.windowNumber);fflush(stdout);
  __block NSUInteger tick=0;
  NSTimer *animation=[NSTimer scheduledTimerWithTimeInterval:0.20 repeats:YES block:^(NSTimer *timer){
   tick++;window.backgroundColor=[NSColor colorWithCalibratedHue:(tick%12)/12.0 saturation:0.9 brightness:0.9 alpha:1.0];
   [window display];
  }];
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_UTILITY,0),^{
   char line[32];
   while(fgets(line,sizeof(line),stdin)) {
    NSString *command=[[NSString stringWithUTF8String:line] stringByTrimmingCharactersInSet:NSCharacterSet.whitespaceAndNewlineCharacterSet];
    dispatch_async(dispatch_get_main_queue(),^{
     if([command isEqualToString:@"minimize"]) [window miniaturize:nil];
     else if([command isEqualToString:@"hide"]) [app hide:nil];
     else if([command isEqualToString:@"restore"]) { [app unhideWithoutActivation];[window deminiaturize:nil];[window orderFrontRegardless]; }
     else if([command isEqualToString:@"close"]) [window close];
     else if([command isEqualToString:@"destroy"]) { [animation invalidate];[window close];window=nil; }
     dispatch_after(dispatch_time(DISPATCH_TIME_NOW, ([command isEqualToString:@"close"] || [command isEqualToString:@"destroy"]) ? 0 : 700*NSEC_PER_MSEC),dispatch_get_main_queue(),^{
      if ([command isEqualToString:@"minimize"] && !window.miniaturized) printf("failed minimize state\n");
      else if ([command isEqualToString:@"hide"] && !app.hidden) printf("failed hidden state\n");
      else printf("done %s\n",command.UTF8String);
      fflush(stdout);
     });
    });
   }
  });
  [app run];
 }
 return 0;
}
