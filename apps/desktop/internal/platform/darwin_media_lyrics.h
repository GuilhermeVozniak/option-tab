#ifndef OT_MEDIA_LYRICS_H
#define OT_MEDIA_LYRICS_H
#include <stdint.h>
int ot_lyrics_main_thread(void);
void *ot_lyrics_choose_start(void);
void ot_lyrics_choose_cancel(void *);
char *ot_lyrics_choose_poll(void *);
void ot_lyrics_choose_release(void *);
char *ot_lyrics_read(const void *,int,uint64_t,uint64_t);
#endif
