//go:build darwin

#import "darwin_launcher_items.h"
#import "darwin_launcher.h"
#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <fcntl.h>
#include <libproc.h>
#include <sys/mount.h>
#include <sys/stat.h>
#include <unistd.h>
#ifndef OT_ITEM_CURRENT
extern int goLauncherItemCurrent(uintptr_t);
#define OT_ITEM_CURRENT(t) goLauncherItemCurrent(t)
#endif
#ifndef OT_ITEM_PANEL_CURRENT
#define OT_ITEM_PANEL_CURRENT(t, d) ot_launcher_panel_validate(t, d)
#endif
#ifndef OT_ITEM_PANEL
#define OT_ITEM_PANEL() [NSOpenPanel openPanel]
#endif
#ifndef OT_ITEM_MAIN
#define OT_ITEM_MAIN(block) dispatch_async(dispatch_get_main_queue(), block)
#endif
#ifndef OT_ITEM_OPEN_APP
#define OT_ITEM_OPEN_APP(url, config, done)                                    \
  [NSWorkspace.sharedWorkspace openApplicationAtURL:url                        \
                                      configuration:config                     \
                                  completionHandler:done]
#define OT_ITEM_OPEN_URL(url, config, done)                                    \
  [NSWorkspace.sharedWorkspace openURL:url                                     \
                         configuration:config                                  \
                     completionHandler:done]
#endif
#ifndef OT_ITEM_WORK
#define OT_ITEM_WORK(block)                                                    \
  dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), block)
#endif
#ifndef OT_ITEM_PROCESS_APP
#define OT_ITEM_PROCESS_APP(pid)                                               \
  [NSRunningApplication runningApplicationWithProcessIdentifier:pid]
#endif
#ifndef OT_ITEM_RELAUNCH_SECONDS
#define OT_ITEM_RELAUNCH_SECONDS 10
#endif
#ifndef OT_ITEM_BEFORE_GUARD
#define OT_ITEM_BEFORE_GUARD() ((void)0)
#endif
@interface OTLauncherItemScope : NSObject
@property NSURL *url;
@property NSString *kind, *bundle;
@property BOOL scoped;
@property struct stat identity;
@end
@implementation OTLauncherItemScope
- (void)dealloc {
  if (_scoped)
    [_url stopAccessingSecurityScopedResource];
}
@end
@interface OTLauncherItemJob : NSObject
@property NSLock *lock;
@property NSOpenPanel *panel;
@property NSString *result;
@property uintptr_t guard;
@property BOOL cancelled;
@end
@implementation OTLauncherItemJob
@end
static BOOL itemCurrent(OTLauncherItemJob *job) {
  [job.lock lock];
  BOOL cancelled = job.cancelled;
  [job.lock unlock];
  return !cancelled && OT_ITEM_CURRENT(job.guard);
}
static void itemFinish(OTLauncherItemJob *job, NSDictionary *reply) {
  NSData *data = [NSJSONSerialization dataWithJSONObject:reply
                                                 options:0
                                                   error:nil];
  NSString *text = data ? [[NSString alloc] initWithData:data
                                                encoding:NSUTF8StringEncoding]
                        : @"{\"error\":\"unavailable\"}";
  [job.lock lock];
  if (!job.result)
    job.result = text;
  [job.lock unlock];
}
static BOOL itemStatSame(struct stat a, struct stat b) {
  return a.st_dev == b.st_dev && a.st_ino == b.st_ino &&
         a.st_mode == b.st_mode && a.st_size == b.st_size &&
         a.st_mtimespec.tv_sec == b.st_mtimespec.tv_sec &&
         a.st_mtimespec.tv_nsec == b.st_mtimespec.tv_nsec &&
         a.st_ctimespec.tv_sec == b.st_ctimespec.tv_sec &&
         a.st_ctimespec.tv_nsec == b.st_ctimespec.tv_nsec;
}
static BOOL itemKind(NSURL *url, NSString *kind, NSString **bundle) {
  if (!url.isFileURL ||
      ![url.path
          isEqual:url.URLByStandardizingPath.URLByResolvingSymlinksInPath.path])
    return NO;
  struct stat st;
  struct statfs fs;
  if (lstat(url.fileSystemRepresentation, &st) ||
      statfs(url.fileSystemRepresentation, &fs) || !(fs.f_flags & MNT_LOCAL))
    return NO;
  if ([kind isEqual:@"app"]) {
    NSBundle *app = [NSBundle bundleWithURL:url];
    NSString *id = app.bundleIdentifier;
    if (!S_ISDIR(st.st_mode) ||
        ![url.pathExtension.lowercaseString isEqual:@"app"] || !id.length)
      return NO;
    if (bundle)
      *bundle = id;
    return YES;
  }
  if ([kind isEqual:@"folder"]) {
    NSNumber *package = nil;
    [url getResourceValue:&package forKey:NSURLIsPackageKey error:nil];
    return S_ISDIR(st.st_mode) && package && !package.boolValue;
  }
  if (!S_ISREG(st.st_mode) || (st.st_mode & 0111))
    return NO;
  UTType *type = nil;
  [url getResourceValue:&type forKey:NSURLContentTypeKey error:nil];
  if ([kind isEqual:@"icon"])
    return [url.pathExtension.lowercaseString isEqual:@"png"];
  return [kind isEqual:@"file"] && type &&
         ![type conformsToType:UTTypeExecutable] &&
         ![type conformsToType:UTTypeScript] &&
         ![type conformsToType:UTTypeApplication];
}
static NSString *itemLabel(NSURL *url, NSString *kind) {
  if (![kind isEqual:@"app"])
    return url.lastPathComponent;
  NSBundle *bundle = [NSBundle bundleWithURL:url];
  for (NSString *key in @[ @"CFBundleDisplayName", @"CFBundleName" ]) {
    id value =
        bundle.localizedInfoDictionary[key] ?: bundle.infoDictionary[key];
    if ([value isKindOfClass:NSString.class] && [value length])
      return value;
  }
  return url.URLByDeletingPathExtension.lastPathComponent;
}
static NSDictionary *itemSelect(NSURL *url, NSString *kind, uintptr_t guard) {
  if (!OT_ITEM_CURRENT(guard))
    return @{@"error" : @"cancelled"};
  BOOL scoped = [url startAccessingSecurityScopedResource];
  @try {
    NSString *bundle = @"";
    if (!itemKind(url, kind, &bundle))
      return @{@"error" : @"unsupported"};
    struct stat before, after;
    if (lstat(url.fileSystemRepresentation, &before))
      return @{@"error" : @"missing"};
    if ([kind isEqual:@"icon"]) {
      int fd = open(url.fileSystemRepresentation,
                    O_RDONLY | O_CLOEXEC | O_NOFOLLOW | O_NONBLOCK);
      if (fd < 0)
        return @{@"error" : @"accessRequired"};
      @try {
        struct stat opened;
        if (fstat(fd, &opened) || !itemStatSame(before, opened) ||
            opened.st_size < 1 || opened.st_size > 2 * 1024 * 1024)
          return @{@"error" : @"tooLarge"};
        NSMutableData *data = [NSMutableData dataWithLength:opened.st_size];
        NSUInteger at = 0;
        while (at < data.length) {
          if (!OT_ITEM_CURRENT(guard))
            return @{@"error" : @"cancelled"};
          ssize_t n = read(fd, (char *)data.mutableBytes + at,
                           MIN((NSUInteger)16384, data.length - at));
          if (n < 0 && errno == EINTR)
            continue;
          if (n <= 0)
            return @{@"error" : @"changed"};
          at += n;
        }
        if (fstat(fd, &after) || !itemStatSame(before, after) ||
            lstat(url.fileSystemRepresentation, &after) ||
            !itemStatSame(before, after))
          return @{@"error" : @"changed"};
        return @{@"archive" : [data base64EncodedStringWithOptions:0]};
      } @finally {
        close(fd);
      }
    }
    NSData *bookmark =
        [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
            includingResourceValuesForKeys:nil
                             relativeToURL:nil
                                     error:nil];
    if (!bookmark || bookmark.length > 1024 * 1024)
      return @{@"error" : @"unavailable"};
    if (!OT_ITEM_CURRENT(guard))
      return @{@"error" : @"cancelled"};
    if (lstat(url.fileSystemRepresentation, &after) ||
        !itemStatSame(before, after))
      return @{@"error" : @"changed"};
    return @{
      @"kind" : kind,
      @"label" : itemLabel(url, kind),
      @"selectedPath" : url.path,
      @"bundleID" : bundle,
      @"bookmark" : [bookmark base64EncodedStringWithOptions:0]
    };
  } @finally {
    if (scoped)
      [url stopAccessingSecurityScopedResource];
  }
}
static OTLauncherItemScope *itemResolve(NSDictionary *record) {
  NSData *bytes =
      [[NSData alloc] initWithBase64EncodedString:record[@"bookmark"]
                                          options:0];
  if (!bytes.length || bytes.length > 1024 * 1024)
    return nil;
  BOOL stale = NO;
  NSURL *url =
      [NSURL URLByResolvingBookmarkData:bytes
                                options:NSURLBookmarkResolutionWithoutUI |
                                        NSURLBookmarkResolutionWithoutMounting |
                                        NSURLBookmarkResolutionWithSecurityScope
                          relativeToURL:nil
                    bookmarkDataIsStale:&stale
                                  error:nil];
  if (!url)
    return nil;
  url = url.URLByStandardizingPath.URLByResolvingSymlinksInPath;
  // A stale bookmark may have fallen back to a replacement at the old path.
  // Never silently refresh that authority. Moved URLs are reported disabled by
  // the outer resolver, which compares the selected canonical location.
  if (stale && [url.path isEqual:record[@"selectedPath"]])
    return nil;
  OTLauncherItemScope *s = [OTLauncherItemScope new];
  s.url = url;
  s.scoped = [url startAccessingSecurityScopedResource];
  s.kind = record[@"kind"];
  s.bundle = record[@"bundleID"] ?: @"";
  NSString *bundle = @"";
  if (!itemKind(url, s.kind, &bundle) || ![bundle isEqual:s.bundle])
    return nil;
  struct stat st;
  if (lstat(url.fileSystemRepresentation, &st))
    return nil;
  s.identity = st;
  return s;
}
static BOOL itemSame(OTLauncherItemScope *s) {
  if (!s)
    return NO;
  struct stat st;
  NSString *bundle = @"";
  return itemKind(s.url, s.kind, &bundle) && [bundle isEqual:s.bundle] &&
         !lstat(s.url.fileSystemRepresentation, &st) &&
         itemStatSame(s.identity, st);
}
int ot_launcher_item_same(void *p) {
  @autoreleasepool {
    return itemSame((__bridge OTLauncherItemScope *)p);
  }
}
void *ot_launcher_item_resolve(const char *raw, char **error) {
  @autoreleasepool {
    NSData *data = [[NSString stringWithUTF8String:raw]
        dataUsingEncoding:NSUTF8StringEncoding];
    NSDictionary *record = [NSJSONSerialization JSONObjectWithData:data
                                                           options:0
                                                             error:nil];
    OTLauncherItemScope *s = itemResolve(record);
    if (!s) {
      struct stat st;
      NSString *path = record[@"selectedPath"];
      int result = path ? lstat(path.fileSystemRepresentation, &st) : -1;
      const char *code = result < 0 ? (errno == ENOENT   ? "missing"
                                       : errno == EACCES ? "accessRequired"
                                                         : "unavailable")
                                    : "changed";
      *error = strdup(code);
      return NULL;
    }
    if (![s.url.path isEqual:record[@"selectedPath"]]) {
      *error = strdup("moved");
      return NULL;
    }
    return (__bridge_retained void *)s;
  }
}
#ifndef OT_ITEM_RUNNING
#define OT_ITEM_RUNNING(bundle)                                                \
  [NSRunningApplication runningApplicationsWithBundleIdentifier:bundle]
#endif
#ifndef OT_ITEM_PROCESS_INFO
#define OT_ITEM_PROCESS_INFO(pid, info)                                        \
  (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, info, sizeof(*(info))) ==            \
   sizeof(*(info)))
#endif
static NSArray *itemMatchingApps(OTLauncherItemScope *s, NSArray *apps) {
  if (!apps || apps.count > 128)
    return nil;
  NSMutableArray *matches = [NSMutableArray new];
  for (NSRunningApplication *app in apps) {
    if (!app.terminated && [app.bundleIdentifier isEqual:s.bundle] &&
        [app.bundleURL.URLByStandardizingPath.URLByResolvingSymlinksInPath.path
            isEqual:s.url.path])
      [matches addObject:app];
  }
  return matches;
}
static NSDictionary *itemRunning(OTLauncherItemScope *s) {
  if (![s.kind isEqual:@"app"])
    return @{};
  if (!itemSame(s))
    return @{@"state" : @"unavailable"};
  NSArray *first = itemMatchingApps(s, OT_ITEM_RUNNING(s.bundle));
  if (!first)
    return @{@"state" : @"unavailable"};
  if (first.count > 1)
    return @{@"state" : @"ambiguous"};
  NSRunningApplication *app = first.firstObject;
  struct proc_bsdinfo before = {0}, after = {0};
  if (app && (app.processIdentifier <= 0 ||
              !OT_ITEM_PROCESS_INFO(app.processIdentifier, &before) ||
              !before.pbi_start_tvsec))
    return @{@"state" : @"unavailable"};
  NSArray *second = itemMatchingApps(s, OT_ITEM_RUNNING(s.bundle));
  if (!second || second.count != first.count || !itemSame(s))
    return @{@"state" : @"unavailable"};
  if (!second.count)
    return @{@"state" : @"stopped"};
  NSRunningApplication *current = second.firstObject;
  if (current.processIdentifier != app.processIdentifier ||
      !OT_ITEM_PROCESS_INFO(current.processIdentifier, &after) ||
      before.pbi_start_tvsec != after.pbi_start_tvsec ||
      before.pbi_start_tvusec != after.pbi_start_tvusec || current.terminated)
    return @{@"state" : @"unavailable"};
  return @{
    @"state" : @"running",
    @"pid" : @(current.processIdentifier),
    @"startSeconds" : @(after.pbi_start_tvsec),
    @"startMicros" : @(after.pbi_start_tvusec)
  };
}
char *ot_launcher_item_scope_info(void *p) {
  @autoreleasepool {
    OTLauncherItemScope *s = (__bridge OTLauncherItemScope *)p;
    NSDictionary *process = itemRunning(s);
    NSString *fingerprint = [NSString
        stringWithFormat:@"%llu:%llu:%u:%lld:%ld:%ld:%ld:%ld",
                         (uint64_t)s.identity.st_dev,
                         (uint64_t)s.identity.st_ino, s.identity.st_mode,
                         s.identity.st_size, s.identity.st_mtimespec.tv_sec,
                         s.identity.st_mtimespec.tv_nsec,
                         s.identity.st_ctimespec.tv_sec,
                         s.identity.st_ctimespec.tv_nsec];
    NSData *d = [NSJSONSerialization dataWithJSONObject:@{
      @"path" : s.url.path,
      @"bundleID" : s.bundle,
      @"fingerprint" : fingerprint,
      @"process" : process,
      @"processState" : process[@"state"] ?: @"",
      @"label" : itemLabel(s.url, s.kind)
    }
                                                options:0
                                                  error:nil];
    return strdup([[NSString alloc] initWithData:d
                                        encoding:NSUTF8StringEncoding]
                      .UTF8String);
  }
}
void ot_launcher_item_scope_release(void *p) {
  if (p)
    CFBridgingRelease(p);
}
int ot_launcher_item_main(void) { return NSThread.isMainThread; }
void *ot_launcher_item_choose(const char *kind, uintptr_t guard) {
  OTLauncherItemJob *job = [OTLauncherItemJob new];
  job.lock = [NSLock new];
  job.guard = guard;
  NSString *type = [NSString stringWithUTF8String:kind];
  OT_ITEM_MAIN(^{
    if (!itemCurrent(job)) {
      itemFinish(job, @{@"error" : @"cancelled"});
      return;
    }
    NSOpenPanel *panel = OT_ITEM_PANEL();
    if (!panel) {
      itemFinish(job, @{@"error" : @"unavailable"});
      return;
    }
    job.panel = panel;
    panel.canChooseFiles = ![type isEqual:@"folder"];
    panel.canChooseDirectories = [type isEqual:@"folder"];
    panel.allowsMultipleSelection = NO;
    panel.canCreateDirectories = NO;
    panel.resolvesAliases = NO;
    panel.treatsFilePackagesAsDirectories = NO;
    if ([type isEqual:@"app"])
      panel.allowedContentTypes = @[ UTTypeApplicationBundle ];
    if ([type isEqual:@"icon"])
      panel.allowedContentTypes = @[ UTTypePNG ];
    if (!itemCurrent(job)) {
      job.panel = nil;
      itemFinish(job, @{@"error" : @"cancelled"});
      return;
    }
    [panel beginWithCompletionHandler:^(NSModalResponse response) {
      NSURL *url = panel.URL;
      job.panel = nil;
      if (response != NSModalResponseOK || !url || !itemCurrent(job)) {
        itemFinish(job, @{@"error" : @"cancelled"});
        return;
      }
      OT_ITEM_WORK(^{
        NSDictionary *r = itemSelect(url, type, guard);
        itemFinish(job, itemCurrent(job) ? r : @{@"error" : @"cancelled"});
      });
    }];
  });
  return (__bridge_retained void *)job;
}
void ot_launcher_item_cancel(void *p) {
  OTLauncherItemJob *job = (__bridge OTLauncherItemJob *)p;
  [job.lock lock];
  job.cancelled = YES;
  [job.lock unlock];
  OT_ITEM_MAIN(^{
    [job.panel cancel:nil];
  });
}
char *ot_launcher_item_poll(void *p) {
  OTLauncherItemJob *job = (__bridge OTLauncherItemJob *)p;
  [job.lock lock];
  char *r = job.result ? strdup(job.result.UTF8String) : NULL;
  [job.lock unlock];
  return r;
}
void ot_launcher_item_release(void *p) {
  if (p)
    CFBridgingRelease(p);
}
static BOOL itemProcess(int pid, uint64_t sec, uint64_t usec) {
  struct proc_bsdinfo info = {0};
  return pid > 0 && sec && OT_ITEM_PROCESS_INFO(pid, &info) &&
         info.pbi_start_tvsec == sec && info.pbi_start_tvusec == usec;
}
void *ot_launcher_item_open(void *p, const char *kind, const char *display,
                            uint64_t token, int pid, uint64_t sec,
                            uint64_t usec, const char *link, uintptr_t guard) {
  OTLauncherItemScope *scope = (__bridge OTLauncherItemScope *)p;
  NSString *operation = [NSString stringWithUTF8String:kind],
           *uuid = [NSString stringWithUTF8String:display];
  NSURL *url =
      scope.url ?: [NSURL URLWithString:[NSString stringWithUTF8String:link]];
  OTLauncherItemJob *job = [OTLauncherItemJob new];
  job.lock = [NSLock new];
  job.guard = guard;
  OT_ITEM_MAIN(^{
    BOOL (^current)(void) = ^BOOL {
      return OT_ITEM_PANEL_CURRENT(token, uuid.UTF8String) &&
             (!scope || itemSame(scope)) && itemCurrent(job) &&
             OT_ITEM_PANEL_CURRENT(token, uuid.UTF8String) &&
             (!scope || itemSame(scope));
    };
    OT_ITEM_BEFORE_GUARD();
    if (!current()) {
      itemFinish(job, @{@"error" : @"retired"});
      return;
    }
    void (^launch)(void) = ^{
      if (!current()) {
        itemFinish(job, @{@"error" : @"retired"});
        return;
      }
      if ([scope.kind isEqual:@"app"] &&
          ![itemRunning(scope)[@"state"] isEqual:@"stopped"]) {
        itemFinish(job, @{@"error" : @"changed"});
        return;
      }
      NSWorkspaceOpenConfiguration *config =
          [NSWorkspaceOpenConfiguration configuration];
      config.promptsUserIfNeeded = NO;
      config.addsToRecentItems = NO;
      config.allowsRunningApplicationSubstitution = NO;
      config.createsNewApplicationInstance = NO;
      void (^done)(NSRunningApplication *, NSError *) =
          ^(NSRunningApplication *app, NSError *error) {
            itemFinish(job, error ? @{@"error" : @"dispatchRefused"} : @{});
          };
      if (!current() || ([scope.kind isEqual:@"app"] &&
                         ![itemRunning(scope)[@"state"] isEqual:@"stopped"])) {
        itemFinish(job, @{@"error" : @"changed"});
        return;
      }
      if ([scope.kind isEqual:@"app"])
        OT_ITEM_OPEN_APP(url, config, done);
      else
        OT_ITEM_OPEN_URL(url, config, done);
    };
    if ([operation isEqual:@"relaunch"]) {
      NSRunningApplication *app = OT_ITEM_PROCESS_APP(pid);
      if (!scope || ![scope.kind isEqual:@"app"] ||
          !itemProcess(pid, sec, usec) || app.terminated ||
          ![app.bundleIdentifier isEqual:scope.bundle] ||
          ![app.bundleURL.URLByResolvingSymlinksInPath.path isEqual:url.path] ||
          !current() || !itemProcess(pid, sec, usec) || app.terminated ||
          ![app terminate]) {
        itemFinish(job, @{@"error" : @"dispatchRefused"});
        return;
      }
      OT_ITEM_WORK(^{
        NSDate *deadline =
            [NSDate dateWithTimeIntervalSinceNow:OT_ITEM_RELAUNCH_SECONDS];
        while (!app.terminated && deadline.timeIntervalSinceNow > 0 &&
               itemCurrent(job)) {
          [NSThread sleepForTimeInterval:.025];
        }
        OT_ITEM_MAIN(^{
          if (!app.terminated || !current()) {
            itemFinish(job, @{
              @"error" : (!app.terminated && itemCurrent(job)) ? @"timeout"
                                                               : @"retired"
            });
            return;
          }
          NSArray *others = OT_ITEM_RUNNING(scope.bundle);
          if (others.count > 128) {
            itemFinish(job, @{@"error" : @"unavailable"});
            return;
          }
          BOOL matching = NO;
          for (NSRunningApplication *other in others) {
            if (!other.terminated &&
                [other.bundleIdentifier isEqual:scope.bundle] &&
                [other.bundleURL.URLByStandardizingPath
                        .URLByResolvingSymlinksInPath.path isEqual:url.path])
              matching = YES;
          }
          if (matching) {
            itemFinish(job, @{@"error" : @"changed"});
            return;
          }
          launch();
        });
      });
    } else {
      if (pid > 0) {
        NSRunningApplication *app = OT_ITEM_PROCESS_APP(pid);
        if (!scope || ![scope.kind isEqual:@"app"] ||
            !itemProcess(pid, sec, usec) || app.terminated ||
            ![app.bundleIdentifier isEqual:scope.bundle] ||
            ![app.bundleURL.URLByResolvingSymlinksInPath.path
                isEqual:url.path] ||
            !current() || !itemProcess(pid, sec, usec)) {
          itemFinish(job, @{@"error" : @"changed"});
          return;
        }
        if (!current() || !itemProcess(pid, sec, usec) || app.terminated) {
          itemFinish(job, @{@"error" : @"changed"});
          return;
        }
        itemFinish(job, [app activateWithOptions:0]
                            ? @{}
                            : @{@"error" : @"dispatchRefused"});
        return;
      }
      launch();
    }
  });
  return (__bridge_retained void *)job;
}
