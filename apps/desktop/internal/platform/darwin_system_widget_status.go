//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework IOKit -framework SystemConfiguration
#include <stdlib.h>
#include "darwin_system_widget_status.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unsafe"
)

type (
	darwinBatterySource struct{}
	darwinNetworkSource struct{}
)

func NewBatterySource() BatterySource { return darwinBatterySource{} }
func NewNetworkSource() NetworkSource { return darwinNetworkSource{} }
func systemNativeJSON(raw *C.char, out any) bool {
	if raw == nil {
		return false
	}
	defer C.free(unsafe.Pointer(raw))
	return json.Unmarshal([]byte(C.GoString(raw)), out) == nil
}

func (darwinBatterySource) ObserveBattery(ctx context.Context, emit func(BatterySnapshot)) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid battery observer")
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	return observeSystemBattery(ctx, emit, func() BatterySnapshot {
		// Reset after each read so delayed native calls cannot cause catch-up polls.
		defer ticker.Reset(5 * time.Second)
		var s BatterySnapshot
		if !systemNativeJSON(C.ot_widget_battery_read(), &s) {
			return BatterySnapshot{Status: "unavailable", Reason: "nativeReadFailed", PowerSource: "unknown"}
		}
		return s
	}, ticker.C)
}

func (darwinNetworkSource) ObserveNetwork(ctx context.Context, usage bool, emit func(NetworkSnapshot)) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid network observer")
	}
	interval := 2 * time.Second
	if usage {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	return observeSystemNetwork(ctx, usage, emit, systemNetworkOps{Ticks: ticker.C, Now: time.Now, Status: func() systemNetworkState {
		var raw struct {
			NetworkSnapshot
			Interface string
			Index     uint32
		}
		if !systemNativeJSON(C.ot_widget_network_status(), &raw) {
			return systemNetworkState{Snapshot: NetworkSnapshot{Status: "unavailable", Reason: "nativeReadFailed", Category: "unknown"}}
		}
		return systemNetworkState{Snapshot: raw.NetworkSnapshot, Interface: raw.Interface, Index: raw.Index}
	}, Counters: func(state systemNetworkState) systemCounter {
		var result systemCounter
		name := C.CString(state.Interface)
		defer C.free(unsafe.Pointer(name))
		if !systemNativeJSON(C.ot_widget_network_counters(name, C.uint32_t(state.Index)), &result) {
			return systemCounter{}
		}
		return result
	}})
}
