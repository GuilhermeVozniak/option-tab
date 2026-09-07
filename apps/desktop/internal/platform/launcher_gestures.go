package platform

import (
	"time"

	"option-tab/internal/domain"
)

type (
	LauncherGesturePolicy struct {
		Epoch, Session, Revision, Admission uint64
		DisplayUUID                         string
		Enabled, Scroll, Magnify, Swipe     bool
		Bounds                              domain.Bounds
	}
	LauncherGestureEvent struct {
		Epoch, Session, Revision, Admission, Sequence, GestureID uint64
		DisplayUUID                                              string
		Timestamp                                                time.Time
		Kind, Phase, MomentumPhase                               string
		PanelX, PanelY, DeltaX, DeltaY, Magnification            float64
		Owned, Precise                                           bool
	}
	LauncherGestureSource interface {
		SetLauncherGesturePolicy(LauncherGesturePolicy, func(LauncherGestureEvent)) error
	}
	LauncherGestureValidator interface {
		ValidateLauncherGesture(epoch, session, revision, admission, gestureID uint64) bool
	}
	LauncherGestureAcknowledger interface {
		CompleteLauncherGesture(epoch, session, revision, admission, gestureID uint64)
	}
	// Receipt closes only after polling and any already-admitted callback return.
	LauncherGestureDrainer interface{ LauncherGestureDone() <-chan struct{} }
)
