#import <Cocoa/Cocoa.h>
#include <unistd.h>
static NSString *statePath;
static NSWindow *kept, *eligible, *sheet;
static BOOL eligibleClosed;
static NSDictionary *windowState(NSWindow *window) {
  return @{
    @"id" : @(window.windowNumber),
    @"visible" : @(window.visible),
    @"minimized" : @(window.miniaturized)
  };
}
static void report(void) {
  NSDictionary *value = @{
    @"pid" : @(getpid()),
    @"kept" : windowState(kept),
    @"eligible" : windowState(eligible),
    @"sheet" : windowState(sheet),
    @"eligibleClosed" : @(eligibleClosed),
    @"sheetAttached" : (kept.attachedSheet == sheet ? @YES : @NO)
  };
  NSData *data = [NSJSONSerialization dataWithJSONObject:value
                                                 options:0
                                                   error:nil];
  [data writeToFile:statePath atomically:YES];
}
static NSWindow *makeWindow(NSString *title, NSRect frame) {
  NSWindow *window = [[NSWindow alloc]
      initWithContentRect:frame
                styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                          NSWindowStyleMaskMiniaturizable |
                          NSWindowStyleMaskResizable
                  backing:NSBackingStoreBuffered
                    defer:NO];
  window.title = title;
  window.releasedWhenClosed = NO;
  NSTextField *label = [NSTextField labelWithString:title];
  label.frame = NSMakeRect(20, 60, 300, 30);
  [window.contentView addSubview:label];
  return window;
}
int main(int argc, const char **argv) {
  @autoreleasepool {
    if (argc != 2)
      return 2;
    statePath = [NSString stringWithUTF8String:argv[1]];
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
    NSRect screen = NSScreen.mainScreen.visibleFrame;
    kept = makeWindow(
        @"D14 kept sheet parent",
        NSMakeRect(NSMinX(screen) + 80, NSMaxY(screen) - 280, 340, 180));
    eligible = makeWindow(
        @"D14 eligible exact target",
        NSMakeRect(NSMinX(screen) + 450, NSMaxY(screen) - 280, 340, 180));
    sheet = makeWindow(@"D14 protected sheet", NSMakeRect(0, 0, 260, 100));
    [kept orderFrontRegardless];
    [eligible orderFrontRegardless];
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 200 * NSEC_PER_MSEC),
                   dispatch_get_main_queue(), ^{
                     [kept beginSheet:sheet
                         completionHandler:^(NSModalResponse response){
                         }];
                   });
    [NSNotificationCenter.defaultCenter
        addObserverForName:NSWindowWillCloseNotification
                    object:eligible
                     queue:nil
                usingBlock:^(NSNotification *note) {
                  eligibleClosed = YES;
                  report();
                }];
    [NSTimer
        scheduledTimerWithTimeInterval:.05
                               repeats:YES
                                 block:^(NSTimer *timer) {
                                   NSString *command = [NSString
                                       stringWithContentsOfFile:
                                           [statePath stringByAppendingString:
                                                          @".command"]
                                                       encoding:
                                                           NSUTF8StringEncoding
                                                          error:nil];
                                   if ([command isEqualToString:@"stop"]) {
                                     [kept endSheet:sheet];
                                     [sheet orderOut:nil];
                                     [kept orderOut:nil];
                                     [eligible orderOut:nil];
                                     [NSApp terminate:nil];
                                     return;
                                   }
                                   report();
                                 }];
    [NSApp run];
  }
  return 0;
}
