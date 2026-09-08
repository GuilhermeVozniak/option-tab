#ifndef OT_DIAGNOSTICS_EXPORT_H
#define OT_DIAGNOSTICS_EXPORT_H
#include <stddef.h>
#include <stdint.h>
int ot_diagnostics_main_thread(void);
void *ot_json_save_start(const void *, size_t, uintptr_t, const char *);
void *ot_diagnostics_save_start(const void *, size_t, uintptr_t);
void ot_diagnostics_save_cancel(void *);
int ot_diagnostics_save_poll(void *);
void ot_diagnostics_save_release(void *);
#endif
