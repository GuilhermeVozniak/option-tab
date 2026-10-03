//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework QuartzCore
#include <stdlib.h>
#include "darwin_material.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

type darwinMaterialNative struct{}

func (*darwinPlatform) CreatePreviewMaterial(panel DockPanel) (MaterialSurface, error) {
	preview, ok := panel.(*darwinWheelPanel)
	if !ok || preview == nil {
		return nil, ErrMaterialUnavailable
	}
	preview.mu.Lock()
	closed := preview.closed
	preview.mu.Unlock()
	if closed {
		return nil, ErrMaterialRetired
	}
	token := uint64(C.ot_material_preview(C.uint64_t(preview.token)))
	if token == 0 {
		return nil, ErrMaterialUnavailable
	}
	preview.mu.Lock()
	closed = preview.closed
	preview.mu.Unlock()
	if closed {
		C.ot_material_close(C.uint64_t(token))
		return nil, ErrMaterialRetired
	}
	return newMaterialSurface(token, darwinMaterialNative{}), nil
}

func (*darwinPlatform) CreateOverlayMaterial(host unsafe.Pointer) (MaterialSurface, error) {
	if host == nil {
		return nil, ErrMaterialUnavailable
	}
	token := uint64(C.ot_material_overlay(host))
	if token == 0 {
		return nil, ErrMaterialUnavailable
	}
	return newMaterialSurface(token, darwinMaterialNative{}), nil
}

//export goMaterialAdmission
func goMaterialAdmission(token C.uintptr_t) C.int {
	if token != 0 {
		if guard, ok := cgo.Handle(token).Value().(func() bool); ok && guard() {
			return 1
		}
	}
	return 0
}

func (darwinMaterialNative) Apply(token uint64, s MaterialStyle, guard func() bool) error {
	admission := cgo.NewHandle(guard)
	defer admission.Delete()
	theme := C.CString(s.Theme)
	defer C.free(unsafe.Pointer(theme))
	enabled := 0
	if s.Enabled {
		enabled = 1
	}
	if C.ot_material_apply(C.uint64_t(token), C.uint64_t(s.Scope.Session), C.uint64_t(s.Scope.Revision), C.int(enabled), theme, C.int(s.CornerRadiusPx), C.double(s.Rect.X), C.double(s.Rect.Y), C.double(s.Rect.W), C.double(s.Rect.H), C.uintptr_t(admission)) == 0 {
		return ErrMaterialUnavailable
	}
	return nil
}

func (darwinMaterialNative) Retire(token uint64, s MaterialScope, fade int, guard func() bool) error {
	admission := cgo.NewHandle(guard)
	defer admission.Delete()
	if C.ot_material_retire(C.uint64_t(token), C.uint64_t(s.Session), C.uint64_t(s.Revision), C.int(fade), C.uintptr_t(admission)) == 0 {
		return ErrMaterialRetired
	}
	return nil
}

func (darwinMaterialNative) Close(token uint64) error {
	C.ot_material_close(C.uint64_t(token))
	return nil
}
