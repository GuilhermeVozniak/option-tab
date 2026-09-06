//go:build darwin

package platform

/*
#include "darwin_dock_panel.h"
*/
import "C"

import (
	"unsafe"

	"option-tab/internal/domain"
)

type darwinDockPanelNative struct{}

func (p *darwinPlatform) CreateDockPanel(host unsafe.Pointer) (DockPanel, error) {
	if host == nil {
		return nil, ErrDockPanelHostClosed
	}
	token := uint64(C.ot_dock_panel_create(host))
	if token == 0 {
		return nil, ErrDockPanelHostClosed
	}
	return newDockPanel(token, darwinDockPanelNative{}), nil
}

func (darwinDockPanelNative) Show(token uint64, b domain.Bounds) error {
	if C.ot_dock_panel_show(C.uint64_t(token), C.double(b.X), C.double(b.Y), C.double(b.W), C.double(b.H)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func (darwinDockPanelNative) Hide(token uint64) error {
	if C.ot_dock_panel_hide(C.uint64_t(token)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func (darwinDockPanelNative) Close(token uint64) error {
	C.ot_dock_panel_close(C.uint64_t(token))
	return nil
}
