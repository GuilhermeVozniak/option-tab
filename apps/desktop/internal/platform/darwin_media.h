#ifndef OT_MEDIA_H
#define OT_MEDIA_H
#include <stdint.h>
char *ot_media_read(const char *provider);
char *ot_media_permission(const char *provider, int ask);
char *ot_media_command(const char *provider,int pid,const char *launch,const char *track,const char *kind,int64_t position,uintptr_t guard);
char *ot_media_artwork(const char *provider,int pid,const char *launch,const char *track);
#endif
