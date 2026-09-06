#ifndef OT_DOCK_INPUT_H
#define OT_DOCK_INPUT_H
#include <stdint.h>
#include <stddef.h>

typedef struct {
 uint64_t generation;
 int dock_pid, app_pid;
 double expiry, x,y,w,h;
 uint32_t screen;
 int edge;
 char path[1024], bundle[256];
} OTDockInputTarget;
typedef struct {
 void *event;
 OTDockInputTarget target;
 uint64_t sequence,gesture;
 double at;
 int type,cancelled,validated,terminal;
} OTDockInputRaw;
typedef struct {
 double x,y,dx,dy;
 int button,mods,precise,inverted,phase,momentum;
} OTDockInputFields;

double ot_dock_input_now(void);
void *ot_dock_input_create(int click,int scroll,int right,int test);
void ot_dock_input_target(void *owner, OTDockInputTarget target);
void ot_dock_input_pump(void *owner);
int ot_dock_input_pop(void *owner, OTDockInputRaw *out);
void ot_dock_input_ack(void *owner,uint64_t generation,uint64_t gesture,int valid);
void ot_dock_input_stop(void *owner);
int ot_dock_input_current(uint64_t generation,uint64_t gesture);
int ot_dock_input_owned(uint64_t generation,uint64_t gesture);
void ot_dock_input_complete(uint64_t generation,uint64_t gesture);
int ot_dock_input_completed(uint64_t generation,uint64_t gesture);
int ot_dock_input_validate_app(OTDockInputRaw raw);
int ot_dock_input_validate(OTDockInputRaw raw);
OTDockInputFields ot_dock_input_fields(void *event);
void ot_dock_input_release(void *event);
// Isolated seam: only owners created with test=1 accept injected events; no tap
// is installed and replay is recorded instead of posted to the desktop.
int ot_dock_input_test_event(void *owner,int type,double x,double y,int mods,int synthetic,int phase,int momentum);
int ot_dock_input_test_replays(void *owner);
int ot_dock_input_test_alive(void);
void ot_dock_input_test_idle(void *owner);
void ot_dock_input_test_changed_process(void *owner);
uint64_t ot_dock_input_synthetic_passes(void);
uint64_t ot_dock_input_installations(void);
int ot_dock_input_health(void *owner);
void ot_dock_input_test_health(void *owner,int mask);
#endif
