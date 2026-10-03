#ifndef OT_FOLDER_H
#define OT_FOLDER_H
#include <stdint.h>
void *ot_folder_begin(const char *, const void *, int, char **, char **);
void ot_folder_end(void *);
int ot_folder_open(void *, const char *, uint64_t, uint64_t, uint64_t, uint64_t,
                   uintptr_t);
void *ot_folder_grant_start(const char *);
void ot_folder_grant_cancel(void *);
char *ot_folder_grant_poll(void *);
void ot_folder_grant_release(void *);
char *ot_folder_test_bookmark(const char *);
int ot_folder_open_probe(void *, const char *, uint64_t, uint64_t, uint64_t,
                         uint64_t, uintptr_t);
#endif
