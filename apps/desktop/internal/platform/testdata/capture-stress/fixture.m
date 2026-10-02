#import <Cocoa/Cocoa.h>
#include <stdio.h>
int main(void) {
  @autoreleasepool {
    NSApplication *app = [NSApplication sharedApplication];
    [app setActivationPolicy:NSApplicationActivationPolicyAccessory];
    NSMutableArray<NSWindow *> *windows = [NSMutableArray new];
    for (int i=0; i<4; i++) {
      NSWindow *w = [[NSWindow alloc] initWithContentRect:NSMakeRect(80+i*35,80+i*35,280,180) styleMask:NSWindowStyleMaskTitled backing:NSBackingStoreBuffered defer:NO];
      w.title = [NSString stringWithFormat:@"Option Tab capture test %d", i+1];
      w.releasedWhenClosed = NO;
      w.ignoresMouseEvents = YES;
      [w orderBack:nil];
      [windows addObject:w];
      printf("%ld%s", (long)w.windowNumber, i==3 ? "\n" : ",");
    }
    fflush(stdout);
    __block NSUInteger tick=0;
    [NSTimer scheduledTimerWithTimeInterval:0.2 repeats:YES block:^(NSTimer *timer) {
      tick++;
      for (NSUInteger i=0;i<windows.count;i++) {
        windows[i].backgroundColor=[NSColor colorWithCalibratedHue:((tick+i)%12)/12.0 saturation:0.9 brightness:0.9 alpha:1];
        [windows[i] display];
      }
    }];
    [app run];
  }
  return 0;
}
