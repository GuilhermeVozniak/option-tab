#import <Cocoa/Cocoa.h>
@interface Fixture : NSObject <NSApplicationDelegate>
@property NSWindow *window;
@end
@implementation Fixture
- (void)applicationDidFinishLaunching:(NSNotification *)note {
 self.window=[[NSWindow alloc] initWithContentRect:NSMakeRect(280,300,400,180) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable backing:NSBackingStoreBuffered defer:NO];
 self.window.title=@"Disposable Dock Observer Fixture";self.window.releasedWhenClosed=NO;[self.window orderFrontRegardless];
}
@end
int main(void){@autoreleasepool {NSApplication *app=NSApplication.sharedApplication;Fixture *fixture=[Fixture new];app.delegate=fixture;[app setActivationPolicy:NSApplicationActivationPolicyRegular];[app run];}}
