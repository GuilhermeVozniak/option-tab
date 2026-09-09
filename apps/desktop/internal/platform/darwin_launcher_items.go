//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "darwin_launcher_items.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"runtime/cgo"
	"time"
	"unsafe"
)

type launcherNativeRecord struct {
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	SelectedPath string `json:"selectedPath"`
	BundleID     string `json:"bundleID"`
	Bookmark     []byte `json:"bookmark"`
	Archive      []byte `json:"archive,omitempty"`
	Error        string `json:"error,omitempty"`
}

func launcherNativeWait(ctx context.Context, p unsafe.Pointer) (launcherNativeRecord, error) {
	if p == nil {
		return launcherNativeRecord{}, launcherItemError("unavailable")
	}
	defer C.ot_launcher_item_release(p)
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	cancelled := false
	for {
		if ctx.Err() != nil && !cancelled {
			cancelled = true
			C.ot_launcher_item_cancel(p)
		}
		if raw := C.ot_launcher_item_poll(p); raw != nil {
			data := C.GoString(raw)
			C.free(unsafe.Pointer(raw))
			if err := ctx.Err(); err != nil {
				return launcherNativeRecord{}, err
			}
			var r launcherNativeRecord
			if json.Unmarshal([]byte(data), &r) != nil {
				return r, launcherItemError("unavailable")
			}
			if r.Error == "cancelled" {
				return r, context.Canceled
			}
			if r.Error != "" {
				return r, launcherItemError(r.Error)
			}
			return r, nil
		}
		<-timer.C
	}
}

func chooseLauncherNative(ctx context.Context, kind string) (launcherNativeRecord, error) {
	if C.ot_launcher_item_main() != 0 {
		return launcherNativeRecord{}, launcherItemError("unavailable")
	}
	guard := cgo.NewHandle(func() bool { return ctx.Err() == nil })
	defer guard.Delete()
	k := C.CString(kind)
	defer C.free(unsafe.Pointer(k))
	return launcherNativeWait(ctx, C.ot_launcher_item_choose(k, C.uintptr_t(guard)))
}

type launcherNativeScope struct {
	pointer     unsafe.Pointer
	path        string
	fingerprint string
	label       string
	process     ProcessIdentity
}

func resolveLauncherNative(record launcherNativeRecord) (*launcherNativeScope, error) {
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	raw := C.CString(string(data))
	defer C.free(unsafe.Pointer(raw))
	var failure *C.char
	p := C.ot_launcher_item_resolve(raw, &failure)
	if failure != nil {
		code := C.GoString(failure)
		C.free(unsafe.Pointer(failure))
		return nil, launcherItemError(code)
	}
	if p == nil {
		return nil, launcherItemError("unavailable")
	}
	info := C.ot_launcher_item_scope_info(p)
	var dto struct {
		Path, Fingerprint, ProcessState, Label string
		Process                                ProcessIdentity
	}
	err = json.Unmarshal([]byte(C.GoString(info)), &dto)
	C.free(unsafe.Pointer(info))
	if err != nil {
		C.ot_launcher_item_scope_release(p)
		return nil, err
	}
	if dto.ProcessState == "ambiguous" || dto.ProcessState == "unavailable" {
		C.ot_launcher_item_scope_release(p)
		return nil, launcherItemError("unavailable")
	}
	return &launcherNativeScope{pointer: p, path: dto.Path, fingerprint: dto.Fingerprint, process: dto.Process, label: boundedLauncherLabel(dto.Label, "Application")}, nil
}
func (s *launcherNativeScope) close()        { C.ot_launcher_item_scope_release(s.pointer) }
func (s *launcherNativeScope) current() bool { return C.ot_launcher_item_same(s.pointer) != 0 }
func openLauncherNative(ctx context.Context, scope *launcherNativeScope, a LauncherItemAction, link string, guard func() error) error {
	if C.ot_launcher_item_main() != 0 {
		return launcherItemError("unavailable")
	}
	check := cgo.NewHandle(func() bool { return ctx.Err() == nil && guard() == nil && ctx.Err() == nil })
	defer check.Delete()
	kind := C.CString(a.Kind)
	defer C.free(unsafe.Pointer(kind))
	display := C.CString(a.DisplayUUID)
	defer C.free(unsafe.Pointer(display))
	url := C.CString(link)
	defer C.free(unsafe.Pointer(url))
	var p unsafe.Pointer
	if scope != nil {
		p = scope.pointer
	}
	_, err := launcherNativeWait(ctx, C.ot_launcher_item_open(p, kind, display, C.uint64_t(a.PanelToken), C.int(a.Process.PID), C.uint64_t(a.Process.StartSeconds), C.uint64_t(a.Process.StartMicros), url, C.uintptr_t(check)))
	return err
}

//export goLauncherItemCurrent
func goLauncherItemCurrent(token C.uintptr_t) C.int {
	if token != 0 {
		if check, ok := cgo.Handle(token).Value().(func() bool); ok && check() {
			return 1
		}
	}
	return 0
}
