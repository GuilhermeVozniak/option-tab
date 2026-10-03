#ifndef OT_WINDOW_DRAG_H
#define OT_WINDOW_DRAG_H
#include <stdint.h>
typedef struct {
  uint64_t generation, gesture, sequence;
  int kind;
  double at, x, y;
} OTWindowDragRaw;
typedef struct {
  OTWindowDragRaw raw;
  uint32_t window;
  int app;
  double wx, wy;
  char role[64], subrole[64];
} OTWindowDragSample;
typedef struct {
  int root, known, modal, sheet, child, self;
  uint32_t parent;
  double x, y, w, h;
  char role[64], subrole[64], reason[128];
} OTActionWindowRole;
void *ot_window_drag_create(int test);
void ot_window_drag_pump(void *);
int ot_window_drag_pop(void *, OTWindowDragRaw *);
void ot_window_drag_stop(void *);
int ot_window_drag_sample(OTWindowDragRaw, OTWindowDragSample *);
void ot_window_drag_clear(uint64_t generation);
uint64_t ot_window_drag_generation(void *);
int ot_window_drag_current(uint64_t, uint64_t, uint32_t, int, double, double,
                           double);
double ot_window_drag_now(void);
OTActionWindowRole ot_window_drag_role(uint32_t, int);
int ot_window_drag_perform(int, uint32_t, int);
int ot_window_drag_perform_guarded(int, uint32_t, int, uintptr_t);
int ot_window_drag_guard_probe(uintptr_t);
extern int goWindowDragFinalGuard(uintptr_t);
uint64_t ot_window_drag_probe(void);
int ot_window_drag_active(void);
int ot_window_drag_trusted(void);
int ot_window_drag_listening_allowed(void);
int ot_window_drag_healthy(void *);
uint64_t ot_window_drag_evidence_epoch(void);
#endif
