#ifndef OT_MATERIAL_H
#define OT_MATERIAL_H
#include <stdint.h>
uint64_t ot_material_preview(uint64_t);
uint64_t ot_material_overlay(void *);
int ot_material_apply(uint64_t, uint64_t, uint64_t, int, const char *, int,
                      double, double, double, double, uintptr_t);
int ot_material_retire(uint64_t, uint64_t, uint64_t, int, uintptr_t);
void ot_material_close(uint64_t);
#endif
