#ifndef OT_AUTOMATION_ACTIVE_WINDOW_H
#define OT_AUTOMATION_ACTIVE_WINDOW_H
#include <stdint.h>
typedef struct {
  uint32_t window;
  int pid;
  uint64_t sec, usec;
} OTAutomationIdentity;
int ot_automation_process_identity(int pid, OTAutomationIdentity *out);
int ot_automation_window_identity(uint32_t window, OTAutomationIdentity *out);
int ot_automation_window_current(OTAutomationIdentity identity);
char *ot_automation_active_window(void);
int ot_automation_window_action(int action, OTAutomationIdentity identity,
                                int desired, uintptr_t guard);
#endif
