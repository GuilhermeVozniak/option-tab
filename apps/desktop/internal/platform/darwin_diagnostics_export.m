//go:build darwin

#import "darwin_diagnostics_export.h"
#import <Cocoa/Cocoa.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <sys/stat.h>
#include <unistd.h>

#ifndef OT_DIAGNOSTIC_BEFORE_COMMIT
#define OT_DIAGNOSTIC_BEFORE_COMMIT(g) ((void)0)
#endif
#ifndef OT_DIAGNOSTIC_AFTER_COMMIT
#define OT_DIAGNOSTIC_AFTER_COMMIT(g) ((void)0)
#endif
#ifndef OT_DIAGNOSTIC_MAKE_PANEL
#define OT_DIAGNOSTIC_MAKE_PANEL() [NSSavePanel savePanel]
#endif
#ifndef OT_DIAGNOSTIC_SCHEDULE_MAIN
#define OT_DIAGNOSTIC_SCHEDULE_MAIN(work)                                      \
  dispatch_async(dispatch_get_main_queue(), work)
#endif
#ifndef OT_DIAGNOSTIC_CONTEXT_CURRENT
extern int goDiagnosticContextCurrent(uintptr_t);
#define OT_DIAGNOSTIC_CONTEXT_CURRENT(token) goDiagnosticContextCurrent(token)
#endif
@interface OTDiagnosticSave : NSObject
@property NSLock *lock;
@property NSSavePanel *panel;
@property NSData *data;
@property BOOL cancelled;
@property int result;
@property uintptr_t admission;
@end
@implementation OTDiagnosticSave
@end
static void diagnosticFinish(OTDiagnosticSave *g, int result) {
  [g.lock lock];
  if (!g.result)
    g.result = result;
  [g.lock unlock];
}
static BOOL diagnosticCancelled(OTDiagnosticSave *g) {
  [g.lock lock];
  BOOL cancelled = g.cancelled;
  [g.lock unlock];
  return cancelled || !OT_DIAGNOSTIC_CONTEXT_CURRENT(g.admission);
}
// A captured parent directory FD plus exclusive atomic rename prevents a
// destination created during preparation from being overwritten. No raw path
// or native error descriptions escape this owner.
static void diagnosticWrite(OTDiagnosticSave *g, NSURL *selected) {
  @autoreleasepool {
    if (diagnosticCancelled(g)) {
      diagnosticFinish(g, 2);
      return;
    }
    if (!selected.isFileURL || g.data.length == 0 ||
        g.data.length > 512 * 1024) {
      diagnosticFinish(g, 4);
      return;
    }
    BOOL scoped = [selected startAccessingSecurityScopedResource];
    NSURL *parent = selected.URLByDeletingLastPathComponent
                        .URLByStandardizingPath.URLByResolvingSymlinksInPath;
    NSString *name = selected.lastPathComponent;
    int result = 4, dir = -1, stage = -1, fd = -1;
    BOOL stageCreated = NO, fileCreated = NO;
    struct stat stageIdentity = {0}, fileIdentity = {0}, observed = {0};
    NSString *temporary = [@".option-tab-diagnostic-"
        stringByAppendingString:NSUUID.UUID.UUIDString];
    const char *payload = "payload";
    if (name.length == 0 || [name isEqual:@"."] || [name isEqual:@".."] ||
        [name containsString:@"/"])
      goto done;
    dir = open(parent.fileSystemRepresentation,
               O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC);
    if (dir < 0)
      goto done;
    if (fstatat(dir, name.fileSystemRepresentation, &observed,
                AT_SYMLINK_NOFOLLOW) == 0) {
      result = 3;
      goto done;
    }
    if (errno != ENOENT)
      goto done;
    if (mkdirat(dir, temporary.fileSystemRepresentation, 0700) != 0)
      goto done;
    stageCreated = YES;
    stage = openat(dir, temporary.fileSystemRepresentation,
                   O_RDONLY | O_DIRECTORY | O_NOFOLLOW | O_CLOEXEC);
    if (stage < 0 || fstat(stage, &stageIdentity) != 0 ||
        !S_ISDIR(stageIdentity.st_mode) ||
        (stageIdentity.st_mode & 0777) != 0700 ||
        stageIdentity.st_uid != geteuid())
      goto done;
    fd = openat(stage, payload,
                O_RDWR | O_CREAT | O_EXCL | O_NOFOLLOW | O_CLOEXEC, 0600);
    if (fd < 0 || fstat(fd, &fileIdentity) != 0)
      goto done;
    fileCreated = YES;
    const unsigned char *bytes = g.data.bytes;
    size_t left = g.data.length;
    while (left) {
      if (diagnosticCancelled(g)) {
        result = 2;
        goto done;
      }
      ssize_t count = write(fd, bytes, MIN(left, (size_t)16384));
      if (count < 0 && errno == EINTR)
        continue;
      if (count <= 0)
        goto done;
      left -= (size_t)count;
      bytes += count;
    }
    if (fsync(fd) != 0)
      goto done;
    OT_DIAGNOSTIC_BEFORE_COMMIT(g);
    // Both retained descriptors and their no-follow names must still match.
    // The private stage prevents unrelated users from substituting the source.
    // This is not a security boundary against an arbitrary same-UID writer.
    if (fstatat(dir, temporary.fileSystemRepresentation, &observed,
                AT_SYMLINK_NOFOLLOW) != 0 ||
        !S_ISDIR(observed.st_mode) || observed.st_dev != stageIdentity.st_dev ||
        observed.st_ino != stageIdentity.st_ino)
      goto done;
    if (fstatat(stage, payload, &observed, AT_SYMLINK_NOFOLLOW) != 0 ||
        !S_ISREG(observed.st_mode) || observed.st_dev != fileIdentity.st_dev ||
        observed.st_ino != fileIdentity.st_ino || observed.st_nlink != 1 ||
        observed.st_size != (off_t)g.data.length)
      goto done;
    // Also refuse an observed rewrite of the retained inode's bytes.
    unsigned char verify[16384];
    size_t offset = 0;
    while (offset < g.data.length) {
      if (diagnosticCancelled(g)) {
        result = 2;
        goto done;
      }
      size_t wanted = MIN(sizeof(verify), g.data.length - offset);
      ssize_t got = pread(fd, verify, wanted, (off_t)offset);
      if (got < 0 && errno == EINTR)
        continue;
      if (got != (ssize_t)wanted ||
          memcmp(verify, (const unsigned char *)g.data.bytes + offset, wanted))
        goto done;
      offset += wanted;
    }
    [g.lock lock];
    if (g.cancelled || !OT_DIAGNOSTIC_CONTEXT_CURRENT(g.admission))
      result = 2;
    else if (renameatx_np(stage, payload, dir, name.fileSystemRepresentation,
                          RENAME_EXCL) == 0) {
      fileCreated = NO;
      result = 1;
    } else if (errno == EEXIST)
      result = 3;
    [g.lock unlock];
    OT_DIAGNOSTIC_AFTER_COMMIT(g);
  done:
    // Never delete an observed substituted source or stage directory.
    if (fileCreated && stage >= 0 &&
        fstatat(stage, payload, &observed, AT_SYMLINK_NOFOLLOW) == 0 &&
        S_ISREG(observed.st_mode) && observed.st_dev == fileIdentity.st_dev &&
        observed.st_ino == fileIdentity.st_ino)
      unlinkat(stage, payload, 0);
    if (fd >= 0)
      close(fd);
    if (stage >= 0)
      close(stage);
    if (stageCreated && dir >= 0 &&
        fstatat(dir, temporary.fileSystemRepresentation, &observed,
                AT_SYMLINK_NOFOLLOW) == 0 &&
        S_ISDIR(observed.st_mode) && observed.st_dev == stageIdentity.st_dev &&
        observed.st_ino == stageIdentity.st_ino)
      unlinkat(dir, temporary.fileSystemRepresentation, AT_REMOVEDIR);
    if (dir >= 0)
      close(dir);
    if (scoped)
      [selected stopAccessingSecurityScopedResource];
    diagnosticFinish(g, result);
  }
}
int ot_diagnostics_main_thread(void) { return NSThread.isMainThread ? 1 : 0; }
void *ot_diagnostics_save_start(const void *bytes, size_t length,
                                uintptr_t admission) {
  return ot_json_save_start(bytes, length, admission, "option-tab-diagnostics.json");
}
void *ot_json_save_start(const void *bytes, size_t length, uintptr_t admission,
                         const char *filename) {
  @autoreleasepool {
    if (!filename ||
        (strcmp(filename, "option-tab-diagnostics.json") &&
         strcmp(filename, "option-tab-launcher-profile.json") &&
         strcmp(filename, "option-tab-settings.json")) ||
        !bytes || length == 0 || length > 512 * 1024)
      return NULL;
    NSString *name = [NSString stringWithUTF8String:filename];
    OTDiagnosticSave *g = [OTDiagnosticSave new];
    g.lock = [NSLock new];
    g.data = [NSData dataWithBytes:bytes length:length];
    g.admission = admission;
    OT_DIAGNOSTIC_SCHEDULE_MAIN(^{
      if (diagnosticCancelled(g)) {
        diagnosticFinish(g, 2);
        return;
      }
      NSSavePanel *panel = OT_DIAGNOSTIC_MAKE_PANEL();
      g.panel = panel;
      panel.nameFieldStringValue = name;
      panel.allowedContentTypes = @[ UTTypeJSON ];
      panel.canCreateDirectories = YES;
      if (diagnosticCancelled(g)) {
        g.panel = nil;
        diagnosticFinish(g, 2);
        return;
      }
      [panel beginWithCompletionHandler:^(NSModalResponse response) {
        NSURL *selected = panel.URL;
        g.panel = nil;
        if (response != NSModalResponseOK || !selected ||
            diagnosticCancelled(g)) {
          diagnosticFinish(g, 2);
          return;
        }
        dispatch_async(dispatch_get_global_queue(QOS_CLASS_UTILITY, 0), ^{
          diagnosticWrite(g, selected);
        });
      }];
    });
    return (__bridge_retained void *)g;
  }
}
void ot_diagnostics_save_cancel(void *value) {
  OTDiagnosticSave *g = (__bridge OTDiagnosticSave *)value;
  [g.lock lock];
  g.cancelled = YES;
  [g.lock unlock];
  OT_DIAGNOSTIC_SCHEDULE_MAIN(^{
    [g.panel cancel:nil];
  });
}
int ot_diagnostics_save_poll(void *value) {
  OTDiagnosticSave *g = (__bridge OTDiagnosticSave *)value;
  [g.lock lock];
  int result = g.result;
  [g.lock unlock];
  return result;
}
void ot_diagnostics_save_release(void *value) {
  if (value)
    CFBridgingRelease(value);
}
