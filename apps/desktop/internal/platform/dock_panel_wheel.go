package platform

import (
	"time"

	"option-tab/internal/domain"
)

type PreviewRegion struct {
	WindowID domain.WindowID
	AppID    domain.AppID
	Bounds   domain.Bounds
}

type DockPanelWheelPolicy struct {
	Session  uint64
	Revision uint64
	Enabled  bool
	Regions  []PreviewRegion
}

type DockPanelWheelEvent struct {
	Session, Revision, Sequence, GestureID uint64
	Timestamp                              time.Time
	WindowID                               domain.WindowID
	AppID                                  domain.AppID
	PanelX, PanelY, DeltaX, DeltaY         float64
	Owned, Precise, DirectionInverted      bool
	Phase, MomentumPhase                   string
	Reason                                 string
}

type DockPanelWheelSource interface {
	SetDockPanelWheelPolicy(DockPanelWheelPolicy, func(DockPanelWheelEvent)) error
}

// Validation remains true through normal completion until acknowledged; explicit
// cancellation, policy changes, hide and close invalidate it immediately.
type DockPanelWheelGestureValidator interface {
	ValidateDockPanelWheelGesture(session, revision, gestureID uint64) bool
}
type DockPanelWheelGestureAcknowledger interface {
	CompleteDockPanelWheelGesture(session, revision, gestureID uint64)
}
