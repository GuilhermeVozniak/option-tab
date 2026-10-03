#pragma once
#include <stdint.h>
typedef struct {
  uint64_t epoch, session, revision, admission;
  int enabled, scroll, magnify, swipe;
  double w, h;
  char display[128];
} OTLauncherGesturePolicy;
typedef struct {
  OTLauncherGesturePolicy policy;
  uint64_t sequence, gesture;
  double time, x, y, dx, dy, magnification;
  int kind, phase, momentum, precise, owned;
} OTLauncherGestureEvent;
int ot_launcher_gesture_policy(uint64_t, OTLauncherGesturePolicy);
int ot_launcher_gesture_policy_guarded(uint64_t, OTLauncherGesturePolicy, uintptr_t);
int ot_go_launcher_gesture_policy_current(uintptr_t);
int ot_launcher_gesture_next(uint64_t, OTLauncherGestureEvent *);
int ot_launcher_gesture_valid(uint64_t, uint64_t, uint64_t, uint64_t, uint64_t,
                              uint64_t);
void ot_launcher_gesture_complete(uint64_t, uint64_t, uint64_t, uint64_t,
                                  uint64_t, uint64_t);
#ifdef __OBJC__
#import <Cocoa/Cocoa.h>
BOOL OTLauncherGestureHandle(uint64_t, NSEvent *, NSView *);
void OTLauncherGestureRetire(uint64_t, BOOL);
int OTLauncherGestureHost(uint64_t, const char *);
#endif
