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

// Private CGS (SkyLight) API for Space resolution, as AltTab uses. These live in
// CoreGraphics and link without a public header. All callers degrade to 0 when
// the connection or result is unavailable, so a missing or changed API makes
// Space data inert (filters fall back to "all") rather than crashing.
typedef int CGSConnectionID;
extern CGSConnectionID CGSMainConnectionID(void);
extern uint64_t CGSGetActiveSpace(CGSConnectionID cid);
extern CFArrayRef CGSCopySpacesForWindows(CGSConnectionID cid, int mask, CFArrayRef windowIDs);

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

// spaceForWindow resolves the Space id containing wid via the private CGS API,
// returning 0 when it cannot be determined (so Space filters stay inert).
static uint64_t spaceForWindow(CGSConnectionID cid, CGWindowID wid) {
  if (cid == 0 || wid == 0) return 0;
  CFNumberRef num = CFNumberCreate(NULL, kCFNumberSInt32Type, &wid);
  CFArrayRef warr = CFArrayCreate(NULL, (const void **)&num, 1, &kCFTypeArrayCallBacks);
  uint64_t sid = 0;
  CFArrayRef spaces = CGSCopySpacesForWindows(cid, 0x7, warr);
  if (spaces) {
    if (CFArrayGetCount(spaces) > 0) {
      CFNumberRef s = (CFNumberRef)CFArrayGetValueAtIndex(spaces, 0);
      CFNumberGetValue(s, kCFNumberSInt64Type, &sid);
    }
    CFRelease(spaces);
  }
  CFRelease(warr);
  CFRelease(num);
  return sid;
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
      uint64_t space = spaceForWindow(cid, wid != nil ? (CGWindowID)[wid unsignedIntValue] : 0);

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

// ot_window_pid resolves the owning pid of a single window id via a targeted
// CGWindowList query (no AX/Space enrichment), keeping the action path fast.
int ot_window_pid(uint32_t wid) {
  @autoreleasepool {
    CGWindowID ids[1] = { (CGWindowID)wid };
    CFArrayRef arr = CFArrayCreate(NULL, (const void **)ids, 1, NULL);
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

static void otInstallFrontObserver(void) {
  if (gFrontObserverInstalled) return;
  gFrontObserverInstalled = YES;
  pid_t self = [[NSProcessInfo processInfo] processIdentifier];
  NSRunningApplication *front = [[NSWorkspace sharedWorkspace] frontmostApplication];
  if (front && front.processIdentifier != self) {
    gLastRealFrontPid = front.processIdentifier;
  }
  [[[NSWorkspace sharedWorkspace] notificationCenter]
      addObserverForName:NSWorkspaceDidActivateApplicationNotification
                  object:nil
                   queue:nil
              usingBlock:^(NSNotification *note) {
    NSRunningApplication *app = note.userInfo[NSWorkspaceApplicationKey];
    if (app && app.processIdentifier != self) {
      gLastRealFrontPid = app.processIdentifier;
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
        @"main": @(did == CGMainDisplayID()),
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

// otMakeKeyWindow posts the two SkyLight event records AltTab uses to make a
// specific window key after its process is fronted.
static void otMakeKeyWindow(ProcessSerialNumber psn, uint32_t wid) {
  uint8_t bytes[0xf8] = {0};
  bytes[0x04] = 0xf8;
  bytes[0x08] = 0x01;
  bytes[0x3a] = 0x10;
  memcpy(&bytes[0x3c], &wid, sizeof(uint32_t));
  memset(&bytes[0x20], 0xff, 0x10);
  bytes[0x08] = 0x02;
  SLPSPostEventRecordTo(&psn, bytes);
  bytes[0x08] = 0x01;
  SLPSPostEventRecordTo(&psn, bytes);
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
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) return 0;
    AXUIElementPerformAction(w, kAXRaiseAction);
    AXUIElementSetAttributeValue(w, kAXMainAttribute, kCFBooleanTrue);
    CFRelease(w);
    if (!fronted) {
      // The pid could not be resolved for SkyLight fronting (e.g. an accessory
      // process): fall back to activating the app so the window still gets
      // focus. This never switches Spaces, so it can't pull the window over.
      NSRunningApplication *app =
          [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
      [app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
    }
    return 1;
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
  uint64_t modflags; // required modifier mask (excluding shift)
  uint16_t keycode;
  int withShift;     // 1 if this registration is the shift/reverse variant
  int used;
} OTChord;

#define OT_MAX_CHORDS 32
static OTChord gChords[OT_MAX_CHORDS];
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

static const uint64_t kModMask = (kCGEventFlagMaskControl | kCGEventFlagMaskAlternate |
                                  kCGEventFlagMaskShift | kCGEventFlagMaskCommand);

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
    if (gActive && (flags & gHoldMask) != gHoldMask) {
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

    int shiftHeld = (flags & kCGEventFlagMaskShift) ? 1 : 0;
    for (int i = 0; i < OT_MAX_CHORDS; i++) {
      if (!gChords[i].used) continue;
      if (gChords[i].keycode != keycode) continue;
      uint64_t need = gChords[i].modflags;
      if ((flags & need) != need) continue;

      if (!gActive) {
        gActive = 1;
        gHoldMask = need;
        goHotkeyEvent(0, gChords[i].id); // activate
      } else if (shiftHeld) {
        goHotkeyEvent(2, gChords[i].id); // reverse
      } else {
        goHotkeyEvent(1, gChords[i].id); // advance
      }
      return NULL; // consume the chord
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
  for (int i = 0; i < OT_MAX_CHORDS; i++) {
    if (!gChords[i].used) {
      gChords[i].id = id;
      gChords[i].modflags = modflags | (withShift ? kCGEventFlagMaskShift : 0);
      gChords[i].keycode = keycode;
      gChords[i].withShift = withShift;
      gChords[i].used = 1;
      return 1;
    }
  }
  return 0;
}

void ot_hotkey_capture_start(void) { gCaptureMode = 1; }

void ot_hotkey_capture_stop(void) { gCaptureMode = 0; }

void ot_hotkey_set_open(int open) {
  atomic_store_explicit(&gSwitcherOpen, open ? 1 : 0, memory_order_relaxed);
}

void ot_hotkey_unregister(int id) {
  for (int i = 0; i < OT_MAX_CHORDS; i++) {
    if (gChords[i].used && gChords[i].id == id) gChords[i].used = 0;
  }
}

void ot_hotkey_stop(void) {
  if (gTap) CGEventTapEnable(gTap, false);
  if (gRunLoop) CFRunLoopStop(gRunLoop);
}
