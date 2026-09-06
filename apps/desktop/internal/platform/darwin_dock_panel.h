#include <stdint.h>
uint64_t ot_dock_panel_create(void *host);
int ot_dock_panel_show(uint64_t token, double x, double y, double w, double h);
int ot_dock_panel_hide(uint64_t token);
int ot_dock_panel_close(uint64_t token);
