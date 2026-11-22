#ifndef OT_KEY_TEXT_H
#define OT_KEY_TEXT_H
#include <stdint.h>

// ot_key_layout_refresh reads the current keyboard layout so that
// ot_key_text_without_option can translate keys on any thread. Main thread
// only: Text Input Sources aborts the process when called anywhere else.
void ot_key_layout_refresh(void);

// ot_key_layout_watch refreshes the layout now and again whenever the user
// picks another one. Main thread only; later calls do nothing.
void ot_key_layout_watch(void);

// ot_key_text_without_option writes what keycode types with Option released
// but Shift and Caps Lock as in flags (CGEventFlags), as UTF-16 units. It
// returns the number of units written, or 0 when no layout has been read yet
// or the key types nothing. Safe on any thread.
int ot_key_text_without_option(uint16_t keycode, uint64_t flags, uint16_t *out, int capacity);

#endif
