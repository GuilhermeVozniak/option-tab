//go:build !darwin

package platform

import "unsafe"

func (*stub) CreatePreviewMaterial(DockPanel) (MaterialSurface, error) {
	return nil, ErrMaterialUnavailable
}

func (*stub) CreateOverlayMaterial(unsafe.Pointer) (MaterialSurface, error) {
	return nil, ErrMaterialUnavailable
}
