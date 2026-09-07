package platform

import (
	"context"
	"unsafe"

	"option-tab/internal/domain"
)

// MaterialScope orders visual mutations within one exact native host lifetime.
// Revision is independent of capture and renderer content revisions.
type MaterialScope struct {
	Session, Revision uint64
}

type MaterialStyle struct {
	Scope          MaterialScope
	Enabled        bool
	Theme          string
	CornerRadiusPx int
	Rect           domain.Bounds // Host-local, top-left logical points.
}

type MaterialStatus struct {
	Session  uint64 `json:"session"`
	Revision uint64 `json:"revision"`
	State    string `json:"state"` // system|solid|unavailable
	Reason   string `json:"reason,omitempty"`
}

// MaterialSurface never changes its host's input or activation policy. Calls
// must occur outside App/window locks because native adapters dispatch to AppKit.
type MaterialSurface interface {
	Apply(context.Context, MaterialStyle) (MaterialStatus, error)
	Retire(MaterialScope, int) error // Terminal for the session; fade is 0 or 180 ms.
	Close() error
}

type PreviewMaterialHost interface {
	CreatePreviewMaterial(DockPanel) (MaterialSurface, error)
}

type OverlayMaterialHost interface {
	CreateOverlayMaterial(unsafe.Pointer) (MaterialSurface, error)
}
