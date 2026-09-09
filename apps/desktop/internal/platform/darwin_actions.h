#ifndef OT_DARWIN_ACTIONS_H
#define OT_DARWIN_ACTIONS_H
#include <stdint.h>
void ot_action_prepare_focus(uint32_t wid, int pid);
int ot_action_raise_window(uint32_t wid, int pid);
int ot_action_set_minimized(uint32_t wid, int pid);
int ot_action_close_window(uint32_t wid, int pid);
int ot_action_app_valid(int pid);
int ot_action_windows_available(int pid);
// 1 root, 0 confirmed non-window, negative unresolved/refused.
int ot_action_window_is_root(uint32_t wid, int pid);
int ot_action_force_quit(int pid);
// Returns 1 on success, 0 on refusal/failure, -1 when no supported command exists.
int ot_action_new_window(int pid);
#endif
