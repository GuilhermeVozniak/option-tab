package platform

import (
	"context"
	"time"

	"option-tab/internal/hotkey"
)

type DockInputKind string

const (
	DockInputLeftDown  DockInputKind = "leftDown"
	DockInputLeftUp    DockInputKind = "leftUp"
	DockInputMove      DockInputKind = "move"
	DockInputScroll    DockInputKind = "scroll"
	DockInputRightDown DockInputKind = "rightDown"
	DockInputRightUp   DockInputKind = "rightUp"
	DockInputCancelled DockInputKind = "cancelled"
	// DockInputClickSlop is shared by native ownership and the pure recognizer.
	DockInputClickSlop = 6.0
)

// DockInputEvent carries an immutable target revalidated by the source worker.
// Owned may be true only after synchronous native ownership AND fresh worker
// identity validation; the reducer cannot retroactively suppress native input.
// Cancellation is delivered even after ownership is released (Owned=false).
// Sequence increases across a generation. GestureID increases for each click
// down or new scroll stream. Phase-less scroll rearms after >250ms idle,
// including momentum in that timer; a fresh phased stream starts at began.
// Deltas are already normalized to visual direction by native code exactly once:
// positive DeltaY means show, negative means hide. DirectionInverted is metadata.
type DockInputEvent struct {
	Sequence, Generation, GestureID   uint64
	Timestamp                         time.Time
	DockPID                           int
	Reason                            string
	Kind                              DockInputKind
	Item                              DockItem
	PointerX, PointerY                float64
	DeltaX, DeltaY                    float64
	Button                            int
	Modifiers                         hotkey.ModSet
	Owned, Precise, DirectionInverted bool
	Phase, MomentumPhase              string // began, changed, ended, cancelled, none (or empty)
}

type DockInputPolicy struct {
	ClickToHide, ScrollShowHide, ModifiedRightClick bool
}

// DockInputTarget is copied into native storage; no Go pointer may be retained
// there. A nil Item immediately invalidates the cache. Targets expire after
// 150ms and generation must match before the native callback may own input.
type DockInputTarget struct {
	Generation uint64
	DockPID    int
	ObservedAt time.Time
	Item       *DockItem
}

// DockInputSource consumes a bounded latest-value target channel. Closing the
// channel cancels observation. Reconfiguration must cancel and join the old
// source before replacement; no native tap implementation is supplied here.
type DockInputSource interface {
	ObserveDockInput(context.Context, DockInputPolicy, <-chan DockInputTarget, func(DockInputEvent)) error
}

// DockInputGestureValidator reads native cancellation state without AX or UI.
// Completion keeps the token valid until replacement, cancellation or teardown.
type DockInputGestureValidator interface {
	DockInputGestureCurrent(generation, gestureID uint64) bool
}

// DockInputGestureAcknowledger releases pending native action admission after
// execution/refusal (or a terminal scroll that produced no intent).
type DockInputGestureAcknowledger interface {
	CompleteDockInputGesture(generation, gestureID uint64)
}
