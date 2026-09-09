#include <stdint.h>
uint64_t ot_automation_start(void);
int ot_automation_status(uint64_t server);
char *ot_automation_pop(uint64_t server);
int ot_automation_current(uint64_t server, uint64_t request);
void ot_automation_complete(uint64_t server, uint64_t request, const char *json, const char *code, const char *message);
void ot_automation_stop(uint64_t server);
// Must be called on the AppKit main thread before its loop exits. Never joins Go.
void ot_automation_drain_main(uint64_t server);
