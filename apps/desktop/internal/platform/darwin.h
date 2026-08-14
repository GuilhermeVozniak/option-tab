// C API bridging the macOS native backend (darwin.m) to Go (darwin.go).
// Strings returned as char* are heap-allocated and must be freed by the caller
// with free(). All functions are safe to call without special permissions,
// returning empty data or a denied state rather than crashing.
#ifndef OPTION_TAB_DARWIN_H
#define OPTION_TAB_DARWIN_H

#include <stdint.h>

// ot_list_windows_json returns a JSON array of windows across all Spaces,
// including minimized and hidden ones. Each element:
//   {"id":<uint>,"pid":<int>,"app":"<name>","bundle":"<id>","title":"<s>",
//    "x":..,"y":..,"w":..,"h":..,"onscreen":<bool>,"minimized":<bool>,
//    "hidden":<bool>,"fullscreen":<bool>,"screen":<uint>,"space":<uint>,
//    "zorder":<int>}
// Caller frees the returned string.
char *ot_list_windows_json(void);

// ot_active_app_pid returns the pid of the frontmost application, or 0.
int ot_active_app_pid(void);

// ot_window_pid returns the owning pid of a single window id, or 0. Lightweight
// lookup for the action path (focus/close/minimize), avoiding a full enrichment
// pass.
int ot_window_pid(uint32_t wid);

// ot_active_space returns the id of the currently active Space, or 0 if it
// cannot be resolved (private CGS API unavailable).
uint64_t ot_active_space(void);

// ot_active_screen returns the display id of the screen containing the focused
// window (or the main display), or 0.
uint32_t ot_active_screen(void);

// ot_cursor_screen returns the display id of the screen under the mouse cursor,
// or 0.
uint32_t ot_cursor_screen(void);

// ot_screens_json returns a JSON array of displays. Each element:
//   {"id":<uint>,"main":<bool>,"x":..,"y":..,"w":..,"h":..,
//    "vx":..,"vy":..,"vw":..,"vh":..}  (v* is the visible frame).
// Caller frees the returned string.
char *ot_screens_json(void);

// Permission checks: return 1 if granted, 0 otherwise.
int ot_perm_accessibility(void);
int ot_perm_screen_recording(void);

// Permission requests (may show a system prompt). Non-blocking.
void ot_request_accessibility(void);
void ot_request_screen_recording(void);

// ot_open_privacy_settings opens the System Settings privacy pane
// (0 = Accessibility, 1 = Screen Recording). Non-blocking.
void ot_open_privacy_settings(int kind);

// Window/app actions. Return 1 on success, 0 on failure.
int ot_focus_window(uint32_t wid, int pid);
int ot_close_window(uint32_t wid, int pid);
int ot_minimize_window(uint32_t wid, int pid);
// ot_fullscreen_window toggles the window's native fullscreen state.
int ot_fullscreen_window(uint32_t wid, int pid);
int ot_quit_app(int pid);
int ot_hide_app(int pid);

// ot_thumbnail_png_base64 returns a base64-encoded PNG snapshot of the window,
// scaled so its largest side is at most maxpx. Empty string on failure.
// Caller frees the returned string.
char *ot_thumbnail_png_base64(uint32_t wid, int maxpx);

// ot_app_icon_png_base64 returns the icon of the app owning pid as a
// "data:image/png;base64,..." URL, scaled so its largest side is maxpx.
// Empty string on failure. Caller frees the returned string.
char *ot_app_icon_png_base64(int pid, int maxpx);

// ot_thumbnail_dataurl returns a window snapshot as a "data:image/png;base64,..."
// URL via ScreenCaptureKit, scaled to maxpx. Empty string on failure (e.g. no
// Screen Recording permission). Caller frees the returned string.
char *ot_thumbnail_dataurl(uint32_t wid, int maxpx);

// Menubar tray is provided by the Wails v3 SystemTray manager (Go side), so
// there are no native tray entry points here anymore.

// Warps the mouse cursor to the center of the window (cursor follows focus).
void ot_warp_cursor(uint32_t wid);

// ot_hide_dock_icon switches the app to the accessory activation policy so it
// never shows in the Dock or the Cmd-Tab app switcher (bundles also set
// LSUIElement and Wails applies ActivationPolicyAccessory at launch; this is
// used to flip back after the preferences window activated the app).
void ot_hide_dock_icon(void);

// ot_activate_prefs captures the currently frontmost app (for the switcher's
// active-app scope), then flips the app to the regular activation policy and
// activates it so the preferences window can take keyboard focus.
void ot_activate_prefs(void);

// ot_app_hide hides the app ([NSApp hide:]) when it is currently active, which
// returns focus to the previously active app. Used after the overlay hides to
// drop an accidental click-activation. No-op when the app is not active.
void ot_app_hide(void);

// ot_window_fit_screen resizes the (transparent) overlay window to the
// visible frame of the screen with the given display id (0 or unknown = the
// window's current screen), giving the switcher the whole screen to lay out
// in. win is the Wails window's NSWindow pointer. Call before each show;
// dispatched to the main queue.
void ot_window_fit_screen(void *win, uint32_t displayID);

// ot_haptic_tick performs a subtle trackpad haptic tap (no-op on hardware
// without a Force Touch trackpad).
void ot_haptic_tick(void);

// Login item (SMAppService, macOS 13+). Returns 1/0; set returns 1 on success.
int ot_login_item_enabled(void);
int ot_login_item_set(int enabled);

// Hotkey engine. Events are delivered to Go via the exported goHotkeyEvent.
// kinds: 0 activate, 1 advance, 2 reverse, 3 release, 4 cancel.
// Raw key presses while the switcher is open go to goKeyEvent.
int ot_hotkey_start(void);
int ot_hotkey_register(int id, uint64_t modflags, uint16_t keycode, int withShift);
void ot_hotkey_unregister(int id);
void ot_hotkey_stop(void);

// ot_hotkey_set_open tells the tap whether the switcher overlay is open. While
// open, every key press is consumed (so it never reaches the previously active
// app) and forwarded to Go via goKeyEvent; Escape cancels regardless of
// modifier state (covers the "when released: do nothing" mode).
void ot_hotkey_set_open(int open);

// One-shot chord recording for the preferences UI: while armed, the next
// modifier+key press anywhere is delivered to goHotkeyCaptured (and consumed)
// instead of being processed as a hotkey — including chords the webview never
// sees, like Command+Tab. Escape cancels (keycode 0xFFFF).
void ot_hotkey_capture_start(void);
void ot_hotkey_capture_stop(void);

#endif
