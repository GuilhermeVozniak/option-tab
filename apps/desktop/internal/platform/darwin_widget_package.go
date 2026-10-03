//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework UniformTypeIdentifiers
#include <stdlib.h>
#include "darwin_widget_package.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"runtime/cgo"
	"time"
	"unsafe"
)

func NewWidgetPackageSource() WidgetPackageSource {
	return newWidgetPackageSource(chooseNativeWidgetPackage)
}

func chooseNativeWidgetPackage(ctx context.Context) (WidgetPackageFile, error) {
	if C.ot_widget_package_main_thread() != 0 {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "unavailable"}
	}
	if err := ctx.Err(); err != nil {
		return WidgetPackageFile{}, err
	}
	admission := cgo.NewHandle(ctx)
	defer admission.Delete()
	owner := C.ot_widget_package_start(C.uintptr_t(admission))
	if owner == nil {
		return WidgetPackageFile{}, &WidgetPackageError{Code: "unavailable"}
	}
	defer C.ot_widget_package_release(owner)
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	cancelled := false
	for {
		if ctx.Err() != nil && !cancelled {
			cancelled = true
			C.ot_widget_package_cancel(owner)
		}
		raw := C.ot_widget_package_poll(owner)
		if raw != nil {
			var reply struct {
				Name    string
				Archive []byte
				Error   string
			}
			data := C.GoString(raw)
			C.free(unsafe.Pointer(raw))
			if ctx.Err() != nil {
				return WidgetPackageFile{}, ctx.Err()
			}
			if json.Unmarshal([]byte(data), &reply) != nil {
				return WidgetPackageFile{}, &WidgetPackageError{Code: "ioFailure"}
			}
			if reply.Error == "cancelled" {
				return WidgetPackageFile{}, context.Canceled
			}
			if reply.Error != "" {
				return WidgetPackageFile{}, &WidgetPackageError{Code: reply.Error}
			}
			return WidgetPackageFile{Name: reply.Name, Archive: reply.Archive}, nil
		}
		<-timer.C
	}
}

//export goWidgetPackageContextCurrent
func goWidgetPackageContextCurrent(token C.uintptr_t) C.int {
	if token == 0 {
		return 0
	}
	ctx, ok := cgo.Handle(token).Value().(context.Context)
	if ok && ctx.Err() == nil {
		return 1
	}
	return 0
}
