//go:build darwin

#import "darwin_launcher.h"
#import "darwin_dock_lock.h"
#import "darwin_dock_panel.h"
#import <ApplicationServices/ApplicationServices.h>
#import <Cocoa/Cocoa.h>
#import <dlfcn.h>
#import <errno.h>
#import <libproc.h>
#import <stdatomic.h>

extern int goLauncherFinalGuard(uintptr_t);
static char *launcherJSON(id value) {
  NSData *data = value ? [NSJSONSerialization dataWithJSONObject:value
                                                         options:0
                                                           error:NULL]
                       : nil;
  return data ? strdup([[NSString alloc] initWithData:data
                                             encoding:NSUTF8StringEncoding]
                           .UTF8String)
              : NULL;
}
static NSDictionary *launcherRect(NSRect b, double top) {
  return @{
    @"X" : @(b.origin.x),
    @"Y" : @(top - NSMaxY(b)),
    @"W" : @(b.size.width),
    @"H" : @(b.size.height)
  };
}
static NSRect launcherSafeFrame(NSRect frame, NSRect visible,
                                NSEdgeInsets insets) {
  double values[] = {frame.origin.x,     frame.origin.y,      frame.size.width,
                     frame.size.height,  visible.origin.x,    visible.origin.y,
                     visible.size.width, visible.size.height, insets.top,
                     insets.left,        insets.bottom,       insets.right};
  for (int i = 0; i < 12; i++)
    if (!isfinite(values[i]))
      return NSZeroRect;
  if (insets.top < 0 || insets.left < 0 || insets.bottom < 0 ||
      insets.right < 0)
    return NSZeroRect;
  NSRect safe =
      NSMakeRect(frame.origin.x + insets.left, frame.origin.y + insets.bottom,
                 frame.size.width - insets.left - insets.right,
                 frame.size.height - insets.top - insets.bottom);
  if (NSIsEmptyRect(safe) || NSIsEmptyRect(visible))
    return NSZeroRect;
  return NSIntersectionRect(safe, visible);
}
static NSString *launcherUUID(CGDirectDisplayID display) {
  CFUUIDRef uuid = CGDisplayCreateUUIDFromDisplayID(display);
  if (!uuid)
    return nil;
  NSString *s = CFBridgingRelease(CFUUIDCreateString(NULL, uuid));
  CFRelease(uuid);
  return s;
}
char *ot_launcher_spaces(void) {
  @autoreleasepool {
    static void *handle;
    static int (*connection)(void);
    static CFArrayRef (*copy)(int);
    static dispatch_once_t once;
    dispatch_once(&once, ^{
      handle = dlopen(
          "/System/Library/PrivateFrameworks/SkyLight.framework/SkyLight",
          RTLD_LAZY | RTLD_LOCAL);
      if (handle) {
        connection = dlsym(handle, "SLSMainConnectionID");
        copy = dlsym(handle, "SLSCopyManagedDisplaySpaces");
      }
    });
    if (!connection || !copy)
      return NULL;
    id result = CFBridgingRelease(copy(connection()));
    return [result isKindOfClass:NSArray.class] && [result count] <= 32
               ? launcherJSON(result)
               : NULL;
  }
}
static NSDictionary *launcherDockProcess(void) {
  NSArray *apps = [NSRunningApplication
      runningApplicationsWithBundleIdentifier:@"com.apple.dock"];
  if (apps.count != 1)
    return nil;
  NSRunningApplication *app = apps.firstObject;
  if (app.terminated || ![app.bundleIdentifier isEqual:@"com.apple.dock"])
    return nil;
  pid_t pid = app.processIdentifier;
  struct proc_bsdinfo info = {0};
  if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) !=
          sizeof(info) ||
      !info.pbi_start_tvsec)
    return nil;
  return @{
    @"PID" : @(pid),
    @"StartSeconds" : @(info.pbi_start_tvsec),
    @"StartMicros" : @(info.pbi_start_tvusec)
  };
}
static NSDictionary *launcherDockBoundSnapshot(NSDictionary * (^identity)(void),
                                               NSDictionary * (^read)(void)) {
  NSDictionary *before = identity();
  if (!before)
    return nil;
  NSDictionary *snapshot = read();
  NSDictionary *after = identity();
  if (![snapshot isKindOfClass:NSDictionary.class] || ![before isEqual:after] ||
      ![snapshot[@"pid"] isEqual:before[@"PID"]])
    return nil;
  NSMutableDictionary *out = [snapshot mutableCopy];
  out[@"Process"] = before;
  return out;
}
static NSDictionary *launcherFocusedProcess(void) {
  NSRunningApplication *app = NSWorkspace.sharedWorkspace.frontmostApplication;
  NSString *bundle = app.bundleIdentifier;
  pid_t pid = app.processIdentifier;
  if (!app || app.terminated || pid <= 0 || !bundle.length ||
      bundle.length > 255)
    return nil;
  struct proc_bsdinfo info = {0};
  if (proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) !=
          sizeof(info) ||
      !info.pbi_start_tvsec || info.pbi_start_tvusec >= 1000000)
    return nil;
  return @{
    @"Process" : @{
      @"PID" : @(pid),
      @"StartSeconds" : @(info.pbi_start_tvsec),
      @"StartMicros" : @(info.pbi_start_tvusec)
    },
    @"BundleID" : bundle
  };
}
static NSDictionary *launcherFocusEvidence(NSDictionary *before,
                                           NSDictionary *after) {
  if (!before || !after || ![before isEqual:after] ||
      ![before[@"Process"] isKindOfClass:NSDictionary.class] ||
      ![before[@"BundleID"] isKindOfClass:NSString.class] ||
      ![before[@"BundleID"] length])
    return nil;
  return @{
    @"Known" : @YES,
    @"Process" : before[@"Process"],
    @"BundleID" : before[@"BundleID"]
  };
}
static BOOL launcherNotificationMatters(NSString *name, NSString *bundle) {
  if ([name isEqual:NSWorkspaceDidActivateApplicationNotification] ||
      [name isEqual:NSWorkspaceActiveSpaceDidChangeNotification])
    return YES;
  return ([name isEqual:NSWorkspaceDidLaunchApplicationNotification] ||
          [name isEqual:NSWorkspaceDidTerminateApplicationNotification]) &&
         [bundle isEqual:@"com.apple.dock"];
}
char *ot_launcher_environment(void) {
  @autoreleasepool {
    __block NSDictionary *focusBefore = nil;
    __block NSMutableArray *displays = [NSMutableArray array];
    __block BOOL complete = NO;
    void (^read)(void) = ^{
      focusBefore = launcherFocusedProcess();
      NSArray<NSScreen *> *screens = NSScreen.screens;
      double top = NSMaxY(screens.firstObject.frame);
      CGDirectDisplayID active[32];
      uint32_t count = 0;
      complete =
          CGGetActiveDisplayList(32, active, &count) == kCGErrorSuccess &&
          count > 0 && count < 32 && count == screens.count;
      for (NSScreen *screen in screens) {
        CGDirectDisplayID display =
            [screen.deviceDescription[@"NSScreenNumber"] unsignedIntValue];
        NSString *uuid = launcherUUID(display);
        if (!uuid) {
          complete = NO;
          continue;
        }
        // Mirrored layouts yield until their logical UUID mapping is
        // independently proven.
        [displays addObject:@{
          @"UUID" : uuid,
          @"Name" : screen.localizedName ?: @"Display",
          @"ID" : @(display),
          @"Main" : CGDisplayIsMain(display) ? @YES : @NO,
          @"MirrorGroup" : CGDisplayIsInMirrorSet(display) ? @"unresolved"
                                                           : @"",
          @"Frame" : launcherRect(screen.frame, top),
          @"UsableFrame" :
              launcherRect(launcherSafeFrame(screen.frame, screen.visibleFrame,
                                             screen.safeAreaInsets),
                           top),
          @"Scale" : @(screen.backingScaleFactor)
        }];
      }
    };
    if (NSThread.isMainThread)
      read();
    else
      dispatch_sync(dispatch_get_main_queue(), read);
    // Read-only helper only: never creates a D15 owner, tap, placement or
    // preference write.
    NSDictionary *dock = launcherDockBoundSnapshot(
        ^NSDictionary * {
          return launcherDockProcess();
        },
        ^NSDictionary * {
          char *dockJSON = ot_lock_read_container();
          if (!dockJSON)
            return nil;
          id value = [NSJSONSerialization
              JSONObjectWithData:[[NSString stringWithUTF8String:dockJSON]
                                     dataUsingEncoding:NSUTF8StringEncoding]
                         options:0
                           error:NULL];
          free(dockJSON);
          return [value isKindOfClass:NSDictionary.class] ? value : nil;
        });
    __block NSDictionary *focusAfter = nil;
    void (^readFocus)(void) = ^{
      focusAfter = launcherFocusedProcess();
    };
    if (NSThread.isMainThread)
      readFocus();
    else
      dispatch_sync(dispatch_get_main_queue(), readFocus);
    NSDictionary *focus = launcherFocusEvidence(focusBefore, focusAfter);
    return launcherJSON(@{
      @"Focus" : focus ?: @{},
      @"Complete" : @(complete),
      @"Displays" : displays,
      @"Dock" : dock ?: @{}
    });
  }
}
int ot_launcher_pointer(double *x, double *y) {
  CGEventRef event = CGEventCreate(NULL);
  if (!event)
    return 0;
  CGPoint p = CGEventGetLocation(event);
  CFRelease(event);
  if (!isfinite(p.x) || !isfinite(p.y))
    return 0;
  *x = p.x;
  *y = p.y;
  return 1;
}
static BOOL launcherProcess(int pid, uint64_t sec, uint64_t usec) {
  struct proc_bsdinfo info = {0};
  return proc_pidinfo(pid, PROC_PIDTBSDINFO, 0, &info, sizeof(info)) ==
             sizeof(info) &&
         info.pbi_start_tvsec == sec && info.pbi_start_tvusec == usec;
}
static BOOL launcherSpaceEntry(id entry) {
  if (![entry isKindOfClass:NSDictionary.class])
    return NO;
  for (NSString *key in @[ @"id64", @"ManagedSpaceID", @"type" ]) {
    id n = entry[key];
    if (![n isKindOfClass:NSNumber.class] ||
        CFGetTypeID((__bridge CFTypeRef)n) == CFBooleanGetTypeID())
      return NO;
    // Reject signed/fractional/nonfinite/overflow metadata before conversion.
    const char *text = [n stringValue].UTF8String;
    if (!text || !*text)
      return NO;
    for (const char *p = text; *p; p++)
      if (*p < '0' || *p > '9')
        return NO;
    errno = 0;
    char *end = NULL;
    strtoull(text, &end, 10);
    if (errno == ERANGE || !end || *end)
      return NO;
  }
  return [entry[@"id64"] unsignedLongLongValue] > 0 &&
         [entry[@"id64"] isEqual:entry[@"ManagedSpaceID"]];
}
static uint64_t launcherSpaceRecords(id records, NSString *display,
                                     uint64_t expected) {
  if (![records isKindOfClass:NSArray.class] || [records count] > 32)
    return 0;
  NSUInteger matched = 0;
  uint64_t valid = 0;
  for (id record in records) {
    if (![record isKindOfClass:NSDictionary.class] ||
        ![record[@"Display Identifier"] isEqual:display])
      continue;
    matched++;
    id current = record[@"Current Space"];
    id spaces = record[@"Spaces"];
    if (![current isKindOfClass:NSDictionary.class] ||
        ![spaces isKindOfClass:NSArray.class] || [spaces count] > 128)
      continue;
    if (!launcherSpaceEntry(current) ||
        (expected && [current[@"id64"] unsignedLongLongValue] != expected) ||
        [current[@"type"] unsignedLongLongValue] != 0)
      continue;
    NSUInteger count = 0;
    for (id s in spaces)
      if (launcherSpaceEntry(s) && [s[@"id64"] isEqual:current[@"id64"]] &&
          [s[@"ManagedSpaceID"] isEqual:current[@"ManagedSpaceID"]] &&
          [s[@"type"] isEqual:current[@"type"]])
        count++;
    valid = count == 1 ? [current[@"id64"] unsignedLongLongValue] : 0;
  }
  return matched == 1 ? valid : 0;
}
static uint64_t launcherSpaceCurrent(NSString *display, uint64_t expected) {
  char *raw = ot_launcher_spaces();
  if (!raw)
    return NO;
  id records = [NSJSONSerialization
      JSONObjectWithData:[[NSString stringWithUTF8String:raw]
                             dataUsingEncoding:NSUTF8StringEncoding]
                 options:0
                   error:NULL];
  free(raw);
  return launcherSpaceRecords(records, display, expected);
}

uint64_t ot_launcher_space_id(const char *display) {
  return display
             ? launcherSpaceCurrent([NSString stringWithUTF8String:display], 0)
             : 0;
}
int ot_launcher_space_ordinary(const char *display) {
  return ot_launcher_space_id(display) != 0;
}
// Same serial admission pipeline is exercised by the fixture using inert
// blocks.
static BOOL launcherActivatePrepared(BOOL (^identity)(void),
                                     BOOL (^space)(void), BOOL (^visible)(void),
                                     BOOL (^guard)(void),
                                     BOOL (^dispatch)(void)) {
  if (!identity() || !space() || !visible() || !guard())
    return NO;
  if (!space() || !identity() || !visible() || !guard())
    return NO;
  return dispatch();
}
int ot_launcher_activate(uint64_t token, const char *display, uint64_t space,
                         int pid, uint64_t sec, uint64_t usec,
                         const char *bundle, uintptr_t guard) {
  __block int result = 0;
  void (^perform)(void) = ^{
    @autoreleasepool {
      NSString *uuid = [NSString stringWithUTF8String:display],
               *identity = [NSString stringWithUTF8String:bundle];
      NSRunningApplication *app =
          [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
      result = launcherActivatePrepared(
          ^BOOL {
            return app && !app.terminated &&
                   [app.bundleIdentifier isEqual:identity] &&
                   launcherProcess(pid, sec, usec);
          },
          ^BOOL {
            return launcherSpaceCurrent(uuid, space);
          },
          ^BOOL {
            return ot_launcher_panel_visible(token, display) != 0 &&
                   ot_launcher_panel_space(token) == space;
          },
          ^BOOL {
            return goLauncherFinalGuard(guard) != 0;
          },
          ^BOOL {
            return [app activateWithOptions:0];
          });
    }
  };
  if (NSThread.isMainThread)
    perform();
  else
    dispatch_sync(dispatch_get_main_queue(), perform);
  return result;
}

@interface OTLauncherWatch : NSObject {
@public
  atomic_bool dirty;
}
@property uint64_t displayGeneration;
@property NSMutableArray *workspaceTokens;
@property id screenToken;
@end
@implementation OTLauncherWatch
@end
static atomic_uint_fast64_t launcherDisplayGeneration;
static atomic_bool launcherWatchInstalled;
static void launcherDisplayChanged(CGDirectDisplayID display,
                                   CGDisplayChangeSummaryFlags flags,
                                   void *context) {
  // Process-lifetime storage keeps an already-entered callback safe during
  // removal.
  atomic_fetch_add(&launcherDisplayGeneration, 1);
}
void *ot_launcher_watch_start(void) {
  bool expected = false;
  if (!atomic_compare_exchange_strong(&launcherWatchInstalled, &expected, true))
    return NULL;
  OTLauncherWatch *watch = [OTLauncherWatch new];
  watch.displayGeneration = atomic_load(&launcherDisplayGeneration);
  atomic_init(&watch->dirty, true);
  if (CGDisplayRegisterReconfigurationCallback(launcherDisplayChanged, NULL) !=
      kCGErrorSuccess) {
    atomic_store(&launcherWatchInstalled, false);
    return NULL;
  }
  void (^install)(void) = ^{
    watch.workspaceTokens = [NSMutableArray array];
    for (NSString *name in @[
           NSWorkspaceDidActivateApplicationNotification,
           NSWorkspaceActiveSpaceDidChangeNotification,
           NSWorkspaceDidLaunchApplicationNotification,
           NSWorkspaceDidTerminateApplicationNotification
         ]) {
      id token = [NSWorkspace.sharedWorkspace.notificationCenter
          addObserverForName:name
                      object:nil
                       queue:nil
                  usingBlock:^(NSNotification *note) {
                    NSRunningApplication *app =
                        note.userInfo[NSWorkspaceApplicationKey];
                    if (!launcherNotificationMatters(note.name,
                                                     app.bundleIdentifier))
                      return;
                    atomic_store(&watch->dirty, true);
                  }];
      [watch.workspaceTokens addObject:token];
    }
  };
  if (NSThread.isMainThread)
    install();
  else
    dispatch_sync(dispatch_get_main_queue(), install);
  return (__bridge_retained void *)watch;
}
int ot_launcher_watch_changed(void *pointer) {
  OTLauncherWatch *watch = (__bridge OTLauncherWatch *)pointer;
  uint64_t current = atomic_load(&launcherDisplayGeneration);
  BOOL changed = current != watch.displayGeneration;
  watch.displayGeneration = current;
  return atomic_exchange(&watch->dirty, false) || changed;
}
void ot_launcher_watch_stop(void *pointer) {
  OTLauncherWatch *watch = CFBridgingRelease(pointer);
  CGDisplayRemoveReconfigurationCallback(launcherDisplayChanged, NULL);
  void (^remove)(void) = ^{
    for (id token in watch.workspaceTokens)
      [NSWorkspace.sharedWorkspace.notificationCenter removeObserver:token];
    [watch.workspaceTokens removeAllObjects];
  };
  if (NSThread.isMainThread)
    remove();
  else
    dispatch_sync(dispatch_get_main_queue(), remove);
  atomic_store(&launcherWatchInstalled, false);
}
