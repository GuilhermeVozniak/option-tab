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

// Menubar status item. ot_tray_install shows the icon and wires its menu to the
// exported goTrayCommand (0=preferences, 1=toggle pause, 2=quit). All three are
// safe to call from any thread; the work is dispatched to the main queue.
void ot_tray_install(void);
void ot_tray_set_paused(int paused);
// Sets the status-item glyph: "default", "outline", or "dot".
void ot_tray_set_style(const char *style);
void ot_tray_remove(void);

// Warps the mouse cursor to the center of the window (cursor follows focus).
void ot_warp_cursor(uint32_t wid);

// ot_hide_dock_icon switches the app to the accessory activation policy so it
// never shows in the Dock or the Cmd-Tab app switcher (bundles also set
// LSUIElement; this covers `wails dev` runs). Safe to call repeatedly.
void ot_hide_dock_icon(void);

// ot_window_set_prefs_mode(1) turns the single Wails window into a regular
// titled, movable, closable preferences window; (0) restores the borderless
// floating overlay chrome. Dispatched to the main queue.
void ot_window_set_prefs_mode(int on);

// ot_window_init_overlay makes the app window non-opaque with a clear
// background so only the webview's rounded panel is visible (no square
// window backdrop). Call once at startup; dispatched to the main queue.
void ot_window_init_overlay(void);

// ot_window_fit_screen resizes the (transparent) overlay window to the
// visible frame of its screen, giving the switcher the whole screen to lay
// out in. Call before each show; dispatched to the main queue.
void ot_window_fit_screen(void);

// Login item (SMAppService, macOS 13+). Returns 1/0; set returns 1 on success.
int ot_login_item_enabled(void);
int ot_login_item_set(int enabled);

// Hotkey engine. Events are delivered to Go via the exported goHotkeyEvent.
// kinds: 0 activate, 1 advance, 2 reverse, 3 release, 4 cancel.
int ot_hotkey_start(void);
int ot_hotkey_register(int id, uint64_t modflags, uint16_t keycode, int withShift);
void ot_hotkey_unregister(int id);
void ot_hotkey_stop(void);

#endif
