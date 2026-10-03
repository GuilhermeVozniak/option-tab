#ifndef OT_PREVIEW_DRAG_H
#define OT_PREVIEW_DRAG_H
#include <stdint.h>
typedef struct {double width,height;} OTPreviewDragSize;
typedef struct {uint64_t sequence;double x,y;int released,cancelled;} OTPreviewDragSample;
void *ot_preview_drag_create(int test);
int ot_preview_drag_prepare(void *owner,uint32_t window,int pid,OTPreviewDragSize *size);
int ot_preview_drag_start(void *owner);
void ot_preview_drag_check(void *owner);
void ot_preview_drag_pump(void *owner);
void ot_preview_drag_stop_tap(void *owner);
OTPreviewDragSample ot_preview_drag_sample(void *owner);
int ot_preview_drag_position(void *owner,double x,double y);
void ot_preview_drag_cancel(void *owner);
void ot_preview_drag_destroy(void *owner);
int ot_preview_drag_test_event(void *owner,int type,double x,double y,int key);
void ot_preview_drag_test_button(void *owner,int down);
int ot_preview_drag_test_writes(void *owner);
void ot_preview_drag_test_screens(void *owner,uint64_t current,uint64_t registration);
int ot_preview_drag_test_synthetic_up(void *owner);
#endif
