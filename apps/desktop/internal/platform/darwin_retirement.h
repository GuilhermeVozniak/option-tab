#include <stdint.h>
// Numeric process-start identity; zero denotes unavailable, never a match.
int ot_retirement_identity(uint32_t window, int *pid, uint64_t *sec, uint64_t *usec);
int ot_retirement_matches(uint32_t window, int pid, uint64_t sec, uint64_t usec);
// NULL means failed full CG query. Caller owns the returned JSON string.
char *ot_retirement_inventory_json(void);
int ot_retirement_reappeared(uint32_t window, int pid, uint64_t sec, uint64_t usec);
