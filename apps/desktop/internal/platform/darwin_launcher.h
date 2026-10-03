#include <stdint.h>
char *ot_launcher_environment(void);
char *ot_launcher_spaces(void);
int ot_launcher_pointer(double *x, double *y);
int ot_launcher_activate(uint64_t token, const char *display, uint64_t space,
                         int pid, uint64_t sec, uint64_t usec,
                         const char *bundle, uintptr_t guard);

int ot_launcher_space_ordinary(const char *display);

uint64_t ot_launcher_space_id(const char *display);

void *ot_launcher_watch_start(void);
int ot_launcher_watch_changed(void *owner);
void ot_launcher_watch_stop(void *owner);

int ot_launcher_panel_validate(uint64_t token, const char *display);
