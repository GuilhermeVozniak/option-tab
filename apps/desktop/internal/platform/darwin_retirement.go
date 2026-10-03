//go:build darwin

package platform

/*
#include "darwin_retirement.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"option-tab/internal/domain"
)

var retiredWindows windowRetirements

func nativeWindowIdentity(window domain.WindowID) windowIdentity {
	var pid C.int
	var sec, usec C.uint64_t
	if C.ot_retirement_identity(C.uint32_t(window), &pid, &sec, &usec) == 0 {
		return windowIdentity{}
	}
	return windowIdentity{window, int(pid), uint64(sec), uint64(usec)}
}

func nativeRetirementInventory() ([]windowIdentity, bool) {
	data := C.ot_retirement_inventory_json()
	if data == nil {
		return nil, false
	}
	defer C.free(unsafe.Pointer(data))
	var identities []windowIdentity
	if json.Unmarshal([]byte(C.GoString(data)), &identities) != nil {
		return nil, false
	}
	return identities, true
}

func nativeWindowReappeared(id windowIdentity) bool {
	return C.ot_retirement_reappeared(C.uint32_t(id.Window), C.int(id.PID), C.uint64_t(id.StartSec), C.uint64_t(id.StartUsec)) != 0
}
