#ifndef OT_DARWIN_APPS_H
#define OT_DARWIN_APPS_H

char *ot_list_apps_json(void);
// 1 accepted, 0 refused, -1 invalid/stale identity.
int ot_activate_app(int pid);
int ot_frontmost_app_pid(void);

#endif
