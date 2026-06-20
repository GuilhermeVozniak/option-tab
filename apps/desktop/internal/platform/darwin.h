// C API bridging the macOS native backend (darwin.m) to Go (darwin.go).
// Strings returned as char* are heap-allocated and must be freed by the caller
// with free(). All functions are safe to call without special permissions,
// returning empty data or a denied state rather than crashing.
#ifndef OPTION_TAB_DARWIN_H
#define OPTION_TAB_DARWIN_H

#include <stdint.h>

// ot_list_windows_json returns a JSON array of on-screen windows. Each element:
//   {"id":<uint>,"pid":<int>,"app":"<name>","bundle":"<id>","title":"<s>",
//    "x":..,"y":..,"w":..,"h":..,"layer":<int>,"onscreen":<bool>}
// Caller frees the returned string.
char *ot_list_windows_json(void);

// ot_active_app_pid returns the pid of the frontmost application, or 0.
int ot_active_app_pid(void);

// Permission checks: return 1 if granted, 0 otherwise.
int ot_perm_accessibility(void);
int ot_perm_screen_recording(void);

// Permission requests (may show a system prompt). Non-blocking.
void ot_request_accessibility(void);
void ot_request_screen_recording(void);

// Window/app actions. Return 1 on success, 0 on failure.
int ot_focus_window(uint32_t wid, int pid);
int ot_close_window(uint32_t wid, int pid);
int ot_minimize_window(uint32_t wid, int pid);
int ot_quit_app(int pid);
int ot_hide_app(int pid);

// ot_thumbnail_png_base64 returns a base64-encoded PNG snapshot of the window,
// scaled so its largest side is at most maxpx. Empty string on failure.
// Caller frees the returned string.
char *ot_thumbnail_png_base64(uint32_t wid, int maxpx);

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
