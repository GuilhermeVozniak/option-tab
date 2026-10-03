#include <stdint.h>
void *ot_audio_output_start(void);
char *ot_audio_output_read(void *owner);
int ot_audio_output_dirty(void *owner);
void ot_audio_output_close(void *owner);
int ot_audio_output_select(const char *uid, uintptr_t guard);
