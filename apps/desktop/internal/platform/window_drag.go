package platform

import (
	"context"
	"time"

	"option-tab/internal/domain"
)

// WindowRole identifies an AX root window resolved by the observation worker.
// Role metadata alone is not sufficient to authorize a bulk action: attached
// sheets/modal relationships must also be checked by the eventual action service.
type WindowRole struct {
	WindowID      domain.WindowID
	AppID         domain.AppID
	Role, Subrole string
}

// WindowDragEvent uses global top-left logical points. Generation is assigned
// by the lifecycle owner; GestureID increases for each down within a generation,
// and Sequence increases for every event in that generation. The native worker
// must sample the exact resolved AX root; pointer motion alone is not evidence.
// Kind is candidate, moved, up, or cancelled. Candidate carries the initial
// pointer and AX position. A cancellation may omit Window and coordinates.
type WindowDragEvent struct {
	Sequence, Generation, GestureID      uint64
	Timestamp                            time.Time
	Kind                                 string
	Window                               WindowRole
	PointerX, PointerY, WindowX, WindowY float64
}

// WindowDragObservationSource is passive: observation never suppresses input.
// Cancellation must retire all native resources before this method returns.
type WindowDragObservationSource interface {
	ObserveWindowDrags(context.Context, func(WindowDragEvent)) error
}

// ActionWindowRole is a fresh classification. False RelationshipsKnown or
// RootConfirmed is unresolved, never permission to perform a bulk action.
type ActionWindowRole struct {
	SelfApplication bool
	WindowRole
	RootConfirmed, RelationshipsKnown      bool
	Modal, HasAttachedSheet, HasModalChild bool
	ParentWindowID                         domain.WindowID
	Bounds                                 domain.Bounds
	Reason                                 string
}
type WindowRoleSource interface {
	ActionWindowRoles() ([]ActionWindowRole, error)
}

// The implementation rechecks exact identity and all role/relationship facts
// immediately before performing close or setMinimized (never a toggle).
type OtherWindowPerformer interface {
	PerformOtherWindowAction(kind string, id domain.WindowID, app domain.AppID) error
}
type WindowDragValidation struct {
	Generation, GestureID uint64
	WindowID              domain.WindowID
	AppID                 domain.AppID
	WindowX, WindowY      float64
	ObservedAt            time.Time
}
type WindowDragGestureValidator interface {
	WindowDragGestureCurrent(WindowDragValidation) bool
}

// GuardedOtherWindowPerformer evaluates guard after all native target lookups,
// immediately before AX mutation. A guarded bulk action must not fall back to
// the plain performer. Guard refusal is returned unchanged.
type GuardedOtherWindowPerformer interface {
	PerformOtherWindowActionGuarded(kind string, id domain.WindowID, app domain.AppID, guard func() error) error
}
