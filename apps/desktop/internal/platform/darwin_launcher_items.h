#include <stdint.h>
void *ot_launcher_item_choose(const char *, uintptr_t);
void ot_launcher_item_cancel(void *);
char *ot_launcher_item_poll(void *);
void ot_launcher_item_release(void *);
void *ot_launcher_item_resolve(const char *, char **);
char *ot_launcher_item_scope_info(void *);
void ot_launcher_item_scope_release(void *);
void *ot_launcher_item_open(void *, const char *, const char *, uint64_t, int,
                            uint64_t, uint64_t, const char *, uintptr_t);
int ot_launcher_item_same(void *);
int ot_launcher_item_main(void);
