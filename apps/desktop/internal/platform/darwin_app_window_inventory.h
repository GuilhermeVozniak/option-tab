#ifndef OPTIONTAB_APP_WINDOWS_H
#define OPTIONTAB_APP_WINDOWS_H

typedef struct {
  int identity_valid;
  int ax_readable;
  int ax_windows;
  int cg_candidates;
} OTAppWindowEvidence;

OTAppWindowEvidence ot_app_window_evidence(int pid);

#endif
