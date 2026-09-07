#include <stdint.h>
uint64_t ot_dock_panel_create(void *host);
int ot_dock_panel_show(uint64_t token, double x, double y, double w, double h);
int ot_dock_panel_hide(uint64_t token);
int ot_dock_panel_close(uint64_t token);

typedef struct {
  uint64_t session, revision, sequence, gesture, window;
  int app, owned, precise, inverted, phase, momentum, reason;
  double unixTime, x, y, dx, dy;
} OTDockPanelWheelEvent;
int ot_dock_panel_wheel_policy(uint64_t token, const char *json);
int ot_dock_panel_wheel_next(uint64_t token, OTDockPanelWheelEvent *event);
int ot_dock_panel_wheel_valid(uint64_t token, uint64_t session,
                              uint64_t revision, uint64_t gesture);
void ot_dock_panel_wheel_complete(uint64_t token, uint64_t session,
                                  uint64_t revision, uint64_t gesture);

uint64_t ot_launcher_panel_create(void *host, const char *display);
int ot_launcher_panel_visible(uint64_t token, const char *display);

uint64_t ot_launcher_panel_space(uint64_t token);

int ot_launcher_panel_style(uint64_t token, const char *material, const char *theme, int radius);
