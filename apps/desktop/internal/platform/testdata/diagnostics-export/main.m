#import <Cocoa/Cocoa.h>
#include <assert.h>
#define OT_DIAGNOSTIC_CONTEXT_CURRENT(token) (1)
// Stand-in only: this fixture must never construct or present AppKit UI.
static NSString *expectedName;
static int panels;
@interface FixtureSavePanel : NSObject
@property NSString *nameFieldStringValue;
@property NSArray *allowedContentTypes;
@property BOOL canCreateDirectories;
@property NSURL *URL;
@end
@implementation FixtureSavePanel
- (void)beginWithCompletionHandler:(void (^)(NSModalResponse))completion {
  assert([self.nameFieldStringValue isEqual:expectedName]);
  assert(self.allowedContentTypes.count == 1);
  completion(NSModalResponseCancel);
}
- (void)cancel:(id)sender {}
@end
static NSSavePanel *fakePanel(void) {
  panels++;
  return (NSSavePanel *)[FixtureSavePanel new];
}
#define OT_DIAGNOSTIC_MAKE_PANEL() fakePanel()
static void beforeCommit(void);
static void afterCommit(void);
#define OT_DIAGNOSTIC_AFTER_COMMIT(g) afterCommit()
#define OT_DIAGNOSTIC_BEFORE_COMMIT(g) beforeCommit()
#include "../../darwin_diagnostics_export.m"
static OTDiagnosticSave *owner;
static NSURL *target;
static int mode;
static NSURL *substituted;
static void beforeCommit(void) {
  if (mode == 4 || mode == 5) {
    NSURL *parent = target.URLByDeletingLastPathComponent;
    for (NSString *entry in
         [NSFileManager.defaultManager contentsOfDirectoryAtPath:parent.path
                                                           error:nil]) {
      if (![entry hasPrefix:@".option-tab-diagnostic-"])
        continue;
      NSURL *candidate = [parent URLByAppendingPathComponent:entry];
      BOOL directory = NO;
      [NSFileManager.defaultManager fileExistsAtPath:candidate.path
                                         isDirectory:&directory];
      if (directory)
        candidate = [candidate URLByAppendingPathComponent:@"payload"];
      struct stat info;
      if (lstat(candidate.fileSystemRepresentation, &info) != 0)
        continue;
      assert(unlink(candidate.fileSystemRepresentation) == 0);
      if (mode == 4)
        assert(symlink("missing-untrusted-source",
                       candidate.fileSystemRepresentation) == 0);
      else
        assert([@"substituted" writeToURL:candidate
                               atomically:NO
                                 encoding:NSUTF8StringEncoding
                                    error:nil]);
      substituted = candidate;
      break;
    }
    assert(substituted);
  }
  if (mode == 1) {
    [owner.lock lock];
    owner.cancelled = YES;
    [owner.lock unlock];
  }
  if (mode == 2) {
    [@"other" writeToURL:target
              atomically:NO
                encoding:NSUTF8StringEncoding
                   error:nil];
  }
}
static void afterCommit(void) {
  assert(owner.result == 0);
  if (mode == 3) {
    [owner.lock lock];
    owner.cancelled = YES;
    [owner.lock unlock];
  }
}
static int run(NSURL *url, BOOL cancelled, int testMode) {
  owner = [OTDiagnosticSave new];
  owner.lock = [NSLock new];
  owner.data = [@"reviewed" dataUsingEncoding:NSUTF8StringEncoding];
  owner.cancelled = cancelled;
  target = url;
  mode = testMode;
  diagnosticWrite(owner, url);
  return owner.result;
}
int main(int argc, char **argv) {
  @autoreleasepool {
    const char payload[] = "{}";
    void *pending = ot_diagnostics_save_start(payload, 2, 0);
    assert(pending);
    ot_diagnostics_save_cancel(pending);
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:1];
    while (ot_diagnostics_save_poll(pending) == 0 &&
           deadline.timeIntervalSinceNow > 0) {
      [NSRunLoop.mainRunLoop
          runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.005]];
    }
    assert(ot_diagnostics_save_poll(pending) == 2);
    assert(panels == 0);
    ot_diagnostics_save_release(pending);
    assert(!ot_json_save_start(payload, 2, 0, "../private.json"));
    assert(!ot_json_save_start(payload, 2, 0, NULL));
    for (NSString *name in @[@"option-tab-diagnostics.json", @"option-tab-launcher-profile.json", @"option-tab-settings.json"]) {
      expectedName = name;
      void *named = ot_json_save_start(payload, 2, 0, name.UTF8String);
      assert(named);
      deadline = [NSDate dateWithTimeIntervalSinceNow:1];
      while (ot_diagnostics_save_poll(named) == 0 && deadline.timeIntervalSinceNow > 0)
        [NSRunLoop.mainRunLoop runUntilDate:[NSDate dateWithTimeIntervalSinceNow:0.005]];
      assert(ot_diagnostics_save_poll(named) == 2);
      ot_diagnostics_save_release(named);
    }
    assert(panels == 3);
    assert(argc == 2);
    NSURL *dir = [NSURL fileURLWithPath:@(argv[1]) isDirectory:YES];
    NSURL *ok = [dir URLByAppendingPathComponent:@"ok.json"];
    assert(run(ok, NO, 0) == 1);
    assert([[@"reviewed" dataUsingEncoding:NSUTF8StringEncoding]
        isEqual:[NSData dataWithContentsOfURL:ok]]);
    assert(run(ok, NO, 0) == 3);
    assert([[@"reviewed" dataUsingEncoding:NSUTF8StringEncoding]
        isEqual:[NSData dataWithContentsOfURL:ok]]);
    NSURL *cancel = [dir URLByAppendingPathComponent:@"cancel.json"];
    assert(run(cancel, YES, 0) == 2);
    assert(![NSFileManager.defaultManager fileExistsAtPath:cancel.path]);
    assert(run(cancel, NO, 1) == 2);
    assert(![NSFileManager.defaultManager fileExistsAtPath:cancel.path]);
    NSURL *race = [dir URLByAppendingPathComponent:@"race.json"];
    assert(run(race, NO, 2) == 3);
    assert([[NSString stringWithContentsOfURL:race
                                     encoding:NSUTF8StringEncoding
                                        error:nil] isEqual:@"other"]);
    NSURL *late = [dir URLByAppendingPathComponent:@"late.json"];
    assert(run(late, NO, 3) == 1);
    NSURL *link = [dir URLByAppendingPathComponent:@"link.json"];
    assert(symlink(ok.fileSystemRepresentation,
                   link.fileSystemRepresentation) == 0);
    assert(run(link, NO, 0) == 3);
    NSURL *replacement = [dir URLByAppendingPathComponent:@"replacement.json"];
    assert(run(replacement, NO, 4) == 4);
    assert(![NSFileManager.defaultManager fileExistsAtPath:replacement.path]);
    struct stat replacementInfo;
    assert(lstat(substituted.fileSystemRepresentation, &replacementInfo) == 0 &&
           S_ISLNK(replacementInfo.st_mode));
    // Fixture owns the injected resource and removes it itself, not the
    // exporter.
    NSURL *stage = substituted.URLByDeletingLastPathComponent;
    assert(unlink(substituted.fileSystemRepresentation) == 0);
    if (![stage.path isEqual:dir.path])
      assert(rmdir(stage.fileSystemRepresentation) == 0);
    substituted = nil;
    assert(run(replacement, NO, 5) == 4);
    assert(![NSFileManager.defaultManager fileExistsAtPath:replacement.path]);
    assert([[NSString stringWithContentsOfURL:substituted
                                     encoding:NSUTF8StringEncoding
                                        error:nil] isEqual:@"substituted"]);
    stage = substituted.URLByDeletingLastPathComponent;
    assert(unlink(substituted.fileSystemRepresentation) == 0);
    if (![stage.path isEqual:dir.path])
      assert(rmdir(stage.fileSystemRepresentation) == 0);
    for (NSString *name in
         [NSFileManager.defaultManager contentsOfDirectoryAtPath:dir.path
                                                           error:nil])
      assert(![name hasPrefix:@".option-tab-diagnostic-"]);
    puts("PASS exact new-file write, existing/symlink refusal, cancellation, "
         "atomic collision, temp cleanup; no GUI");
  }
}
