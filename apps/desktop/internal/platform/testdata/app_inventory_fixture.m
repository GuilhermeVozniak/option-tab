#import <Cocoa/Cocoa.h>

@interface FixtureDelegate : NSObject <NSApplicationDelegate>
@property(nonatomic, strong) NSWindow *window;
@property(nonatomic, copy) NSString *statePath;
@end

@implementation FixtureDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)note {
  (void)note;
  self.window = [[NSWindow alloc] initWithContentRect:NSMakeRect(180, 180, 260, 120)
                                            styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable
                                              backing:NSBackingStoreBuffered defer:NO];
  self.window.title = @"Option Tab Inventory Fixture";
  [self.window makeKeyAndOrderFront:nil];
  [NSApp activateIgnoringOtherApps:YES];
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 250 * NSEC_PER_MSEC), dispatch_get_main_queue(), ^{
    [self.window close];
    self.window = nil;
    NSString *state = [NSString stringWithFormat:@"%d closed\n", getpid()];
    [state writeToFile:self.statePath atomically:YES encoding:NSUTF8StringEncoding error:nil];
  });
}
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender { return NO; }
@end

int main(int argc, const char *argv[]) {
  @autoreleasepool {
    if (argc != 2) return 2;
    NSApplication *app = NSApplication.sharedApplication;
    app.activationPolicy = NSApplicationActivationPolicyRegular;
    FixtureDelegate *delegate = [FixtureDelegate new];
    delegate.statePath = [NSString stringWithUTF8String:argv[1]];
    app.delegate = delegate;
    [app run];
  }
  return 0;
}
