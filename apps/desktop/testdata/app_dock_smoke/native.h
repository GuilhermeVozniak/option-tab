#include <stdint.h>
int smoke_activate(int pid);
int smoke_foreground(void);
uint32_t smoke_focused_window(int pid);
void smoke_click(double x,double y);
void smoke_key(void);
char *smoke_state(void *host);

int smoke_screen(int index,double *x,double *y);
int smoke_position_fixture(int pid,double x,double y);
void smoke_pointer(double *x,double *y);
void smoke_warp(double x,double y);
char *smoke_fixture_surfaces(int pid);

void smoke_tracking_diagnostic(void *host);
