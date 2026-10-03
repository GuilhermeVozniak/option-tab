#import <Cocoa/Cocoa.h>
@interface Fixture : NSObject <NSApplicationDelegate, NSWindowDelegate>
@property NSMutableArray<NSWindow *> *windows;
@property NSInteger serial;
@end
@implementation Fixture
- (void)newWindow:(id)sender {
  self.serial++;
  NSWindow *w = [[NSWindow alloc] initWithContentRect:NSMakeRect(240+self.serial*24,260,360,210) styleMask:NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskMiniaturizable|NSWindowStyleMaskResizable backing:NSBackingStoreBuffered defer:NO];
  w.title = [NSString stringWithFormat:@"ActionSmokeFixture %ld%@", (long)self.serial, self.serial == 4 ? @" Refuses Close" : @""];
  w.delegate=self; w.releasedWhenClosed=NO;
  NSTextField *label=[NSTextField labelWithString:@"Disposable Option Tab action verification fixture"];
  label.frame=NSMakeRect(20,80,320,40);[w.contentView addSubview:label];
  [self.windows addObject:w];[w makeKeyAndOrderFront:nil];[self writeState];
}
- (BOOL)windowShouldClose:(NSWindow *)sender {return ![sender.title containsString:@"Refuses Close"];}
- (void)windowWillClose:(NSNotification *)note {[self.windows removeObject:note.object];}
- (void)writeState {
  NSMutableArray *windows=[NSMutableArray array];
  for (NSWindow *w in self.windows) [windows addObject:@{@"id":@(w.windowNumber),@"title":w.title,@"minimized":@(w.miniaturized),@"key":@(w.keyWindow)}];
  NSDictionary *state=@{@"pid":@(getpid()),@"bundle":NSBundle.mainBundle.bundleIdentifier,@"windows":windows};
  NSData *data=[NSJSONSerialization dataWithJSONObject:state options:0 error:nil];
  [data writeToFile:[NSString stringWithUTF8String:getenv("OPTION_TAB_ACTION_FIXTURE_STATE")] atomically:YES];
}
- (void)applicationDidFinishLaunching:(NSNotification *)note {
  self.windows=[NSMutableArray array];
  NSMenu *bar=[[NSMenu alloc] init];NSMenuItem *root=[[NSMenuItem alloc] initWithTitle:@"File" action:nil keyEquivalent:@""];
  NSMenu *file=[[NSMenu alloc] initWithTitle:@"File"];
  NSMenuItem *item=[[NSMenuItem alloc] initWithTitle:@"New Window" action:@selector(newWindow:) keyEquivalent:@"n"];item.target=self;
  [file addItem:item];root.submenu=file;[bar addItem:root];NSApp.mainMenu=bar;
  [self newWindow:nil];[self newWindow:nil];[NSApp activateIgnoringOtherApps:YES];
  [NSTimer scheduledTimerWithTimeInterval:0.1 repeats:YES block:^(NSTimer *timer){[self writeState];}];
}
@end
int main(void) {@autoreleasepool {NSApplication *app=NSApplication.sharedApplication;Fixture *delegate=[Fixture new];app.delegate=delegate;[app setActivationPolicy:NSApplicationActivationPolicyRegular];[app run];}}
