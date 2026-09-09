//go:build darwin

package platform

/*
#include "darwin_app_window_inventory.h"
*/
import "C"

import (
	"math"
	"os"

	"option-tab/internal/domain"
)

func (p *darwinPlatform) AppWindowPresence(app domain.AppID) WindowPresence {
	if app <= 0 || int64(app) > math.MaxInt32 || int(app) == os.Getpid() {
		return WindowsUnknown
	}
	evidence := C.ot_app_window_evidence(C.int(app))
	return applicationWindowPresence(evidence.identity_valid == 1, evidence.ax_readable == 1, int(evidence.ax_windows), int(evidence.cg_candidates))
}
