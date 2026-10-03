//go:build darwin

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ServiceManagement/ServiceManagement.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <Carbon/Carbon.h> // ProcessSerialNumber, GetProcessForPID (for SkyLight focus)
#include <pthread.h>
#include <stdatomic.h>
#include <unistd.h>
#include "darwin.h"

// SkyLight private API for precise front-window control (the AltTab technique):
// bringing a *specific* window's process frontmost makes macOS follow to that
// window's Space and make it key, WITHOUT moving the window to the current
// Space (which activate+raise alone does when the space switch is a no-op).
extern void _SLPSSetFrontProcessWithOptions(ProcessSerialNumber *psn, uint32_t wid, uint32_t mode);
extern CGError SLPSPostEventRecordTo(ProcessSerialNumber *psn, uint8_t *bytes);

// Exported from Go (see darwin.go): receives hotkey events from the tap thread.
extern void goHotkeyEvent(int kind, int id);
// Receives a raw key press forwarded while the switcher overlay is open.
extern void goKeyEvent(int keycode, uint64_t flags, const char *text);
// Receives frontmost-app identity changes so Go applies the same Unicode
// case-folding semantics as the controller before the next tap decision.
extern void goHotkeyFrontAppChanged(const char *bundle_id, const char *app_name);

#include <stdio.h>
#include <stdlib.h>
static int otDebug(void) {
  static int d = -1;
  if (d < 0) { const char *e = getenv("OPTIONTAB_DEBUG"); d = (e && *e) ? 1 : 0; }
  return d;
}
#define OTLOG(...) do { if (otDebug()) { fprintf(stderr, "[ot] " __VA_ARGS__); fflush(stderr); } } while (0)

// Private API used (as AltTab does) to map an AXUIElement to its CGWindowID.
extern AXError _AXUIElementGetWindow(AXUIElementRef element, CGWindowID *windowID);
// Private API: build an AXUIElement for another process from a 20-byte remote
// token (pid, 0, 'coco', AXUIElementID). The only way to reach windows absent
// from a kAXWindowsAttribute scan, i.e. windows on other Spaces (AltTab's
// windowByBruteForce). Returns +1 retained, or NULL for a dead id.
extern AXUIElementRef _AXUIElementCreateWithRemoteToken(CFDataRef token);

// Private CGS (SkyLight) API for Space resolution, as AltTab uses. These live in
// CoreGraphics and link without a public header. All callers degrade to 0 when
// the connection or result is unavailable, so a missing or changed API makes
// Space data inert (filters fall back to "all") rather than crashing.
typedef int CGSConnectionID;
extern CGSConnectionID CGSMainConnectionID(void);
extern uint64_t CGSGetActiveSpace(CGSConnectionID cid);
extern CFArrayRef CGSCopyManagedDisplaySpaces(CGSConnectionID cid);
extern CFArrayRef CGSCopyWindowsWithOptionsAndTags(CGSConnectionID cid, uint32_t owner, CFArrayRef spaces, uint32_t options, uint64_t *setTags, uint64_t *clearTags);

// ---- Helpers ----

static char *copyCString(NSString *s) {
  if (s == nil) s = @"";
  const char *utf8 = [s UTF8String];
  char *out = malloc(strlen(utf8) + 1);
  strcpy(out, utf8);
  return out;
}

// ---- Display helpers ----

// displayForPoint returns the CGDirectDisplayID whose bounds contain (x,y) in
// global CG coordinates (origin top-left), falling back to the main display.
static uint32_t displayForPoint(CGFloat x, CGFloat y) {
  CGDirectDisplayID disps[16];
  uint32_t count = 0;
  if (CGGetDisplaysWithPoint(CGPointMake(x, y), 16, disps, &count) == kCGErrorSuccess && count > 0) {
    return (uint32_t)disps[0];
  }
  return (uint32_t)CGMainDisplayID();
}

// isFullscreenRect reports whether a window rectangle exactly covers its display
// (no menubar/dock inset), the signature of a native-fullscreen window.
static BOOL isFullscreenRect(uint32_t disp, CGFloat x, CGFloat y, CGFloat w, CGFloat h) {
  if (disp == 0) return NO;
  CGRect db = CGDisplayBounds((CGDirectDisplayID)disp);
  return fabs(x - db.origin.x) < 2 && fabs(y - db.origin.y) < 2 &&
         fabs(w - db.size.width) < 2 && fabs(h - db.size.height) < 2;
}

// ---- Space helpers ----

// buildWindowSpaceMap returns wid -> Space id for every window the
// WindowServer reports on any Space. The resolution is inverted versus the
// obvious per-window CGSCopySpacesForWindows call because that call returns an
// empty array for most windows that are NOT on the active Space (observed on
// macOS 26.5), which left off-Space windows tagged space=0. One
// CGSCopyWindowsWithOptionsAndTags query per Space (AltTab's approach) is both
// correct and cheaper: M per-Space calls instead of N per-window calls.
// Returns nil when the private API yields nothing, so Space data degrades to 0
// ("unknown", filters stay inert) rather than misbehaving.
static NSDictionary<NSNumber *, NSNumber *> *buildWindowSpaceMap(CGSConnectionID cid) {
  if (cid == 0) return nil;
  CFArrayRef raw = CGSCopyManagedDisplaySpaces(cid);
  if (raw == NULL) return nil;
  NSArray *displays = (__bridge_transfer NSArray *)raw;
  NSMutableDictionary *map = [NSMutableDictionary dictionary];
  for (NSDictionary *display in displays) {
    for (NSDictionary *space in display[@"Spaces"]) {
      NSNumber *sid = space[@"id64"];
      if (sid == nil) continue;
      uint64_t setTags = 0, clearTags = 0;
      CFArrayRef wins = CGSCopyWindowsWithOptionsAndTags(
          cid, 0, (__bridge CFArrayRef)@[ sid ], 0x7, &setTags, &clearTags);
      if (wins == NULL) continue;
      NSArray *wids = (__bridge_transfer NSArray *)wins;
      for (NSNumber *w in wids) {
        if (map[w] == nil) map[w] = sid; // first Space wins
      }
    }
  }
  return map;
}

// ---- Minimized detection ----

// minimizedWindowIDs returns the set of CGWindowIDs that are AX-minimized. It
// scans regular apps' AX windows (requires Accessibility); without that grant it
// returns an empty set and minimized windows simply aren't flagged.
static NSSet<NSNumber *> *minimizedWindowIDs(void) {
  NSMutableSet *set = [NSMutableSet set];
  if (!AXIsProcessTrusted()) return set;
  for (NSRunningApplication *app in [[NSWorkspace sharedWorkspace] runningApplications]) {
    if (app.activationPolicy != NSApplicationActivationPolicyRegular) continue;
    pid_t pid = app.processIdentifier;
    AXUIElementRef axApp = AXUIElementCreateApplication(pid);
    if (!axApp) continue;
    CFArrayRef wins = NULL;
    if (AXUIElementCopyAttributeValue(axApp, kAXWindowsAttribute, (CFTypeRef *)&wins) == kAXErrorSuccess && wins) {
      for (CFIndex i = 0; i < CFArrayGetCount(wins); i++) {
        AXUIElementRef w = (AXUIElementRef)CFArrayGetValueAtIndex(wins, i);
        CFBooleanRef minRef = NULL;
        if (AXUIElementCopyAttributeValue(w, kAXMinimizedAttribute, (CFTypeRef *)&minRef) == kAXErrorSuccess && minRef) {
          BOOL mini = CFBooleanGetValue(minRef);
          CFRelease(minRef);
          if (mini) {
            CGWindowID wid = 0;
            if (_AXUIElementGetWindow(w, &wid) == kAXErrorSuccess && wid != 0) {
              [set addObject:@(wid)];
            }
          }
        }
      }
      CFRelease(wins);
    }
    CFRelease(axApp);
  }
  return set;
}

// ---- Window enumeration ----

char *ot_list_windows_json(void) {
  @autoreleasepool {
    // Omit kCGWindowListOptionOnScreenOnly so windows on other Spaces and
    // minimized windows are included; we tag on/off-screen and minimized below.
    CGWindowListOption opt = kCGWindowListExcludeDesktopElements;
    CFArrayRef list = CGWindowListCopyWindowInfo(opt, kCGNullWindowID);
    NSMutableArray *out = [NSMutableArray array];
    NSArray *windows = (__bridge_transfer NSArray *)list;

    NSSet<NSNumber *> *minimized = minimizedWindowIDs();
    CGSConnectionID cid = CGSMainConnectionID();
    NSDictionary<NSNumber *, NSNumber *> *spaceMap = buildWindowSpaceMap(cid);

    int z = 0; // z-order index among included windows (0 == frontmost)
    for (NSDictionary *info in windows) {
      NSNumber *layer = info[(__bridge NSString *)kCGWindowLayer];
      if (layer == nil || [layer intValue] != 0) continue; // only normal windows

      NSNumber *wid = info[(__bridge NSString *)kCGWindowNumber];
      NSNumber *pid = info[(__bridge NSString *)kCGWindowOwnerPID];
      NSString *owner = info[(__bridge NSString *)kCGWindowOwnerName];
      NSString *name = info[(__bridge NSString *)kCGWindowName];
      NSDictionary *bounds = info[(__bridge NSString *)kCGWindowBounds];
      NSNumber *onscreenNum = info[(__bridge NSString *)kCGWindowIsOnscreen];
      BOOL onscreen = onscreenNum != nil && [onscreenNum boolValue];

      NSString *bundle = @"";
      BOOL hidden = NO;
      if (pid != nil) {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:[pid intValue]];
        if (app.bundleIdentifier != nil) bundle = app.bundleIdentifier;
        hidden = app.isHidden;
      }

      CGFloat bx = [bounds[@"X"] doubleValue];
      CGFloat by = [bounds[@"Y"] doubleValue];
      CGFloat bw = [bounds[@"Width"] doubleValue];
      CGFloat bh = [bounds[@"Height"] doubleValue];
      uint32_t screen = displayForPoint(bx + bw / 2, by + bh / 2);
      BOOL fullscreen = isFullscreenRect(screen, bx, by, bw, bh);
      BOOL isMin = wid != nil && [minimized containsObject:wid];
      uint64_t space = wid != nil ? [spaceMap[wid] unsignedLongLongValue] : 0;

      [out addObject:@{
        @"id": wid ?: @0,
        @"pid": pid ?: @0,
        @"app": owner ?: @"",
        @"bundle": bundle,
        @"title": name ?: @"",
        @"x": @(bx), @"y": @(by), @"w": @(bw), @"h": @(bh),
        @"onscreen": @(onscreen),
        @"minimized": @(isMin),
        @"hidden": @(hidden),
        @"fullscreen": @(fullscreen),
        @"screen": @(screen),
        @"space": @(space),
        @"zorder": @(z),
      }];
      z++;
    }

    NSData *json = [NSJSONSerialization dataWithJSONObject:out options:0 error:nil];
    NSString *s = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
    return copyCString(s);
  }
}

// windowIDArray builds the pointer-sized value array CoreGraphics expects.
// CGWindowID itself is only 32 bits, so passing its address as const void **
// makes CFArrayCreate read past the integer on 64-bit systems.
static CFArrayRef windowIDArray(uint32_t wid) {
  const void *value = (const void *)(uintptr_t)wid;
  return CFArrayCreate(NULL, &value, 1, NULL);
}

uintptr_t ot_window_id_array_value(uint32_t wid) {
  CFArrayRef arr = windowIDArray(wid);
  uintptr_t value = (uintptr_t)CFArrayGetValueAtIndex(arr, 0);
  CFRelease(arr);
  return value;
}

// ot_window_pid resolves the owning pid of a single window id via a targeted
// CGWindowList query (no AX/Space enrichment), keeping the action path fast.
int ot_window_pid(uint32_t wid) {
  @autoreleasepool {
    CFArrayRef arr = windowIDArray(wid);
    CFArrayRef list = CGWindowListCreateDescriptionFromArray(arr);
    CFRelease(arr);
    int pid = 0;
    if (list) {
      NSArray *windows = (__bridge_transfer NSArray *)list;
      if (windows.count > 0) {
        NSNumber *p = windows[0][(__bridge NSString *)kCGWindowOwnerPID];
        if (p != nil) pid = [p intValue];
      }
    }
    return pid;
  }
}

// ---- Frontmost-app tracking ----

// The switcher overlay never activates the app, but the preferences window and
// overlay clicks can make option-tab itself frontmost. The active-app scope
// filter must keep seeing the *real* active app in those moments, so we track
// the most recent activated app that isn't us via NSWorkspace notifications.
static pid_t gLastRealFrontPid = 0;
static BOOL gFrontObserverInstalled = NO;

// From the focus-observation section below; the activation observer doubles
// as the lazy (re)install + report hook once focus tracking is on.
static BOOL gFocusObserverStarted; // tentative; defined below
static void otInstallAXObserver(pid_t pid);
static void otReportFocusedWindowOfApp(pid_t pid);

static void otInstallFrontObserver(void) {
  if (gFrontObserverInstalled) return;
  gFrontObserverInstalled = YES;
  pid_t self = [[NSProcessInfo processInfo] processIdentifier];
  NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
  if (front && front.processIdentifier != self) {
    gLastRealFrontPid = front.processIdentifier;
    goHotkeyFrontAppChanged(front.bundleIdentifier.UTF8String,
                            front.localizedName.UTF8String);
  }
  [[[NSWorkspace sharedWorkspace] notificationCenter]
      addObserverForName:NSWorkspaceDidActivateApplicationNotification
                  object:nil
                   queue:nil
              usingBlock:^(NSNotification *note) {
    NSRunningApplication *app = note.userInfo[NSWorkspaceApplicationKey];
    if (app && app.processIdentifier != self) {
      gLastRealFrontPid = app.processIdentifier;
      goHotkeyFrontAppChanged(app.bundleIdentifier.UTF8String,
                              app.localizedName.UTF8String);
      // Focus tracking rides on the same notification (gated so the eager
      // install from ot_active_app_pid never emits before Go is listening):
      // (re)install the app's AX observer lazily — covers apps launched after
      // start and apps that were not AX-ready earlier — and report the window
      // this activation focused.
      if (gFocusObserverStarted) {
        otInstallAXObserver(app.processIdentifier);
        otReportFocusedWindowOfApp(app.processIdentifier);
      }
    }
  }];
}

int ot_active_app_pid(void) {
  @autoreleasepool {
    otInstallFrontObserver();
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (!app) return (int)gLastRealFrontPid;
    if (app.processIdentifier == [[NSProcessInfo processInfo] processIdentifier]) {
      return (int)gLastRealFrontPid; // we are frontmost (prefs/click): report the real app
    }
    return (int)app.processIdentifier;
  }
}

// ---- Focus observation (MRU feed) ----

// Real focus changes (clicks, the Dock, Spotlight, the OS's own ⌘Tab) must
// reach the Go MRU tracker, or "recently focused" ordering only reflects
// switches made through the overlay. Two sources cover the space, as AltTab
// does: NSWorkspaceDidActivateApplicationNotification for app-level switches
// (piggybacked on the front observer above) and a per-app AXObserver on
// kAXFocusedWindowChangedNotification for window switches within an app. Both
// resolve the focused window to its CGWindowID and hand it to goFocusEvent,
// which appends to the Go-side event queue and never blocks.
//
// All state here is touched only on the main thread — NSWorkspace notification
// blocks and AXObserver callbacks are delivered on the main run loop, and
// ot_focus_observer_start dispatches its body there — so no locking is needed.
extern void goFocusEvent(uint32_t wid);

static BOOL gFocusObserverStarted = NO; // declared above for the front observer
static NSMutableDictionary<NSNumber *, id> *gAXObservers = nil; // pid → AXObserverRef

// otReportFocusedWindowOfApp resolves pid's focused window and reports it.
// Failure is silent: an app that is not AX-ready yet (just launched) gets its
// first real focus reported by its AXObserver instead.
static void otReportFocusedWindowOfApp(pid_t pid) {
  if (!AXIsProcessTrusted()) return;
  AXUIElementRef axApp = AXUIElementCreateApplication(pid);
  if (!axApp) return;
  // A hung app must not stall the main thread for the default 6s.
  AXUIElementSetMessagingTimeout(axApp, 1.0);
  AXUIElementRef win = NULL;
  if (AXUIElementCopyAttributeValue(axApp, kAXFocusedWindowAttribute, (CFTypeRef *)&win) == kAXErrorSuccess && win) {
    CGWindowID wid = 0;
    if (_AXUIElementGetWindow(win, &wid) == kAXErrorSuccess && wid != 0) {
      OTLOG("focusobs: report pid=%d wid=%u\n", pid, wid);
      goFocusEvent(wid);
    }
    CFRelease(win);
  }
  CFRelease(axApp);
}

// otAXFocusCallback fires on kAXFocusedWindowChangedNotification; element IS
// the newly focused window, so a single _AXUIElementGetWindow resolves it — no
// second AX round trip. refcon carries the pid as a fallback for elements that
// fail to resolve (rare: some AX-hostile apps hand back odd elements).
static void otAXFocusCallback(AXObserverRef obs, AXUIElementRef element,
                              CFStringRef notification, void *refcon) {
  CGWindowID wid = 0;
  if (_AXUIElementGetWindow(element, &wid) == kAXErrorSuccess && wid != 0) {
    OTLOG("focusobs: ax pid=%d wid=%u\n", (int)(intptr_t)refcon, wid);
    goFocusEvent(wid);
    return;
  }
  otReportFocusedWindowOfApp((pid_t)(intptr_t)refcon);
}

// otInstallAXObserver subscribes to pid's focused-window changes. Idempotent.
// Failure (app not AX-ready yet, AXObserverCreate error) is a silent skip: the
// activation observer retries on the app's next activation, no timers needed.
static void otInstallAXObserver(pid_t pid) {
  if (!AXIsProcessTrusted()) return;
  if (pid == [[NSProcessInfo processInfo] processIdentifier]) return;
  if (!gAXObservers) gAXObservers = [NSMutableDictionary dictionary];
  if (gAXObservers[@(pid)]) return;
  AXObserverRef obs = NULL;
  if (AXObserverCreate(pid, otAXFocusCallback, &obs) != kAXErrorSuccess || !obs) return;
  AXUIElementRef axApp = AXUIElementCreateApplication(pid);
  if (!axApp) { CFRelease(obs); return; }
  AXUIElementSetMessagingTimeout(axApp, 1.0);
  AXError err = AXObserverAddNotification(obs, axApp, kAXFocusedWindowChangedNotification, (void *)(intptr_t)pid);
  CFRelease(axApp); // the registration lives in the AX server, not this token
  if (err != kAXErrorSuccess) { CFRelease(obs); return; }
  CFRunLoopAddSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(obs), kCFRunLoopDefaultMode);
  gAXObservers[@(pid)] = (__bridge_transfer id)obs; // dict owns Create's +1
  OTLOG("focusobs: installed pid=%d\n", pid);
}

static void otRemoveAXObserver(pid_t pid) {
  id boxed = gAXObservers[@(pid)];
  if (!boxed) return;
  AXObserverRef obs = (__bridge AXObserverRef)boxed;
  CFRunLoopRemoveSource(CFRunLoopGetMain(), AXObserverGetRunLoopSource(obs), kCFRunLoopDefaultMode);
  [gAXObservers removeObjectForKey:@(pid)]; // ARC releases the observer
  OTLOG("focusobs: removed pid=%d\n", pid);
}

void ot_focus_observer_start(void) {
  // Main-queue machinery, unlike ot_hotkey_start's dedicated tap thread: the
  // AX observer run-loop sources must live where the callbacks are serviced.
  dispatch_async(dispatch_get_main_queue(), ^{
    if (gFocusObserverStarted) return;
    gFocusObserverStarted = YES;
    otInstallFrontObserver(); // shared with active-app tracking; idempotent
    // Observe every running regular app now; apps launched later are picked
    // up lazily by the activation block in otInstallFrontObserver.
    for (NSRunningApplication *app in [[NSWorkspace sharedWorkspace] runningApplications]) {
      if (app.activationPolicy != NSApplicationActivationPolicyRegular) continue;
      otInstallAXObserver(app.processIdentifier);
    }
    [[[NSWorkspace sharedWorkspace] notificationCenter]
        addObserverForName:NSWorkspaceDidTerminateApplicationNotification
                    object:nil
                     queue:nil
                usingBlock:^(NSNotification *note) {
      NSRunningApplication *app = note.userInfo[NSWorkspaceApplicationKey];
      if (app) otRemoveAXObserver(app.processIdentifier);
    }];
    // Seed the MRU with the currently focused window so the first activation
    // after launch already has one true recency entry (the alternative —
    // seeding the whole z-order — would erase the tracked/untracked split the
    // ordering relies on).
    if (gLastRealFrontPid != 0) otReportFocusedWindowOfApp(gLastRealFrontPid);
  });
}

// ---- Environment: Spaces & screens ----

uint64_t ot_active_space(void) {
  CGSConnectionID cid = CGSMainConnectionID();
  if (cid == 0) return 0;
  return CGSGetActiveSpace(cid);
}


uint32_t ot_active_screen(void) {
  // The screen owning the focused window of the frontmost app, else main.
  @autoreleasepool {
    NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (front != nil && AXIsProcessTrusted()) {
      AXUIElementRef axApp = AXUIElementCreateApplication(front.processIdentifier);
      if (axApp) {
        AXUIElementRef win = NULL;
        uint32_t screen = 0;
        if (AXUIElementCopyAttributeValue(axApp, kAXFocusedWindowAttribute, (CFTypeRef *)&win) == kAXErrorSuccess && win) {
          AXValueRef posRef = NULL, sizeRef = NULL;
          CGPoint pos = {0, 0};
          CGSize size = {0, 0};
          if (AXUIElementCopyAttributeValue(win, kAXPositionAttribute, (CFTypeRef *)&posRef) == kAXErrorSuccess && posRef) {
            AXValueGetValue(posRef, kAXValueCGPointType, &pos);
            CFRelease(posRef);
          }
          if (AXUIElementCopyAttributeValue(win, kAXSizeAttribute, (CFTypeRef *)&sizeRef) == kAXErrorSuccess && sizeRef) {
            AXValueGetValue(sizeRef, kAXValueCGSizeType, &size);
            CFRelease(sizeRef);
          }
          screen = displayForPoint(pos.x + size.width / 2, pos.y + size.height / 2);
          CFRelease(win);
        }
        CFRelease(axApp);
        if (screen != 0) return screen;
      }
    }
    return (uint32_t)CGMainDisplayID();
  }
}

uint32_t ot_cursor_screen(void) {
  @autoreleasepool {
    NSPoint p = [NSEvent mouseLocation]; // Cocoa: origin bottom-left of main screen
    CGFloat mainH = CGDisplayBounds(CGMainDisplayID()).size.height;
    return displayForPoint(p.x, mainH - p.y); // convert to CG top-left origin
  }
}

char *ot_screens_json(void) {
  @autoreleasepool {
    NSMutableArray *out = [NSMutableArray array];
    for (NSScreen *screen in [NSScreen screens]) {
      NSNumber *num = screen.deviceDescription[@"NSScreenNumber"];
      uint32_t did = num != nil ? [num unsignedIntValue] : 0;
      CGRect full = CGDisplayBounds((CGDirectDisplayID)did);
      NSRect vis = screen.visibleFrame; // Cocoa coords (origin bottom-left)
      CGFloat mainH = CGDisplayBounds(CGMainDisplayID()).size.height;
      [out addObject:@{
        @"id": @(did),
        @"main": did == CGMainDisplayID() ? @YES : @NO,
        @"x": @(full.origin.x), @"y": @(full.origin.y),
        @"w": @(full.size.width), @"h": @(full.size.height),
        // Convert visibleFrame to CG top-left origin for consistency.
        @"vx": @(vis.origin.x),
        @"vy": @(mainH - vis.origin.y - vis.size.height),
        @"vw": @(vis.size.width), @"vh": @(vis.size.height),
      }];
    }
    NSData *json = [NSJSONSerialization dataWithJSONObject:out options:0 error:nil];
    NSString *s = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
    return copyCString(s);
  }
}

// ---- Permissions ----

int ot_perm_accessibility(void) { return AXIsProcessTrusted() ? 1 : 0; }

int ot_perm_screen_recording(void) {
  if (@available(macOS 10.15, *)) {
    return CGPreflightScreenCaptureAccess() ? 1 : 0;
  }
  return 1;
}

void ot_request_accessibility(void) {
  @autoreleasepool {
    NSDictionary *opts = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES};
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)opts);
  }
}

void ot_request_screen_recording(void) {
  if (@available(macOS 10.15, *)) {
    CGRequestScreenCaptureAccess();
  }
}

void ot_open_privacy_settings(int kind) {
  @autoreleasepool {
    NSString *anchor = kind == 1 ? @"Privacy_ScreenCapture" : @"Privacy_Accessibility";
    NSString *url = [@"x-apple.systempreferences:com.apple.preference.security?" stringByAppendingString:anchor];
    [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:url]];
  }
}

// ---- AX window lookup + actions ----

static AXUIElementRef copyAXWindow(uint32_t wid, int pid) {
  AXUIElementRef app = AXUIElementCreateApplication(pid);
  if (app == NULL) return NULL;
  CFArrayRef windows = NULL;
  AXError err = AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, (CFTypeRef *)&windows);
  AXUIElementRef found = NULL;
  if (err == kAXErrorSuccess && windows != NULL) {
    CFIndex n = CFArrayGetCount(windows);
    for (CFIndex i = 0; i < n; i++) {
      AXUIElementRef w = (AXUIElementRef)CFArrayGetValueAtIndex(windows, i);
      CGWindowID cgid = 0;
      if (_AXUIElementGetWindow(w, &cgid) == kAXErrorSuccess && cgid == wid) {
        found = (AXUIElementRef)CFRetain(w);
        break;
      }
    }
    CFRelease(windows);
  }
  CFRelease(app);
  return found;
}

// otMakeKeyWindow posts the synthetic SkyLight event record that makes wid its
// app's key window after the process is fronted (AltTab's current technique).
// Only a left-mouse-DOWN is posted: on macOS 26.5 the down alone transfers key
// focus, while a full down/up pair forms a real click that can activate window
// content. The window-relative point at 0x20 is aimed far past any window's
// bottom-right corner so an app that sanitizes the coordinate can't land it on
// real UI (the old NaN point from memset 0xff was sanitized to (0,0) by some
// apps, clicking their top-left control); the event targets the window by id
// at 0x3c, not by this point. The buffer is 0x100 while the declared record
// length at 0x04 stays 0xf8: macOS 14.7.4+'s CGSEncodeEventRecord reads past
// the record and can crash on a tight allocation.
static void otMakeKeyWindow(ProcessSerialNumber psn, uint32_t wid) {
  uint8_t bytes[0x100] = {0};
  CGPoint point = CGPointMake(300000, 300000);
  bytes[0x04] = 0xf8; // declared record length
  bytes[0x08] = 0x01; // kCGEventLeftMouseDown
  bytes[0x3a] = 0x10;
  memcpy(&bytes[0x20], &point, sizeof(CGPoint));
  memcpy(&bytes[0x3c], &wid, sizeof(uint32_t));
  SLPSPostEventRecordTo(&psn, bytes);
}

// copyAXWindowByBruteForce resolves the AX element for a window that a fresh
// kAXWindowsAttribute scan cannot see — a window on another Space. There is no
// wid→element API, so it enumerates the app's AXUIElementID space through
// remote tokens (AltTab's windowByBruteForce) until it finds the element that
// (a) resolves to wid and (b) has the AXWindow role — descendants (buttons,
// tab bars) resolve to the containing window's wid too, so the role gate keeps
// scanning past them to the window root. Time-bounded: the id space is 64-bit
// and a long-lived app's windows can sit at high ids.
static AXUIElementRef copyAXWindowByBruteForce(uint32_t wid, int pid) {
  uint8_t token[20] = {0};
  int32_t pid32 = (int32_t)pid;
  int32_t magic = 0x636f636f; // 'coco'
  memcpy(token, &pid32, sizeof(pid32));
  memcpy(token + 8, &magic, sizeof(magic));
  CFAbsoluteTime start = CFAbsoluteTimeGetCurrent();
  CFAbsoluteTime deadline = start + 0.25; // wall-clock budget, AltTab parity
  for (uint64_t axid = 0;; axid++) {
    memcpy(token + 12, &axid, sizeof(axid));
    CFDataRef data = CFDataCreate(NULL, token, sizeof(token));
    AXUIElementRef candidate = _AXUIElementCreateWithRemoteToken(data);
    CFRelease(data);
    if (candidate) {
      CGWindowID cgid = 0;
      if (_AXUIElementGetWindow(candidate, &cgid) == kAXErrorSuccess && cgid == wid) {
        CFStringRef role = NULL;
        if (AXUIElementCopyAttributeValue(candidate, kAXRoleAttribute, (CFTypeRef *)&role) == kAXErrorSuccess && role) {
          BOOL isRoot = CFEqual(role, kAXWindowRole);
          CFRelease(role);
          if (isRoot) {
            OTLOG("focus: brute-force found root at id=%llu after %.0fms\n",
                  (unsigned long long)axid, (CFAbsoluteTimeGetCurrent() - start) * 1000.0);
            return candidate;
          }
        }
      }
      CFRelease(candidate);
    }
    if (CFAbsoluteTimeGetCurrent() > deadline) {
      OTLOG("focus: brute-force timeout at id=%llu\n", (unsigned long long)axid);
      return NULL;
    }
  }
}

int ot_focus_window(uint32_t wid, int pid) {
  @autoreleasepool {
    // Front the specific window via SkyLight so macOS navigates to its Space
    // (works cross-Space without pulling the window to the current desktop,
    // which the old activate+raise did when the space switch was a no-op).
    ProcessSerialNumber psn;
    BOOL fronted = NO;
    if (GetProcessForPID(pid, &psn) == noErr) {
      _SLPSSetFrontProcessWithOptions(&psn, wid, 0x200 /* kCPSUserGenerated */);
      otMakeKeyWindow(psn, wid);
      fronted = YES;
    }
    OTLOG("focus: wid=%u pid=%d fronted=%d active-space=%llu\n", wid, pid, fronted,
          (unsigned long long)ot_active_space());
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) {
      // Cross-Space: the fresh kAXWindowsAttribute scan above only lists
      // current-Space and minimized windows, so resolve the element by
      // remote-token brute force instead. The raise below is what makes macOS
      // navigate to the window's Space.
      w = copyAXWindowByBruteForce(wid, pid);
      OTLOG("focus: brute-force fallback %s\n", w != NULL ? "hit" : "miss");
    }
    BOOL raised = NO;
    if (w != NULL) {
      AXError raiseErr = AXUIElementPerformAction(w, kAXRaiseAction);
      AXUIElementSetAttributeValue(w, kAXMainAttribute, kCFBooleanTrue);
      OTLOG("focus: raise err=%d\n", (int)raiseErr);
      CFRelease(w);
      raised = YES;
    }
    if (!fronted) {
      // The pid could not be resolved for SkyLight fronting (e.g. an accessory
      // process): fall back to activating the app so the window still gets
      // focus. This never switches Spaces, so it can't pull the window over.
      NSRunningApplication *app =
          [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
      [app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
    }
    return (fronted || raised) ? 1 : 0;
  }
}

int ot_close_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) return 0;
    CFTypeRef button = NULL;
    int ok = 0;
    if (AXUIElementCopyAttributeValue(w, kAXCloseButtonAttribute, &button) == kAXErrorSuccess && button) {
      ok = (AXUIElementPerformAction((AXUIElementRef)button, kAXPressAction) == kAXErrorSuccess);
      CFRelease(button);
    }
    CFRelease(w);
    return ok;
  }
}

int ot_minimize_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) return 0;
    // Toggle, so the same key deminimizes a minimized window (AltTab parity).
    CFBooleanRef cur = NULL;
    BOOL isMin = NO;
    if (AXUIElementCopyAttributeValue(w, kAXMinimizedAttribute, (CFTypeRef *)&cur) == kAXErrorSuccess && cur) {
      isMin = CFBooleanGetValue(cur);
      CFRelease(cur);
    }
    int ok = (AXUIElementSetAttributeValue(w, kAXMinimizedAttribute, isMin ? kCFBooleanFalse : kCFBooleanTrue) == kAXErrorSuccess);
    CFRelease(w);
    return ok;
  }
}

int ot_fullscreen_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) return 0;
    // AXFullScreen is the (semi-private) standard-window fullscreen attribute.
    CFBooleanRef cur = NULL;
    BOOL isFs = NO;
    if (AXUIElementCopyAttributeValue(w, CFSTR("AXFullScreen"), (CFTypeRef *)&cur) == kAXErrorSuccess && cur) {
      isFs = CFBooleanGetValue(cur);
      CFRelease(cur);
    }
    int ok = (AXUIElementSetAttributeValue(w, CFSTR("AXFullScreen"), isFs ? kCFBooleanFalse : kCFBooleanTrue) == kAXErrorSuccess);
    CFRelease(w);
    return ok;
  }
}

int ot_quit_app(int pid) {
  @autoreleasepool {
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    return (app && [app terminate]) ? 1 : 0;
  }
}

int ot_hide_app(int pid) {
  @autoreleasepool {
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    return (app && [app hide]) ? 1 : 0;
  }
}

// ---- Thumbnail ----

// captureWindowPNG snapshots a single window via ScreenCaptureKit (the
// replacement for CGWindowListCreateImage, removed in macOS 15), scaled so its
// larger side is at most maxpx. Returns nil if Screen Recording is not granted
// or the window cannot be captured. Synchronous via dispatch semaphores.
static NSData *captureWindowPNG(uint32_t wid, int maxpx) {
  if (@available(macOS 14.0, *)) {
    if (maxpx <= 0) maxpx = 256;

    __block SCShareableContent *content = nil;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [SCShareableContent getShareableContentExcludingDesktopWindows:NO
                                              onScreenWindowsOnly:NO
                                                completionHandler:^(SCShareableContent *c, NSError *e) {
      (void)e;
      content = c;
      dispatch_semaphore_signal(sem);
    }];
    if (dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2 * NSEC_PER_SEC))) != 0) return nil;
    if (content == nil) return nil;

    SCWindow *target = nil;
    for (SCWindow *w in content.windows) {
      if (w.windowID == wid) { target = w; break; }
    }
    if (target == nil) return nil;

    CGFloat fw = target.frame.size.width, fh = target.frame.size.height;
    if (fw < 1 || fh < 1) return nil;
    CGFloat scale = (fw > fh) ? (CGFloat)maxpx / fw : (CGFloat)maxpx / fh;
    if (scale > 1.0) scale = 1.0;

    SCContentFilter *filter = [[SCContentFilter alloc] initWithDesktopIndependentWindow:target];
    SCStreamConfiguration *cfg = [[SCStreamConfiguration alloc] init];
    cfg.width = (size_t)(fw * scale);
    cfg.height = (size_t)(fh * scale);
    cfg.showsCursor = NO;
    cfg.ignoreShadowsSingleWindow = YES;

    __block CGImageRef img = NULL;
    dispatch_semaphore_t sem2 = dispatch_semaphore_create(0);
    [SCScreenshotManager captureImageWithFilter:filter
                                  configuration:cfg
                              completionHandler:^(CGImageRef image, NSError *e) {
      (void)e;
      if (image) img = (CGImageRef)CGImageRetain(image);
      dispatch_semaphore_signal(sem2);
    }];
    if (dispatch_semaphore_wait(sem2, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(2 * NSEC_PER_SEC))) != 0) return nil;
    if (img == NULL) return nil;

    NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:img];
    CGImageRelease(img);
    return [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
  }
  return nil;
}

char *ot_thumbnail_png_base64(uint32_t wid, int maxpx) {
  @autoreleasepool {
    NSData *png = captureWindowPNG(wid, maxpx);
    if (png == nil) return copyCString(@"");
    return copyCString([png base64EncodedStringWithOptions:0]);
  }
}

char *ot_thumbnail_dataurl(uint32_t wid, int maxpx) {
  @autoreleasepool {
    NSData *png = captureWindowPNG(wid, maxpx);
    if (png == nil) return copyCString(@"");
    NSString *b64 = [png base64EncodedStringWithOptions:0];
    return copyCString([@"data:image/png;base64," stringByAppendingString:b64]);
  }
}

// ---- App icon ----

char *ot_app_icon_png_base64(int pid, int maxpx) {
  @autoreleasepool {
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    NSImage *icon = app.icon;
    if (icon == nil) return copyCString(@"");
    if (maxpx <= 0) maxpx = 64;

    NSSize target = NSMakeSize(maxpx, maxpx);
    NSImage *resized = [[NSImage alloc] initWithSize:target];
    [resized lockFocus];
    [icon drawInRect:NSMakeRect(0, 0, target.width, target.height)
            fromRect:NSZeroRect
           operation:NSCompositingOperationCopy
            fraction:1.0];
    [resized unlockFocus];

    CGImageRef cg = [resized CGImageForProposedRect:NULL context:nil hints:nil];
    if (cg == NULL) return copyCString(@"");
    NSBitmapImageRep *rep = [[NSBitmapImageRep alloc] initWithCGImage:cg];
    NSData *png = [rep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
    if (png == nil) return copyCString(@"");
    NSString *b64 = [png base64EncodedStringWithOptions:0];
    return copyCString([@"data:image/png;base64," stringByAppendingString:b64]);
  }
}

// ---- Login item (SMAppService) ----

// ---- App presentation (activation policy helpers) ----

void ot_hide_dock_icon(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
  });
}

void ot_activate_prefs(void) {
  otInstallFrontObserver();
  @autoreleasepool {
    NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
    if (front && front.processIdentifier != [[NSProcessInfo processInfo] processIdentifier]) {
      gLastRealFrontPid = front.processIdentifier;
      goHotkeyFrontAppChanged(front.bundleIdentifier.UTF8String,
                              front.localizedName.UTF8String);
    }
  }
  dispatch_async(dispatch_get_main_queue(), ^{
    [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
    [NSApp activateIgnoringOtherApps:YES];
  });
}

void ot_app_hide(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    if ([NSApp isActive]) [NSApp hide:nil];
  });
}

// ot_window_fit_screen resizes the overlay window to the visible frame of the
// screen with the given display id (0/unknown = the window's current screen),
// so the switcher panel appears on the screen the Placement setting chose and
// can lay out against its whole area. The window is fully transparent, so its
// actual bounds are invisible. Dispatched to the main queue.
void ot_window_fit_screen(void *win, uint32_t displayID) {
  dispatch_async(dispatch_get_main_queue(), ^{
    NSWindow *w = (__bridge NSWindow *)win;
    if (w == nil) return;
    // Wails applies Mac.DisableShadow to a hidden window from a
    // WindowDidBecomeKey handler, which never fires for this never-activated
    // window — so the native shadow stays on. On macOS 26 that shadow draws a
    // dark rounded rim around the panel (the transparent window's opaque
    // content), visible as a black outline on dark wallpapers. Re-assert it
    // here on every show, like App.Show already does for AlwaysOnTop.
    w.hasShadow = NO;
    NSScreen *s = w.screen ?: [NSScreen mainScreen];
    if (displayID != 0) {
      for (NSScreen *cand in [NSScreen screens]) {
        NSNumber *num = cand.deviceDescription[@"NSScreenNumber"];
        if (num != nil && [num unsignedIntValue] == displayID) {
          s = cand;
          break;
        }
      }
    }
    if (s == nil) return;
    [w setFrame:s.visibleFrame display:YES];
  });
}

// ot_haptic_tick performs a subtle trackpad tap (selection feedback).
void ot_haptic_tick(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    [[NSHapticFeedbackManager defaultPerformer]
        performFeedbackPattern:NSHapticFeedbackPatternAlignment
               performanceTime:NSHapticFeedbackPerformanceTimeNow];
  });
}

void ot_warp_cursor(uint32_t wid) {
  CFArrayRef arr = CGWindowListCopyWindowInfo(kCGWindowListOptionIncludingWindow, (CGWindowID)wid);
  if (arr == NULL) return;
  if (CFArrayGetCount(arr) > 0) {
    NSDictionary *info = (__bridge NSDictionary *)CFArrayGetValueAtIndex(arr, 0);
    CGRect r = CGRectZero;
    CFDictionaryRef bounds = (__bridge CFDictionaryRef)info[(id)kCGWindowBounds];
    if (bounds != NULL && CGRectMakeWithDictionaryRepresentation(bounds, &r)) {
      CGPoint center = CGPointMake(r.origin.x + r.size.width / 2.0, r.origin.y + r.size.height / 2.0);
      CGWarpMouseCursorPosition(center);
      CGAssociateMouseAndMouseCursorPosition(true);
    }
  }
  CFRelease(arr);
}

int ot_login_item_enabled(void) {
  if (@available(macOS 13.0, *)) {
    return SMAppService.mainAppService.status == SMAppServiceStatusEnabled ? 1 : 0;
  }
  return 0;
}

int ot_login_item_set(int enabled) {
  if (@available(macOS 13.0, *)) {
    NSError *err = nil;
    BOOL ok;
    if (enabled) {
      ok = [SMAppService.mainAppService registerAndReturnError:&err];
    } else {
      ok = [SMAppService.mainAppService unregisterAndReturnError:&err];
    }
    return ok ? 1 : 0;
  }
  return 0;
}

// ---- Hotkey engine (CGEventTap on a dedicated thread) ----

extern void goHotkeyCaptured(uint64_t modflags, uint16_t keycode);

typedef struct {
  int id;
  uint64_t modflags; // exact modifier mask used to match the key press
  uint64_t holdMask; // base modifiers whose release ends the session
  uint16_t keycode;
  int withShift;     // 1 if this registration is the shift/reverse variant
  int used;
} OTChord;

#define OT_MAX_CHORDS 32
static OTChord gChords[OT_MAX_CHORDS];
static pthread_mutex_t gChordLock = PTHREAD_MUTEX_INITIALIZER;
static CFMachPortRef gTap = NULL;
static CFRunLoopSourceRef gSource = NULL;
static CFRunLoopRef gRunLoop = NULL;
static int gActive = 0;          // chord currently held (activate fired, release pending)
static uint64_t gHoldMask = 0;   // modifiers held during the active session
static int gCaptureMode = 0;     // one-shot chord recording for the prefs UI
// gSwitcherOpen mirrors the Go-side overlay visibility (set via
// ot_hotkey_set_open). While 1, every key press is consumed and forwarded so
// typing never leaks into the previously active app — the overlay window never
// becomes key (the app is not activated), so the tap is its only keyboard
// source. Written from the Go thread, read on the tap thread.
static _Atomic int gSwitcherOpen = 0;

// Go recomputes eligibility when settings or the frontmost app changes. The
// event tap only performs this atomic read: no Go callback, AX query, string
// conversion, or case folding occurs on the latency-sensitive tap thread.
static _Atomic int gHotkeysEligible = 1;

static const uint64_t kModMask = (kCGEventFlagMaskControl | kCGEventFlagMaskAlternate |
                                  kCGEventFlagMaskShift | kCGEventFlagMaskCommand);

void ot_hotkey_set_eligible(int eligible) {
  atomic_store_explicit(&gHotkeysEligible, eligible ? 1 : 0,
                        memory_order_release);
}

static int hotkeyPolicyAllowsFrontApp(void) {
  return (int)atomic_load_explicit(&gHotkeysEligible, memory_order_acquire);
}

int ot_hotkey_decide(uint64_t modflags, uint16_t keycode, int active, int open,
                     int *shortcutID, uint64_t *holdMask) {
  OTChord matched = {0};
  uint64_t flags = modflags & kModMask;

  pthread_mutex_lock(&gChordLock);
  // Explicit chords win over generated shift/reverse variants with the same
  // key and modifier mask, independent of registration order.
  for (int synthetic = 0; synthetic <= 1 && !matched.used; synthetic++) {
    for (int i = 0; i < OT_MAX_CHORDS; i++) {
      if (!gChords[i].used || gChords[i].withShift != synthetic) continue;
      if (gChords[i].keycode != keycode || gChords[i].modflags != flags) continue;
      matched = gChords[i];
      break;
    }
  }
  pthread_mutex_unlock(&gChordLock);

  if (!matched.used) return 0;
  if (!active && !open && !hotkeyPolicyAllowsFrontApp()) return 0;
  if (shortcutID != NULL) *shortcutID = matched.id;
  if (holdMask != NULL) *holdMask = matched.holdMask;
  if (!active) return 1;
  return matched.withShift ? 3 : 2;
}

int ot_hotkey_should_release(uint64_t modflags, uint64_t holdMask) {
  uint64_t flags = modflags & kModMask;
  return (flags & holdMask) != holdMask;
}

// forwardKey delivers a key press to Go (goKeyEvent) with its text, so the
// frontend can run navigation/actions/type-to-search while the overlay is open.
static void forwardKey(CGEventRef event, uint16_t keycode, uint64_t flags) {
  UniChar buf[16];
  UniCharCount len = 0;
  CGEventKeyboardGetUnicodeString(event, 16, &len, buf);
  char text[64];
  text[0] = '\0';
  if (len > 0) {
    NSString *s = [[NSString alloc] initWithCharacters:buf length:len];
    if (s != nil) {
      strncpy(text, [s UTF8String], sizeof(text) - 1);
      text[sizeof(text) - 1] = '\0';
    }
  }
  goKeyEvent((int)keycode, flags, text);
}

static CGEventRef tapCallback(CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *ctx) {
  (void)proxy;
  (void)ctx;
  if (type == kCGEventTapDisabledByTimeout || type == kCGEventTapDisabledByUserInput) {
    if (gTap) CGEventTapEnable(gTap, true);
    return event;
  }

  uint64_t flags = (uint64_t)CGEventGetFlags(event) & kModMask;

  if (type == kCGEventFlagsChanged) {
    if (gActive && ot_hotkey_should_release(flags, gHoldMask)) {
      gActive = 0;
      goHotkeyEvent(3, 0); // release
    }
    return event;
  }

  if (type == kCGEventKeyDown) {
    uint16_t keycode = (uint16_t)CGEventGetIntegerValueField(event, kCGKeyboardEventKeycode);
    int open = (int)atomic_load_explicit(&gSwitcherOpen, memory_order_relaxed);
    OTLOG("keydown keycode=%u flags=0x%llx active=%d open=%d\n", keycode, (unsigned long long)flags, gActive, open);

    // Chord recording: the prefs recorder armed a one-shot capture. It runs
    // before chord matching so even the switcher's own chord (or Command+Tab,
    // which the webview never sees) can be recorded — and is consumed so it
    // doesn't also trigger the system or the switcher.
    if (gCaptureMode) {
      if (keycode == 53) { // Escape cancels recording
        gCaptureMode = 0;
        goHotkeyCaptured(0, 0xFFFF);
        return NULL;
      }
      if (flags != 0) {
        gCaptureMode = 0;
        goHotkeyCaptured(flags, keycode);
        return NULL;
      }
      return event; // unmodified keys pass through (chords need a modifier)
    }

    if (keycode == 53 && (gActive || open)) { // Escape
      gActive = 0;
      goHotkeyEvent(4, 0); // cancel
      return NULL;         // consume
    }

    int shortcutID = 0;
    uint64_t holdMask = 0;
    int action = ot_hotkey_decide(flags, keycode, gActive, open, &shortcutID,
                                  &holdMask);
    if (action != 0) {
      if (action == 1) {
        gActive = 1;
        gHoldMask = holdMask;
        goHotkeyEvent(0, shortcutID);
      } else if (action == 2) {
        goHotkeyEvent(1, shortcutID);
      } else {
        goHotkeyEvent(2, shortcutID);
      }
      return NULL;
    }

    // Switcher open but no chord matched: consume the key so it never reaches
    // the previously active app, and forward it to the overlay (navigation,
    // window actions, type-to-search).
    if (open) {
      forwardKey(event, keycode, flags);
      return NULL;
    }
  }
  return event;
}

static void *hotkeyThread(void *arg) {
  (void)arg;
  @autoreleasepool {
    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventFlagsChanged);
    // Accessibility may not be effective at launch (first run, or the user
    // grants it after the app starts). A session event tap returns NULL until
    // it is. Retry until it succeeds so granting permission takes effect
    // without restarting the app — the same approach AltTab uses.
    for (int attempt = 0; gTap == NULL; attempt++) {
      gTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault,
                              mask, tapCallback, NULL);
      if (gTap == NULL) {
        if (attempt == 0 || (attempt % 10) == 0) {
          OTLOG("CGEventTapCreate NULL (attempt %d) trusted=%d — waiting for Accessibility\n",
                attempt, AXIsProcessTrusted());
        }
        usleep(500000); // 0.5s
      }
    }
    OTLOG("CGEventTapCreate -> OK (tap active); trusted=%d\n", AXIsProcessTrusted());
    gSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, gTap, 0);
    gRunLoop = CFRunLoopGetCurrent();
    CFRunLoopAddSource(gRunLoop, gSource, kCFRunLoopCommonModes);
    CGEventTapEnable(gTap, true);
    CFRunLoopRun();
  }
  return NULL;
}

int ot_hotkey_start(void) {
  OTLOG("ot_hotkey_start called; trusted=%d\n", AXIsProcessTrusted());
  if (gTap != NULL) return 1;
  // Seed frontmost-app tracking early: Wails activates the app at launch, so
  // from here on frontmostApplication may be us and the observer is the only
  // source of the real active app.
  otInstallFrontObserver();
  pthread_t t;
  if (pthread_create(&t, NULL, hotkeyThread, NULL) != 0) return 0;
  pthread_detach(t);
  return 1;
}

int ot_hotkey_register(int id, uint64_t modflags, uint16_t keycode, int withShift) {
  OTLOG("register id=%d keycode=%u mods=0x%llx shift=%d\n", id, keycode, (unsigned long long)modflags, withShift);
  pthread_mutex_lock(&gChordLock);
  for (int i = 0; i < OT_MAX_CHORDS; i++) {
    if (!gChords[i].used) {
      gChords[i].id = id;
      gChords[i].modflags = modflags | (withShift ? kCGEventFlagMaskShift : 0);
      gChords[i].holdMask = modflags;
      gChords[i].keycode = keycode;
      gChords[i].withShift = withShift;
      gChords[i].used = 1;
      pthread_mutex_unlock(&gChordLock);
      return 1;
    }
  }
  pthread_mutex_unlock(&gChordLock);
  return 0;
}

void ot_hotkey_capture_start(void) { gCaptureMode = 1; }

void ot_hotkey_capture_stop(void) { gCaptureMode = 0; }

void ot_hotkey_set_open(int open) {
  atomic_store_explicit(&gSwitcherOpen, open ? 1 : 0, memory_order_relaxed);
}

void ot_hotkey_unregister(int id) {
  pthread_mutex_lock(&gChordLock);
  for (int i = 0; i < OT_MAX_CHORDS; i++) {
    if (gChords[i].used && gChords[i].id == id) gChords[i].used = 0;
  }
  pthread_mutex_unlock(&gChordLock);
}

void ot_hotkey_stop(void) {
  if (gTap) CGEventTapEnable(gTap, false);
  if (gRunLoop) CFRunLoopStop(gRunLoop);
}

// Shared non-activating lookup for explicit actions and bulk eligibility.
// Resolving a remote root never fronts the app or switches Spaces.
static AXUIElementRef copyActionWindowRoot(uint32_t wid, int pid) {
  AXUIElementRef window = copyAXWindow(wid, pid);
  if (!window) window = copyAXWindowByBruteForce(wid, pid);
  if (!window) return NULL;
  CFTypeRef role = NULL;
  AXError result = AXUIElementCopyAttributeValue(window, kAXRoleAttribute, &role);
  BOOL root = result == kAXErrorSuccess && role && CFGetTypeID(role) == CFStringGetTypeID() && CFEqual(role, kAXWindowRole);
  if (role) CFRelease(role);
  if (!root) { CFRelease(window); return NULL; }
  return window;
}

// Explicit-target actions share the proven cross-Space lookup, but separately
// acknowledge the AX operation rather than treating PID lookup as success.
void ot_action_prepare_focus(uint32_t wid, int pid) {
  @autoreleasepool {
    ProcessSerialNumber psn;
    if (GetProcessForPID(pid, &psn) == noErr) {
      _SLPSSetFrontProcessWithOptions(&psn, wid, 0x200);
      otMakeKeyWindow(psn, wid);
    }
  }
}
int ot_action_raise_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef window = copyActionWindowRoot(wid, pid);
    if (!window) return 0;
    AXError raised = AXUIElementPerformAction(window, kAXRaiseAction);
    if (raised == kAXErrorSuccess) AXUIElementSetAttributeValue(window, kAXMainAttribute, kCFBooleanTrue);
    CFRelease(window);
    return raised == kAXErrorSuccess;
  }
}
// Unlike the single-window toggle, a bulk minimize always writes true, even
// when enumeration was stale or omitted the window's minimized state.
int ot_action_set_minimized(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef window = copyActionWindowRoot(wid, pid);
    if (!window) return 0;
    AXError result = AXUIElementSetAttributeValue(window, kAXMinimizedAttribute, kCFBooleanTrue);
    CFRelease(window);
    return result == kAXErrorSuccess;
  }
}

// Guard enumeration failures separately from non-window CG surfaces. A denied
// AX application must not produce an apparently successful empty bulk action.
int ot_action_windows_available(int pid) {
  @autoreleasepool {
    AXUIElementRef app = AXUIElementCreateApplication(pid);
    AXUIElementSetMessagingTimeout(app, 1.0);
    CFTypeRef windows = NULL;
    AXError result = AXUIElementCopyAttributeValue(app, kAXWindowsAttribute, &windows);
    int available = result == kAXErrorSuccess && windows && CFGetTypeID(windows) == CFArrayGetTypeID();
    if (windows) CFRelease(windows);
    CFRelease(app);
    return available;
  }
}
// 1: confirmed AXWindow; 0: confirmed different role; negative: unknown.
// Missing remote roots and AX errors must not be mistaken for CG-only surfaces.
int ot_action_window_is_root(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef window = copyAXWindow(wid, pid);
    if (!window) window = copyAXWindowByBruteForce(wid, pid);
    if (!window) return -1;
    CFTypeRef role = NULL;
    AXError result = AXUIElementCopyAttributeValue(window, kAXRoleAttribute, &role);
    int classification = -2;
    if (result == kAXErrorSuccess && role && CFGetTypeID(role) == CFStringGetTypeID()) {
      classification = CFEqual(role, kAXWindowRole) ? 1 : 0;
    }
    if (role) CFRelease(role);
    CFRelease(window);
    return classification;
  }
}
int ot_action_close_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef window = copyActionWindowRoot(wid, pid);
    if (!window) return 0;
    CFTypeRef button = NULL;
    AXError result = AXUIElementCopyAttributeValue(window, kAXCloseButtonAttribute, &button);
    BOOL closed = result == kAXErrorSuccess && button && CFGetTypeID(button) == AXUIElementGetTypeID() &&
      AXUIElementPerformAction((AXUIElementRef)button, kAXPressAction) == kAXErrorSuccess;
    if (button) CFRelease(button);
    CFRelease(window);
    return closed;
  }
}
