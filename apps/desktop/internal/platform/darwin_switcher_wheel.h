#ifndef OT_SWITCHER_WHEEL_H
#define OT_SWITCHER_WHEEL_H
#include "darwin_dock_panel.h"
uint64_t ot_switcher_wheel_create(void *host);
int ot_switcher_wheel_close(uint64_t token);
int ot_switcher_wheel_policy(uint64_t token, const char *json);
int ot_switcher_wheel_next(uint64_t token, OTDockPanelWheelEvent *event);
int ot_switcher_wheel_valid(uint64_t token, uint64_t session, uint64_t revision, uint64_t gesture);
void ot_switcher_wheel_complete(uint64_t token, uint64_t session, uint64_t revision, uint64_t gesture);
#endif
