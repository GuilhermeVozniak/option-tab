//go:build darwin

#import "darwin_widget_package.h"
#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <errno.h>
#include <fcntl.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <unistd.h>

#ifndef OT_WIDGET_PACKAGE_PANEL
#define OT_WIDGET_PACKAGE_PANEL() [NSOpenPanel openPanel]
#endif
#ifndef OT_WIDGET_PACKAGE_MAIN
#define OT_WIDGET_PACKAGE_MAIN(...)                                            \
  dispatch_async(dispatch_get_main_queue(), (__VA_ARGS__))
#endif
#ifndef OT_WIDGET_PACKAGE_WORK
#define OT_WIDGET_PACKAGE_WORK(...)                                            \
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0),       \
                 (__VA_ARGS__))
#endif
#ifndef OT_WIDGET_PACKAGE_CURRENT
extern int goWidgetPackageContextCurrent(uintptr_t);
#define OT_WIDGET_PACKAGE_CURRENT(token) goWidgetPackageContextCurrent(token)
#endif
#ifndef OT_WIDGET_PACKAGE_BEFORE_FINAL
#define OT_WIDGET_PACKAGE_BEFORE_FINAL(url) ((void)0)
#endif
#ifndef OT_WIDGET_PACKAGE_SCOPE_START
#define OT_WIDGET_PACKAGE_SCOPE_START(url)                                     \
  [url startAccessingSecurityScopedResource]
#define OT_WIDGET_PACKAGE_SCOPE_STOP(url)                                      \
  [url stopAccessingSecurityScopedResource]
#endif
@interface OTWidgetPackageChooser : NSObject
@property NSLock *lock;
@property NSOpenPanel *panel;
@property BOOL cancelled;
@property uintptr_t admission;
@property NSString *result;
@end
@implementation OTWidgetPackageChooser
@end
static NSDictionary *widgetPackageError(NSString *code) {
  return @{@"error" : code};
}
static BOOL widgetPackageCancelled(OTWidgetPackageChooser *g) {
  [g.lock lock];
  BOOL cancelled = g.cancelled;
  [g.lock unlock];
  return cancelled || !OT_WIDGET_PACKAGE_CURRENT(g.admission);
}
static void widgetPackageFinish(OTWidgetPackageChooser *g,
                                NSDictionary *value) {
  NSData *data = [NSJSONSerialization dataWithJSONObject:value
                                                 options:0
                                                   error:nil];
  NSString *result = data ? [[NSString alloc] initWithData:data
                                                  encoding:NSUTF8StringEncoding]
                          : @"{\"error\":\"ioFailure\"}";
  [g.lock lock];
  if (!g.result)
    g.result = result;
  [g.lock unlock];
}
static BOOL widgetPackageSame(struct stat a, struct stat b) {
  return a.st_dev == b.st_dev && a.st_ino == b.st_ino &&
         a.st_mode == b.st_mode && a.st_size == b.st_size &&
         a.st_mtimespec.tv_sec == b.st_mtimespec.tv_sec &&
         a.st_mtimespec.tv_nsec == b.st_mtimespec.tv_nsec &&
         a.st_ctimespec.tv_sec == b.st_ctimespec.tv_sec &&
         a.st_ctimespec.tv_nsec == b.st_ctimespec.tv_nsec;
}
// Snapshot only: no bookmarks or retained access grants. File bytes remain on
// the selected descriptor; pathname replacement never retargets the read.
static NSDictionary *widgetPackageRead(OTWidgetPackageChooser *g, NSURL *url) {
  @autoreleasepool {
    if (widgetPackageCancelled(g))
      return widgetPackageError(@"cancelled");
    if (!url.isFileURL)
      return widgetPackageError(@"invalidFile");
    NSString *path = url.URLByStandardizingPath.path;
    NSString *ext = path.pathExtension.lowercaseString;
    if (![@[ @"zip", @"otwidget" ] containsObject:ext])
      return widgetPackageError(@"invalidFile");
    BOOL scoped = OT_WIDGET_PACKAGE_SCOPE_START(url);
    int fd = -1;
    @try {
      struct stat selected, before, after, named;
      struct statfs fs;
      if (lstat(path.fileSystemRepresentation, &selected) ||
          !S_ISREG(selected.st_mode) || (selected.st_mode & 0111))
        return widgetPackageError(@"invalidFile");
      fd = open(path.fileSystemRepresentation,
                O_RDONLY | O_CLOEXEC | O_NOFOLLOW | O_NONBLOCK);
      if (fd < 0 || fstat(fd, &before) ||
          !widgetPackageSame(selected, before) || !S_ISREG(before.st_mode) ||
          (before.st_mode & 0111))
        return widgetPackageError(@"changed");
      if (fstatfs(fd, &fs) || !(fs.f_flags & MNT_LOCAL))
        return widgetPackageError(@"invalidFile");
      if (before.st_size < 0 || before.st_size > 4 * 1024 * 1024)
        return widgetPackageError(@"tooLarge");
      NSMutableData *data =
          [NSMutableData dataWithLength:(NSUInteger)before.st_size];
      size_t offset = 0;
      while (offset < data.length) {
        if (widgetPackageCancelled(g))
          return widgetPackageError(@"cancelled");
        ssize_t count = read(fd, (char *)data.mutableBytes + offset,
                             MIN((size_t)16384, data.length - offset));
        if (count < 0 && errno == EINTR)
          continue;
        if (count <= 0)
          return widgetPackageError(@"changed");
        offset += (size_t)count;
      }
      OT_WIDGET_PACKAGE_BEFORE_FINAL(url);
      if (widgetPackageCancelled(g))
        return widgetPackageError(@"cancelled");
      if (fstat(fd, &after) || lstat(path.fileSystemRepresentation, &named) ||
          !widgetPackageSame(before, after) ||
          !widgetPackageSame(before, named))
        return widgetPackageError(@"changed");
      return @{
        @"name" : path.lastPathComponent,
        @"archive" : [data base64EncodedStringWithOptions:0]
      };
    } @finally {
      if (fd >= 0)
        close(fd);
      if (scoped)
        OT_WIDGET_PACKAGE_SCOPE_STOP(url);
    }
  }
}
int ot_widget_package_main_thread(void) {
  return NSThread.isMainThread ? 1 : 0;
}
void *ot_widget_package_start(uintptr_t admission) {
  if (!admission)
    return NULL;
  OTWidgetPackageChooser *g = [OTWidgetPackageChooser new];
  g.lock = [NSLock new];
  g.admission = admission;
  void *handle = (__bridge_retained void *)g;
  OT_WIDGET_PACKAGE_MAIN(^{
    if (widgetPackageCancelled(g)) {
      widgetPackageFinish(g, widgetPackageError(@"cancelled"));
      return;
    }
    NSOpenPanel *panel = OT_WIDGET_PACKAGE_PANEL();
    if (!panel) {
      widgetPackageFinish(g, widgetPackageError(@"unavailable"));
      return;
    }
    g.panel = panel;
    panel.canChooseFiles = YES;
    panel.canChooseDirectories = NO;
    panel.allowsMultipleSelection = NO;
    panel.canCreateDirectories = NO;
    panel.resolvesAliases = NO;
    panel.treatsFilePackagesAsDirectories = NO;
    // Public UTType produces a dynamic extension type without registration.
    UTType *widgetType = [UTType typeWithFilenameExtension:@"otwidget"
                                          conformingToType:UTTypeData];
    if (!widgetType) {
      g.panel = nil;
      widgetPackageFinish(g, widgetPackageError(@"unavailable"));
      return;
    }
    panel.allowedContentTypes = @[ UTTypeZIP, widgetType ];
    panel.prompt = @"Review Widget Package";
    if (widgetPackageCancelled(g)) {
      g.panel = nil;
      widgetPackageFinish(g, widgetPackageError(@"cancelled"));
      return;
    }
    [panel beginWithCompletionHandler:^(NSModalResponse response) {
      NSURL *selected = panel.URL;
      g.panel = nil;
      if (widgetPackageCancelled(g) || response != NSModalResponseOK ||
          !selected) {
        widgetPackageFinish(g, widgetPackageError(@"cancelled"));
        return;
      }
      OT_WIDGET_PACKAGE_WORK(^{
        NSDictionary *result = widgetPackageRead(g, selected);
        if (widgetPackageCancelled(g))
          result = widgetPackageError(@"cancelled");
        // Publish only after the file descriptor and temporary security scope
        // join.
        widgetPackageFinish(g, result);
      });
    }];
  });
  return handle;
}
void ot_widget_package_cancel(void *value) {
  OTWidgetPackageChooser *g = (__bridge OTWidgetPackageChooser *)value;
  [g.lock lock];
  g.cancelled = YES;
  [g.lock unlock];
  OT_WIDGET_PACKAGE_MAIN(^{
    [g.panel cancel:nil];
  });
}
char *ot_widget_package_poll(void *value) {
  OTWidgetPackageChooser *g = (__bridge OTWidgetPackageChooser *)value;
  [g.lock lock];
  char *result = g.result ? strdup(g.result.UTF8String) : NULL;
  [g.lock unlock];
  return result;
}
void ot_widget_package_release(void *value) {
  if (value)
    CFBridgingRelease(value);
}
