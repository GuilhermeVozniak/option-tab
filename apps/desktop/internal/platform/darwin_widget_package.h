#ifndef OT_WIDGET_PACKAGE_H
#define OT_WIDGET_PACKAGE_H
#include <stdint.h>
int ot_widget_package_main_thread(void);
void *ot_widget_package_start(uintptr_t);
void ot_widget_package_cancel(void *);
char *ot_widget_package_poll(void *);
void ot_widget_package_release(void *);
#endif
