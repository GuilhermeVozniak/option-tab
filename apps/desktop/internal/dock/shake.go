package dock

import (
	"math"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// ShakeIntent is evidence for a subsequent guarded action, not authorization to
// dispatch. The executor must revalidate generation/gesture, exact root identity,
// and latest AX position, then classify other windows and their modal children.
type ShakeIntent struct {
	Generation, GestureID uint64
	Kind                  string
	WindowID              domain.WindowID
	AppID                 domain.AppID
	WindowX, WindowY      float64
	Timestamp             time.Time
}

// ShakeRecognizer is a bounded, single-owner reducer. It stores one gesture and
// no sample history. Only AX movement correlated with the pointer can arm it.
// The caller must Reset on session loss and advance SetGeneration before a new
// source starts. Queued callbacks cannot select their own admitted generation.
type ShakeRecognizer struct {
	action                        string
	generation, sequence, gesture uint64
	active, fired                 bool
	last                          platform.WindowDragEvent
	started                       time.Time
	moves, reversals              int
	direction, extreme, path      float64
}

func NewShakeRecognizer(action string) *ShakeRecognizer {
	r := &ShakeRecognizer{}
	r.Configure(action)
	return r
}

// Configure discards an in-progress gesture when its action changes. Invalid
// values fail closed; no persistent configuration package is needed here.
func (r *ShakeRecognizer) Configure(action string) {
	if action != "minimizeOthers" && action != "closeOthers" {
		action = "none"
	}
	if action != r.action {
		r.Reset()
		r.action = action
	}
}

// SetGeneration admits strictly newer observation lifetimes. Generation zero
// and old generations cannot revive an obsolete source.
func (r *ShakeRecognizer) SetGeneration(generation uint64) {
	if generation == 0 || generation <= r.generation {
		return
	}
	r.Reset()
	r.generation = generation
	r.sequence = 0
	r.gesture = 0
}

// Reset retires the current gesture without forgetting sequence/gesture high
// water marks, so a delayed candidate cannot reopen that gesture.
func (r *ShakeRecognizer) Reset() {
	r.active = false
	r.fired = false
	r.last = platform.WindowDragEvent{}
	r.started = time.Time{}
	r.moves = 0
	r.reversals = 0
	r.direction = 0
	r.extreme = 0
	r.path = 0
}

func (r *ShakeRecognizer) Step(e platform.WindowDragEvent) *ShakeIntent {
	if e.Generation == 0 || e.Generation != r.generation || e.Sequence == 0 || e.Sequence <= r.sequence {
		return nil
	}
	r.sequence = e.Sequence
	if e.GestureID == 0 || e.GestureID < r.gesture {
		return nil
	}
	if e.Kind == "candidate" {
		if e.GestureID <= r.gesture {
			return nil
		}
		r.Reset()
		r.gesture = e.GestureID
		if r.action == "none" || !validShakeSample(e) {
			return nil
		}
		r.active = true
		r.restartEvidence(e)
		return nil
	}
	if e.GestureID != r.gesture || !r.active {
		return nil
	}
	if e.Kind != "moved" {
		r.Reset()
		return nil
	}
	if r.fired {
		return nil
	}
	if !validShakeSample(e) || e.Window != r.last.Window || !e.Timestamp.After(r.last.Timestamp) {
		r.Reset()
		return nil
	}
	dx, dy := e.WindowX-r.last.WindowX, e.WindowY-r.last.WindowY
	px, py := e.PointerX-r.last.PointerX, e.PointerY-r.last.PointerY
	// A two-point absolute allowance covers AX rounding; at larger distances
	// require displacement to agree within 20%. Opposite horizontal directions
	// and vertical-dominant movement never contribute evidence.
	correlated := dx != 0 && px*dx > 0 && math.Abs(dy) <= math.Abs(dx) && math.Abs(py) <= math.Abs(px) && math.Abs(dx-px) <= max(2, math.Abs(dx)*0.2) && math.Abs(dy-py) <= max(2, math.Abs(dy)*0.2)
	if !correlated || e.Timestamp.Sub(r.last.Timestamp) > 250*time.Millisecond || e.Timestamp.Sub(r.started) > 700*time.Millisecond {
		r.restartEvidence(e)
		return nil
	}
	r.last = e
	r.moves++
	r.path += math.Abs(dx)
	if r.direction == 0 {
		r.direction = math.Copysign(1, dx)
		r.extreme = e.WindowX
	} else if (e.WindowX-r.extreme)*r.direction > 0 {
		r.extreme = e.WindowX
	} else if (r.extreme-e.WindowX)*r.direction >= 24 {
		// Small backtracking cannot count as another reversal. Keep the extremum
		// until a full 24-point leg has been observed, even across several samples.
		if r.moves > 2 {
			r.reversals++
		}
		r.direction = -r.direction
		r.extreme = e.WindowX
	}
	if r.moves < 2 || r.reversals < 4 || r.path < 180 {
		return nil
	}
	r.fired = true
	return &ShakeIntent{Generation: e.Generation, GestureID: e.GestureID, Kind: r.action, WindowID: e.Window.WindowID, AppID: e.Window.AppID, WindowX: e.WindowX, WindowY: e.WindowY, Timestamp: e.Timestamp}
}

func (r *ShakeRecognizer) restartEvidence(e platform.WindowDragEvent) {
	r.last = e
	r.started = e.Timestamp
	r.moves = 0
	r.reversals = 0
	r.direction = 0
	r.extreme = e.WindowX
	r.path = 0
}

func validShakeSample(e platform.WindowDragEvent) bool {
	if e.Timestamp.IsZero() || e.Window.WindowID == 0 || e.Window.AppID <= 0 || e.Window.Role != "AXWindow" || e.Window.Subrole != "AXStandardWindow" {
		return false
	}
	for _, v := range [...]float64{e.PointerX, e.PointerY, e.WindowX, e.WindowY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
