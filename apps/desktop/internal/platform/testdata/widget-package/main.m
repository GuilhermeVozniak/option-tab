#import <Cocoa/Cocoa.h>
#include <sys/stat.h>
#include <unistd.h>
static BOOL current = YES;
static int made = 0, scopes = 0;
static NSMutableArray *mainQueue;
static void (^pending)(NSModalResponse);
static NSURL *selected;
static int mutate = 0;
static void beforeFinal(NSURL *url) {
  if (mutate == 1) {
    unlink(url.fileSystemRepresentation);
    [@"replacement" writeToURL:url
                    atomically:NO
                      encoding:NSUTF8StringEncoding
                         error:nil];
  }
  if (mutate == 2)
    current = NO;
}
@interface FakePackagePanel : NSObject
@property BOOL canChooseFiles, canChooseDirectories, allowsMultipleSelection,
    canCreateDirectories, resolvesAliases, treatsFilePackagesAsDirectories;
@property NSArray *allowedContentTypes;
@property NSString *prompt;
@property(readonly) NSURL *URL;
@end
@implementation FakePackagePanel
- (NSURL *)URL {
  return selected;
}
- (void)beginWithCompletionHandler:(void (^)(NSModalResponse))reply {
  pending = [reply copy];
}
- (void)cancel:(id)sender {
  if (pending) {
    void (^reply)(NSModalResponse) = pending;
    pending = nil;
    reply(NSModalResponseCancel);
  }
}
@end
static id makePanel(void) {
  made++;
  return [FakePackagePanel new];
}
#define OT_WIDGET_PACKAGE_PANEL() makePanel()
#define OT_WIDGET_PACKAGE_MAIN(...) [mainQueue addObject:[(__VA_ARGS__) copy]]
#define OT_WIDGET_PACKAGE_WORK(...) (__VA_ARGS__)()
#define OT_WIDGET_PACKAGE_CURRENT(token) current
#define OT_WIDGET_PACKAGE_BEFORE_FINAL(url) beforeFinal(url)
#define OT_WIDGET_PACKAGE_SCOPE_START(url) (scopes++, YES)
#define OT_WIDGET_PACKAGE_SCOPE_STOP(url) (scopes--)
#include "../../darwin_widget_package.m"
static void require(BOOL v, NSString *why) {
  if (!v) {
    fprintf(stderr, "%s\n", why.UTF8String);
    exit(1);
  }
}
static void drain(void) {
  while (mainQueue.count) {
    void (^work)(void) = mainQueue[0];
    [mainQueue removeObjectAtIndex:0];
    work();
  }
}
static NSDictionary *result(void *owner) {
  char *raw = ot_widget_package_poll(owner);
  require(raw != NULL, @"missing joined result");
  NSData *d = [NSData dataWithBytes:raw length:strlen(raw)];
  free(raw);
  return [NSJSONSerialization JSONObjectWithData:d options:0 error:nil];
}
static NSDictionary *readFile(NSURL *url) {
  OTWidgetPackageChooser *g = [OTWidgetPackageChooser new];
  g.lock = [NSLock new];
  g.admission = 1;
  return widgetPackageRead(g, url);
}
int main(int argc, const char **argv) {
  @autoreleasepool {
    require(argc == 2, @"fixture directory required");
    mainQueue = [NSMutableArray new];
    NSURL *dir = [NSURL fileURLWithPath:@(argv[1]) isDirectory:YES];
    selected = [dir URLByAppendingPathComponent:@"sample.otwidget"];
    NSData *bytes =
        [@"disposable archive snapshot" dataUsingEncoding:NSUTF8StringEncoding];
    require([bytes writeToURL:selected atomically:NO], @"fixture write");
    chmod(selected.fileSystemRepresentation, 0600);
    NSDictionary *good = readFile(selected);
    require([good[@"name"] isEqual:@"sample.otwidget"] &&
                [[[NSData alloc] initWithBase64EncodedString:good[@"archive"]
                                                     options:0] isEqual:bytes],
            @"exact snapshot failed");
    require(scopes == 0, @"scope leaked");
    chmod(selected.fileSystemRepresentation, 0700);
    require(readFile(selected)[@"error"] != nil, @"executable accepted");
    chmod(selected.fileSystemRepresentation, 0600);
    NSURL *link = [dir URLByAppendingPathComponent:@"link.zip"];
    symlink(selected.fileSystemRepresentation, link.fileSystemRepresentation);
    require(readFile(link)[@"error"] != nil, @"symlink accepted");
    require(readFile(dir)[@"error"] != nil, @"directory accepted");
    require(readFile([NSURL
                URLWithString:@"https://example.invalid/file.zip"])[@"error"] !=
                nil,
            @"remote URL accepted");
    int fd = open(selected.fileSystemRepresentation, O_WRONLY);
    require(fd >= 0 && ftruncate(fd, 4 * 1024 * 1024 + 1) == 0,
            @"oversize fixture");
    close(fd);
    require([readFile(selected)[@"error"] isEqual:@"tooLarge"],
            @"oversize accepted");
    [bytes writeToURL:selected atomically:NO];
    mutate = 1;
    require([readFile(selected)[@"error"] isEqual:@"changed"],
            @"replacement accepted");
    mutate = 0;
    mutate = 2;
    require([readFile(selected)[@"error"] isEqual:@"cancelled"],
            @"cancelled read accepted");
    mutate = 0;
    current = YES;
    // Queued context cancellation is visible without a polling/native-cancel
    // call.
    void *g = ot_widget_package_start(1);
    current = NO;
    drain();
    require(made == 0 && [result(g)[@"error"] isEqual:@"cancelled"],
            @"queued cancellation created panel");
    ot_widget_package_release(g);
    current = YES;
    g = ot_widget_package_start(1);
    drain();
    require(made == 1 && pending != nil, @"substitute panel not presented");
    ot_widget_package_cancel(g);
    drain();
    require([result(g)[@"error"] isEqual:@"cancelled"],
            @"active cancel failed");
    ot_widget_package_release(g);
    g = ot_widget_package_start(1);
    drain();
    void (^reply)(NSModalResponse) = pending;
    pending = nil;
    reply(NSModalResponseOK);
    require(result(g)[@"archive"] != nil, @"approved disposable read failed");
    ot_widget_package_release(g);
    require(scopes == 0 && mainQueue.count == 0 && pending == nil,
            @"owner/scope cleanup incomplete");
    puts("PASS: injected chooser queued/active cancel, exact local snapshot, "
         "replacement, metadata and size refusal; no visible UI");
  }
}
