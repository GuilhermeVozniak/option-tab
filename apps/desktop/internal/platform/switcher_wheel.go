package platform

import (
	"context"
	"unsafe"
)

// SwitcherWheel attaches only an input owner to the existing overlay window;
// it never shows, activates or reparents the Wails host.
type SwitcherWheel interface {
	DockPanelWheelSource
	DockPanelWheelGestureValidator
	DockPanelWheelGestureAcknowledger
	Close() error
	Done() <-chan struct{}
}

type SwitcherWheelHost interface {
	CreateSwitcherWheel(unsafe.Pointer) (SwitcherWheel, error)
}

// GuardedPointerPerformer preserves the existing five pointer action semantics,
// including toggle minimize/fullscreen, after final captured identity admission.
type GuardedPointerPerformer interface {
	PerformPointerAction(context.Context, string, AutomationWindowIdentity, func() error) error
}
