#ifndef OT_DOCK_OBSERVER_H
#define OT_DOCK_OBSERVER_H
void *ot_dock_observer_create(void);
char *ot_dock_observer_poll(void *observer);
void ot_dock_observer_destroy(void *observer);
#endif
