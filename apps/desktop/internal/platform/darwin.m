//go:build darwin

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ServiceManagement/ServiceManagement.h>
#include <pthread.h>
#include "darwin.h"

// Exported from Go (see darwin.go): receives hotkey events from the tap thread.
extern void goHotkeyEvent(int kind, int id);

// Private API used (as AltTab does) to map an AXUIElement to its CGWindowID.
extern AXError _AXUIElementGetWindow(AXUIElementRef element, CGWindowID *windowID);

// ---- Helpers ----

static char *copyCString(NSString *s) {
  if (s == nil) s = @"";
  const char *utf8 = [s UTF8String];
  char *out = malloc(strlen(utf8) + 1);
  strcpy(out, utf8);
  return out;
}

// ---- Window enumeration ----

char *ot_list_windows_json(void) {
  @autoreleasepool {
    CGWindowListOption opt = kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements;
    CFArrayRef list = CGWindowListCopyWindowInfo(opt, kCGNullWindowID);
    NSMutableArray *out = [NSMutableArray array];
    NSArray *windows = (__bridge_transfer NSArray *)list;
    NSWorkspace *ws = [NSWorkspace sharedWorkspace];

    for (NSDictionary *info in windows) {
      NSNumber *layer = info[(__bridge NSString *)kCGWindowLayer];
      if (layer == nil || [layer intValue] != 0) continue; // only normal windows

      NSNumber *wid = info[(__bridge NSString *)kCGWindowNumber];
      NSNumber *pid = info[(__bridge NSString *)kCGWindowOwnerPID];
      NSString *owner = info[(__bridge NSString *)kCGWindowOwnerName];
      NSString *name = info[(__bridge NSString *)kCGWindowName];
      NSDictionary *bounds = info[(__bridge NSString *)kCGWindowBounds];

      NSString *bundle = @"";
      if (pid != nil) {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:[pid intValue]];
        if (app.bundleIdentifier != nil) bundle = app.bundleIdentifier;
      }

      [out addObject:@{
        @"id": wid ?: @0,
        @"pid": pid ?: @0,
        @"app": owner ?: @"",
        @"bundle": bundle,
        @"title": name ?: @"",
        @"x": bounds[@"X"] ?: @0,
        @"y": bounds[@"Y"] ?: @0,
        @"w": bounds[@"Width"] ?: @0,
        @"h": bounds[@"Height"] ?: @0,
        @"layer": layer,
        @"onscreen": @YES,
      }];
    }

    NSData *json = [NSJSONSerialization dataWithJSONObject:out options:0 error:nil];
    NSString *s = [[NSString alloc] initWithData:json encoding:NSUTF8StringEncoding];
    return copyCString(s);
  }
}

int ot_active_app_pid(void) {
  @autoreleasepool {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    return app ? (int)app.processIdentifier : 0;
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

int ot_focus_window(uint32_t wid, int pid) {
  @autoreleasepool {
    AXUIElementRef w = copyAXWindow(wid, pid);
    if (w == NULL) return 0;
    AXUIElementPerformAction(w, kAXRaiseAction);
    AXUIElementSetAttributeValue(w, kAXMainAttribute, kCFBooleanTrue);
    CFRelease(w);
    NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
    [app activateWithOptions:NSApplicationActivateIgnoringOtherApps];
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
    int ok = (AXUIElementSetAttributeValue(w, kAXMinimizedAttribute, kCFBooleanTrue) == kAXErrorSuccess);
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

char *ot_thumbnail_png_base64(uint32_t wid, int maxpx) {
  // CGWindowListCreateImage was removed in macOS 15. Live previews via
  // ScreenCaptureKit are planned; until then the overlay renders app glyphs and
  // this returns an empty string so the Go layer reports "no thumbnail".
  (void)wid;
  (void)maxpx;
  return copyCString(@"");
}

// ---- Login item (SMAppService) ----

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
static int gActive = 0;          // switcher currently open
static uint64_t gHoldMask = 0;   // modifiers held during the active session

static const uint64_t kModMask = (kCGEventFlagMaskControl | kCGEventFlagMaskAlternate |
                                  kCGEventFlagMaskShift | kCGEventFlagMaskCommand);

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

    if (keycode == 53 && gActive) { // Escape
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
  }
  return event;
}

static void *hotkeyThread(void *arg) {
  (void)arg;
  @autoreleasepool {
    CGEventMask mask = CGEventMaskBit(kCGEventKeyDown) | CGEventMaskBit(kCGEventFlagsChanged);
    gTap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionDefault,
                            mask, tapCallback, NULL);
    if (gTap == NULL) return NULL;
    gSource = CFMachPortCreateRunLoopSource(kCFAllocatorDefault, gTap, 0);
    gRunLoop = CFRunLoopGetCurrent();
    CFRunLoopAddSource(gRunLoop, gSource, kCFRunLoopCommonModes);
    CGEventTapEnable(gTap, true);
    CFRunLoopRun();
  }
  return NULL;
}

int ot_hotkey_start(void) {
  if (gTap != NULL) return 1;
  pthread_t t;
  if (pthread_create(&t, NULL, hotkeyThread, NULL) != 0) return 0;
  pthread_detach(t);
  return 1;
}

int ot_hotkey_register(int id, uint64_t modflags, uint16_t keycode, int withShift) {
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

void ot_hotkey_unregister(int id) {
  for (int i = 0; i < OT_MAX_CHORDS; i++) {
    if (gChords[i].used && gChords[i].id == id) gChords[i].used = 0;
  }
}

void ot_hotkey_stop(void) {
  if (gTap) CGEventTapEnable(gTap, false);
  if (gRunLoop) CFRunLoopStop(gRunLoop);
}
