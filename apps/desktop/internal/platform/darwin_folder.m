//go:build darwin

#import "darwin_folder.h"
#import <Cocoa/Cocoa.h>
#import <sys/stat.h>
extern int goFolderFinalGuard(uintptr_t);
@interface OTFolderScope : NSObject
@property NSURL *url;
@property BOOL started;
@end
@implementation OTFolderScope
- (void)dealloc {
  if (_started)
    [_url stopAccessingSecurityScopedResource];
}
@end
static NSURL *folderURL(const char *path) {
  if (!path)
    return nil;
  NSString *p = [NSString stringWithUTF8String:path];
  if (!p.isAbsolutePath)
    return nil;
  NSURL *u = [NSURL fileURLWithPath:p];
  return [u.URLByStandardizingPath.URLByResolvingSymlinksInPath.path isEqual:p]
             ? u
             : nil;
}
static NSData *bookmark(NSURL *url, NSError **error) {
  return [url bookmarkDataWithOptions:NSURLBookmarkCreationWithSecurityScope
       includingResourceValuesForKeys:@[
         NSURLFileResourceIdentifierKey, NSURLVolumeIdentifierKey
       ]
                        relativeToURL:nil
                                error:error];
}
void *ot_folder_begin(const char *path, const void *data, int length,
                      char **message, char **refresh) {
  @autoreleasepool {
    NSURL *requested = folderURL(path);
    if (!requested) {
      *message = strdup("folder path is no longer canonical");
      return NULL;
    }
    NSURL *resolved = requested;
    BOOL stale = NO;
    NSError *error = nil;
    NSData *bytes =
        length > 0 ? [NSData dataWithBytes:data length:length] : nil;
    if (bytes) {
      resolved = [NSURL
          URLByResolvingBookmarkData:bytes
                             options:NSURLBookmarkResolutionWithoutUI |
                                     NSURLBookmarkResolutionWithoutMounting |
                                     NSURLBookmarkResolutionWithSecurityScope
                       relativeToURL:nil
                 bookmarkDataIsStale:&stale
                               error:&error];
      if (!resolved ||
          ![resolved.URLByStandardizingPath.URLByResolvingSymlinksInPath.path
              isEqual:requested.path]) {
        *message =
            strdup("folder bookmark is stale, revoked, or points elsewhere");
        return NULL;
      }
    }
    OTFolderScope *scope = [OTFolderScope new];
    scope.url = resolved;
    scope.started =
        bytes ? [resolved startAccessingSecurityScopedResource] : NO;
    if (bytes) {
      NSArray *keys =
          @[ NSURLFileResourceIdentifierKey, NSURLVolumeIdentifierKey ];
      NSDictionary *original = [NSURL resourceValuesForKeys:keys
                                           fromBookmarkData:bytes];
      NSDictionary *current = [resolved resourceValuesForKeys:keys
                                                        error:&error];
      for (NSString *key in keys) {
        if (!original[key] || !current[key] ||
            ![original[key] isEqual:current[key]]) {
          *message =
              strdup("folder bookmark file identity changed or is unavailable");
          return NULL;
        }
      }
      if (stale) {
        NSData *updated = bookmark(resolved, &error);
        if (!updated) {
          *message = strdup("folder bookmark refresh failed");
          return NULL;
        }
        *refresh =
            strdup([updated base64EncodedStringWithOptions:0].UTF8String);
      }
    }
    return (__bridge_retained void *)scope;
  }
}
void ot_folder_end(void *value) {
  if (value) {
    CFBridgingRelease(value);
  }
}
static int folderOpen(void *value, const char *path, uint64_t pd, uint64_t pi,
                      uint64_t cd, uint64_t ci, uintptr_t guard, BOOL test) {
  @autoreleasepool {
    OTFolderScope *scope = (__bridge OTFolderScope *)value;
    NSString *child = [NSString stringWithUTF8String:path];
    if (!scope || !child || !guard)
      return -1;
    __block int result = -1;
    void (^open)(void) = ^{
      @autoreleasepool {
        BOOL (^currentIdentity)(void) = ^BOOL {
          NSURL *current = folderURL(path);
          if (!current || ![current.URLByDeletingLastPathComponent.path
                              isEqual:scope.url.URLByStandardizingPath
                                          .URLByResolvingSymlinksInPath.path])
            return NO;
          struct stat parent, entry;
          return !lstat(scope.url.fileSystemRepresentation, &parent) &&
                 !lstat(current.fileSystemRepresentation, &entry) &&
                 S_ISDIR(parent.st_mode) &&
                 (S_ISDIR(entry.st_mode) || S_ISREG(entry.st_mode)) &&
                 (uint64_t)parent.st_dev == pd &&
                 (uint64_t)parent.st_ino == pi &&
                 (uint64_t)entry.st_dev == cd && (uint64_t)entry.st_ino == ci;
        };
        if (!currentIdentity())
          return;
        if (!goFolderFinalGuard(guard)) {
          result = -2;
          return;
        }
        // The external guard can block; retire filesystem replacements that
        // occurred during it before handing the path to NSWorkspace.
        if (!currentIdentity())
          return;
        NSURL *url = [NSURL fileURLWithPath:child];
        result =
            test ? -4 : ([NSWorkspace.sharedWorkspace openURL:url] ? 0 : -3);
      }
    };
    if (test || NSThread.isMainThread)
      open();
    else
      dispatch_sync(dispatch_get_main_queue(), open);
    return result;
  }
}
@interface OTFolderGrant : NSObject
@property NSLock *lock;
@property NSOpenPanel *panel;
@property NSURL *requested;
@property NSString *result;
@property BOOL cancelled;
@end
@implementation OTFolderGrant
@end
static NSString *grantJSON(NSDictionary *value) {
  NSData *d = [NSJSONSerialization dataWithJSONObject:value
                                              options:0
                                                error:nil];
  return [[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding];
}
static void grantFinish(OTFolderGrant *g, NSDictionary *value) {
  [g.lock lock];
  if (!g.result)
    g.result = grantJSON(value);
  [g.lock unlock];
}
void *ot_folder_grant_start(const char *path) {
  @autoreleasepool {
    NSURL *requested = folderURL(path);
    if (!requested)
      return NULL;
    OTFolderGrant *g = [OTFolderGrant new];
    g.lock = [NSLock new];
    g.requested = requested;
    dispatch_async(dispatch_get_main_queue(), ^{
      @autoreleasepool {
        [g.lock lock];
        BOOL cancelled = g.cancelled;
        [g.lock unlock];
        if (cancelled) {
          grantFinish(g, @{
            @"Status" : @"permissionRequired",
            @"Reason" : @"folder access cancelled"
          });
          return;
        }
        NSOpenPanel *panel = [NSOpenPanel openPanel];
        g.panel = panel;
        panel.canChooseFiles = NO;
        panel.canChooseDirectories = YES;
        panel.allowsMultipleSelection = NO;
        panel.canCreateDirectories = NO;
        panel.directoryURL = requested;
        [panel beginWithCompletionHandler:^(NSModalResponse response) {
          @autoreleasepool {
            [g.lock lock];
            BOOL stop = g.cancelled;
            [g.lock unlock];
            NSURL *selected = panel.URL;
            if (stop || response != NSModalResponseOK || !selected) {
              grantFinish(g, @{
                @"Status" : @"permissionRequired",
                @"Reason" : @"folder access cancelled"
              });
              g.panel = nil;
              return;
            }
            BOOL started = [selected startAccessingSecurityScopedResource];
            NSDictionary *result;
            NSURL *canonical =
                selected.URLByStandardizingPath.URLByResolvingSymlinksInPath;
            struct stat info;
            NSError *error = nil;
            if (![canonical.path isEqual:requested.path] ||
                lstat(canonical.fileSystemRepresentation, &info) ||
                !S_ISDIR(info.st_mode)) {
              result = @{
                @"Status" : @"permissionRequired",
                @"Reason" :
                    @"selected folder does not match the captured Dock folder"
              };
            } else {
              NSData *bytes = bookmark(selected, &error);
              if (bytes)
                result = @{
                  @"Status" : @"ready",
                  @"Bookmark" : [bytes base64EncodedStringWithOptions:0],
                  @"Device" : @((uint64_t)info.st_dev),
                  @"Inode" : @((uint64_t)info.st_ino)
                };
              else
                result = @{
                  @"Status" : @"permissionRequired",
                  @"Reason" : error.localizedDescription
                      ?: @"bookmark creation failed"
                };
            }
            if (started)
              [selected stopAccessingSecurityScopedResource];
            grantFinish(g, result);
            g.panel = nil;
          }
        }];
      }
    });
    return (__bridge_retained void *)g;
  }
}
void ot_folder_grant_cancel(void *value) {
  OTFolderGrant *g = (__bridge OTFolderGrant *)value;
  [g.lock lock];
  g.cancelled = YES;
  [g.lock unlock];
  dispatch_async(dispatch_get_main_queue(), ^{
    [g.panel cancel:nil];
  });
}
char *ot_folder_grant_poll(void *value) {
  @autoreleasepool {
    OTFolderGrant *g = (__bridge OTFolderGrant *)value;
    [g.lock lock];
    NSString *result = g.result;
    [g.lock unlock];
    return result ? strdup(result.UTF8String) : NULL;
  }
}
void ot_folder_grant_release(void *value) {
  if (value) {
    CFBridgingRelease(value);
  }
}
// Read-only fixture seam: create bookmark bytes without showing any chooser.
char *ot_folder_test_bookmark(const char *path) {
  @autoreleasepool {
    NSURL *url = folderURL(path);
    if (!url)
      return NULL;
    NSData *data = bookmark(url, NULL);
    return data ? strdup([data base64EncodedStringWithOptions:0].UTF8String)
                : NULL;
  }
}

int ot_folder_open(void *s, const char *p, uint64_t pd, uint64_t pi,
                   uint64_t cd, uint64_t ci, uintptr_t g) {
  return folderOpen(s, p, pd, pi, cd, ci, g, NO);
}
int ot_folder_open_probe(void *s, const char *p, uint64_t pd, uint64_t pi,
                         uint64_t cd, uint64_t ci, uintptr_t g) {
  return folderOpen(s, p, pd, pi, cd, ci, g, YES);
}
