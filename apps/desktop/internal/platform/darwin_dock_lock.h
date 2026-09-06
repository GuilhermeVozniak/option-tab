#ifndef OT_DOCK_LOCK_H
#define OT_DOCK_LOCK_H
#include <stdint.h>
typedef struct {
  double x, y, w, h, dx, dy;
} OTLockSegment;
void *ot_lock_create(int test);
void ot_lock_destroy(void *);
int ot_lock_start(void *);
void ot_lock_pump(void *);
void ot_lock_stop(void *);
char *ot_lock_snapshot(void *);
char *ot_lock_displays(void);
uint64_t ot_lock_generation(void);
uint64_t ot_lock_physical(void *);
void ot_lock_publish(void *, uint64_t, double, uint64_t, OTLockSegment *, int);
void ot_lock_disable(void *);
int ot_lock_health(void *);
int ot_lock_bypass(uint64_t);
int ot_lock_buttons(void);
void ot_lock_pointer(double *, double *);
void ot_lock_begin_placement(void *, uint64_t);
void ot_lock_cancel_placement(void *, uint64_t);
int ot_lock_move(void *, uint64_t, uint64_t, uint64_t, double, double, double,
                 double);
int ot_lock_restore(void *, uint64_t, uint64_t, uint64_t, double, double);
double ot_lock_now(void);
uint64_t ot_lock_probe(void);
uint64_t ot_lock_placement_probe(void);
uint64_t ot_lock_advance_environment(void);
int ot_lock_test_moves(void *);
void ot_lock_retire(void *);
void ot_lock_transport_only(void *);
int ot_lock_foreground(void);
uint64_t ot_lock_delivery_probe(void);
int ot_lock_carriers_seen(void *);
#endif
