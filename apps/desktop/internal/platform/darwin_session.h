#ifndef OT_SESSION_OBSERVER_H
#define OT_SESSION_OBSERVER_H
#include <stdint.h>
typedef struct {
 uint64_t revision;
 int valid, on_console, login_done, lock_state;
 int session_event, screen_event, sleep_event, lock_event;
} OTSessionSnapshot;
void *ot_session_observer_create(void);
OTSessionSnapshot ot_session_observer_poll(void *observer);
void ot_session_observer_destroy(void *observer);
#endif
