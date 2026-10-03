#import <AppKit/AppKit.h>
#import <libproc.h>
#import <sys/proc_info.h>
#import <unistd.h>

static void reply(NSDictionary *value) {
  NSData *data = [NSJSONSerialization dataWithJSONObject:value options:0 error:NULL];
  if (data.length > 4096) _exit(2);
  fwrite(data.bytes, 1, data.length, stdout);
  fputc('\n', stdout);
  fflush(stdout);
}
static void finish(void) {
  NSApp.dockTile.badgeLabel = nil;
  [NSApp.dockTile display];
  [NSApp terminate:nil];
}
@interface BadgeFixture : NSObject <NSApplicationDelegate>
@end
@implementation BadgeFixture
- (void)applicationDidFinishLaunching:(NSNotification *)notification {
  (void)notification;
  struct proc_bsdinfo info = {0};
  if (proc_pidinfo(getpid(), PROC_PIDTBSDINFO, 0, &info, sizeof(info)) != sizeof(info)) {
    finish();
    return;
  }
  NSURL *url = NSBundle.mainBundle.bundleURL.URLByStandardizingPath.URLByResolvingSymlinksInPath;
  reply(@{@"ready": @YES, @"pid": @(getpid()), @"seconds": @(info.pbi_start_tvsec),
          @"micros": @(info.pbi_start_tvusec), @"bundleID": NSBundle.mainBundle.bundleIdentifier,
          @"bundleURL": url.absoluteString});
  dispatch_after(dispatch_time(DISPATCH_TIME_NOW, 45 * NSEC_PER_SEC), dispatch_get_main_queue(), ^{ finish(); });
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
    @autoreleasepool {
      char line[32];
      unsigned commands = 0;
      while (commands++ < 8 && fgets(line, sizeof(line), stdin)) {
        NSString *command = [NSString stringWithUTF8String:line];
        if (![command isEqualToString:@"count\n"] && ![command isEqualToString:@"indicator\n"] &&
            ![command isEqualToString:@"empty\n"] && ![command isEqualToString:@"quit\n"]) break;
        dispatch_async(dispatch_get_main_queue(), ^{
          if ([command isEqualToString:@"quit\n"]) { reply(@{@"ack": @"quit"}); finish(); return; }
          NSString *label = [command isEqualToString:@"count\n"] ? @"7" :
                            [command isEqualToString:@"indicator\n"] ? @"!" : nil;
          NSApp.dockTile.badgeLabel = label;
          [NSApp.dockTile display];
          reply(@{@"ack": [command stringByTrimmingCharactersInSet:NSCharacterSet.newlineCharacterSet]});
        });
        if ([command isEqualToString:@"quit\n"]) return;
      }
      dispatch_async(dispatch_get_main_queue(), ^{ finish(); });
    }
  });
}
@end
int main(void) {
  @autoreleasepool {
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    BadgeFixture *delegate = [BadgeFixture new];
    NSApp.delegate = delegate;
    [NSApp run];
  }
  return 0;
}
