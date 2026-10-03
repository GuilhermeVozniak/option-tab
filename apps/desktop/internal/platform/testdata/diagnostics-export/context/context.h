#include <stdint.h>
void *context_start(uintptr_t);
void context_pump(void);
int context_result(void *);
int context_panels(void);
void context_release(void *);
int context_write(uintptr_t, const char *);
