// Test-only AppKit host. Never presents a panel or opens a default application.
#import "../../darwin_folder.h"
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#import <stdatomic.h>
#import <sys/stat.h>

static NSURL *chosen;
static NSModalResponse choice;
static NSMutableArray<NSString *> *opened;
static NSString *replaceDuringGuard;
static atomic_int guardAllowed, guardCalls;
int goFolderFinalGuard(uintptr_t token) {
  atomic_fetch_add(&guardCalls, 1);
  if (replaceDuringGuard) {
    NSString *path = replaceDuringGuard;
    replaceDuringGuard = nil;
    [[NSFileManager defaultManager]
        moveItemAtPath:path
                toPath:[path stringByAppendingString:@".old"]
                 error:nil];
    [@"replacement" writeToFile:path
                     atomically:YES
                       encoding:NSUTF8StringEncoding
                          error:nil];
  }
  return token == 71 && atomic_load(&guardAllowed);
}
@interface FixturePanel : NSObject
@property BOOL canChooseFiles, canChooseDirectories, allowsMultipleSelection,
    canCreateDirectories;
@property NSURL *directoryURL;
@property NSString *prompt;
@property(copy) void (^completion)(NSModalResponse);
- (NSURL *)URL;
- (void)beginWithCompletionHandler:(void (^)(NSModalResponse))callback;
- (void)cancel:(id)sender;
@end
@implementation FixturePanel
- (NSURL *)URL {
  return chosen;
}
- (void)beginWithCompletionHandler:(void (^)(NSModalResponse))callback {
  self.completion = callback;
  dispatch_async(dispatch_get_main_queue(), ^{
    if (self.completion) {
      self.completion(choice);
      self.completion = nil;
    }
  });
}
- (void)cancel:(id)sender {
  if (self.completion) {
    self.completion(NSModalResponseCancel);
    self.completion = nil;
  }
}
@end
static id fixturePanel(id self, SEL cmd) { return [FixturePanel new]; }
static BOOL fixtureOpen(id self, SEL cmd, NSURL *url) {
  NSCAssert(NSThread.isMainThread, @"workspace dispatch must execute on main");
  [opened addObject:url.path];
  return YES;
}
static NSDictionary *grant(const char *path, BOOL cancel) {
  void *g = ot_folder_grant_start(path);
  if (!g)
    return nil;
  if (cancel)
    ot_folder_grant_cancel(g);
  double deadline = NSDate.timeIntervalSinceReferenceDate + 2;
  char *text = NULL;
  while (!text && NSDate.timeIntervalSinceReferenceDate < deadline) {
    text = ot_folder_grant_poll(g);
    if (!text)
      [NSThread sleepForTimeInterval:.005];
  }
  NSDictionary *result =
      text ? [NSJSONSerialization
                 JSONObjectWithData:[NSData dataWithBytes:text
                                                   length:strlen(text)]
                            options:0
                              error:nil]
           : nil;
  free(text);
  ot_folder_grant_release(g);
  return result;
}
#define CHECK(x, message)                                                      \
  do {                                                                         \
    if (!(x)) {                                                                \
      fprintf(stderr, "FAIL %s\n", message);                                   \
      return 1;                                                                \
    }                                                                          \
  } while (0)
static int exercise(NSString *path) {
  const char *root = path.fileSystemRepresentation;
  choice = NSModalResponseCancel;
  chosen = [NSURL fileURLWithPath:path];
  NSDictionary *cancelled = grant(root, NO);
  CHECK([cancelled[@"Status"] isEqual:@"permissionRequired"],
        "cancel callback");
  NSDictionary *preCancelled = grant(root, YES);
  CHECK([preCancelled[@"Status"] isEqual:@"permissionRequired"],
        "request cancellation");
  choice = NSModalResponseOK;
  chosen = [NSURL
      fileURLWithPath:[path stringByAppendingPathComponent:@"subfolder"]];
  CHECK([grant(root, NO)[@"Status"] isEqual:@"permissionRequired"],
        "wrong chosen folder");
  chosen = [NSURL fileURLWithPath:path];
  NSDictionary *granted = grant(root, NO);
  CHECK([granted[@"Status"] isEqual:@"ready"], "fixture bookmark grant");
  NSData *data =
      [[NSData alloc] initWithBase64EncodedString:granted[@"Bookmark"]
                                          options:0];
  CHECK(data.length > 0, "bookmark bytes");
  char *message = NULL, *refresh = NULL;
  void *scope =
      ot_folder_begin(root, data.bytes, (int)data.length, &message, &refresh);
  if (!scope)
    fprintf(stderr, "scope refused %s\n", message ?: "unknown");
  free(message);
  free(refresh);
  CHECK(scope, "bookmark scope reuse");
  struct stat parent;
  CHECK(lstat(root, &parent) == 0, "parent stat");
  atomic_store(&guardAllowed, 1);
  for (NSString *name in @[ @"fixture.txt", @"subfolder" ]) {
    NSString *child = [path stringByAppendingPathComponent:name];
    struct stat st;
    CHECK(lstat(child.fileSystemRepresentation, &st) == 0, "child stat");
    int code =
        ot_folder_open(scope, child.fileSystemRepresentation, parent.st_dev,
                       parent.st_ino, st.st_dev, st.st_ino, 71);
    NSLog(
        @"dispatch code %d child %@ guards %d bookmarkURL %@", code, child,
        atomic_load(&guardCalls),
        [NSURL
            URLByResolvingBookmarkData:data
                               options:NSURLBookmarkResolutionWithoutUI |
                                       NSURLBookmarkResolutionWithSecurityScope
                         relativeToURL:nil
                   bookmarkDataIsStale:NULL
                                 error:nil]);
    CHECK(code == 0, "production dispatch");
    CHECK([opened.lastObject isEqual:child], "exact revalidated URL");
    NSUInteger count = opened.count;
    atomic_store(&guardAllowed, 0);
    CHECK(ot_folder_open(scope, child.fileSystemRepresentation, parent.st_dev,
                         parent.st_ino, st.st_dev, st.st_ino, 71) == -2 &&
              opened.count == count,
          "final cancellation refuses dispatch");
    atomic_store(&guardAllowed, 1);
    CHECK(ot_folder_open(scope, child.fileSystemRepresentation, parent.st_dev,
                         parent.st_ino, st.st_dev, st.st_ino + 1, 71) != 0 &&
              opened.count == count,
          "replaced entry refuses dispatch");
  }
  NSString *replace = [path stringByAppendingPathComponent:@"fixture.txt"];
  struct stat before;
  CHECK(lstat(replace.fileSystemRepresentation, &before) == 0,
        "replacement stat");
  replaceDuringGuard = replace;
  CHECK(ot_folder_open(scope, replace.fileSystemRepresentation, parent.st_dev,
                       parent.st_ino, before.st_dev, before.st_ino, 71) != 0 &&
            opened.count == 2,
        "replacement during external guard refused");
  CHECK(opened.count == 2, "only two exact fixture dispatches");
  ot_folder_end(scope);
  printf("PASS injected chooser cancel/pre-cancel/wrong/grant; real bookmark "
         "reuse; exact production main-queue file+folder dispatch; final-guard "
         "cancellation; identity refusal; dispatches=%lu guards=%d\n",
         (unsigned long)opened.count, atomic_load(&guardCalls));
  return 0;
}
int main(int argc, const char **argv) {
  @autoreleasepool {
    if (argc != 2)
      return 2;
    NSString *path = [NSString stringWithUTF8String:argv[1]];
    if (![path.lastPathComponent hasPrefix:@"option-tab-folder-smoke-"])
      return 2;
    pid_t foreground =
        NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
    [NSApplication sharedApplication];
    [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
    opened = [NSMutableArray array];
    atomic_init(&guardAllowed, 0);
    atomic_init(&guardCalls, 0);
    Method panel =
        class_getClassMethod(NSOpenPanel.class, @selector(openPanel));
    Method workspace =
        class_getInstanceMethod(NSWorkspace.class, @selector(openURL:));
    IMP previousPanel = method_setImplementation(panel, (IMP)fixturePanel);
    IMP previousOpen = method_setImplementation(workspace, (IMP)fixtureOpen);
    __block atomic_bool done;
    atomic_init(&done, false);
    __block int result = 2;
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
      @autoreleasepool {
        result = exercise(path);
        atomic_store(&done, true);
      }
    });
    double deadline = NSDate.timeIntervalSinceReferenceDate + 10;
    while (!atomic_load(&done) &&
           NSDate.timeIntervalSinceReferenceDate < deadline)
      CFRunLoopRunInMode(kCFRunLoopDefaultMode, .01, true);
    if (!atomic_load(&done)) {
      fprintf(stderr, "FAIL native host deadline\n");
      return 2;
    }
    method_setImplementation(panel, previousPanel);
    method_setImplementation(workspace, previousOpen);
    pid_t after =
        NSWorkspace.sharedWorkspace.frontmostApplication.processIdentifier;
    printf("FOREGROUND before=%d after=%d; host completed, no real chooser or "
           "open\n",
           foreground, after);
    if (foreground != after)
      return 1;
    return result;
  }
}
