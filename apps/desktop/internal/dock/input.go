package dock

import (
	"math"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
)

// Intent preserves the native ownership token and exact app. The executor must
// revalidate this token and app identity immediately before dispatch; producing
// an intent does not itself suppress an event or authorize a native action.
type Intent struct {
	Generation, GestureID uint64
	Kind                  string
	AppID                 domain.AppID
}

// InputReducer is a single-owner, constant-space recognizer for already-owned
// native Dock input. SetGeneration admits a source lifetime explicitly; events
// cannot select their own generation. It never performs UI or native actions.
type InputReducer struct {
	policy                        platform.DockInputPolicy
	self                          domain.AppID
	generation, sequence, gesture uint64
	active                        bool
	start                         platform.DockInputEvent
	lastAt, scrollAt              time.Time
	accumulated                   float64
}

func NewInputReducer(policy platform.DockInputPolicy, self domain.AppID) *InputReducer {
	return &InputReducer{policy: policy, self: self}
}

func (r *InputReducer) Configure(policy platform.DockInputPolicy) {
	if policy != r.policy {
		r.Reset()
		r.policy = policy
	}
}

// SetGeneration retires old ownership without letting a delayed old source
// replace the current one. The caller must advance it on source invalidation.
func (r *InputReducer) SetGeneration(generation uint64) {
	if generation == 0 || generation <= r.generation {
		return
	}
	r.Reset()
	r.generation = generation
	r.sequence = 0
	r.gesture = 0
	r.lastAt = time.Time{}
	r.scrollAt = time.Time{}
}

// Reset retires the gesture but preserves replay protection, including the
// legacy scroll idle timer. A released/cancelled ID can never arm again.
func (r *InputReducer) Reset() {
	r.active = false
	r.start = platform.DockInputEvent{}
	r.accumulated = 0
}

func (r *InputReducer) Step(e platform.DockInputEvent) []Intent {
	if e.Generation == 0 || e.Generation != r.generation || e.Sequence == 0 || e.Sequence <= r.sequence {
		return nil
	}
	r.sequence = e.Sequence
	if e.GestureID == 0 || e.GestureID < r.gesture {
		return nil
	}
	if e.Timestamp.IsZero() || (!r.lastAt.IsZero() && e.Timestamp.Before(r.lastAt)) {
		r.Reset()
		return nil
	}
	r.lastAt = e.Timestamp
	previousScroll := r.scrollAt
	if e.Kind == platform.DockInputScroll {
		r.scrollAt = e.Timestamp
	}
	// Process cancellation before Owned: native ownership has already been
	// released when the worker delivers the terminal sample.
	if e.Kind == platform.DockInputCancelled {
		r.gesture = e.GestureID
		r.Reset()
		return nil
	}
	fresh := e.GestureID > r.gesture
	if fresh {
		r.Reset()
		r.gesture = e.GestureID
	}
	if !e.Owned || !r.validTarget(e) {
		r.Reset()
		return nil
	}
	if fresh {
		switch e.Kind {
		case platform.DockInputLeftDown:
			if !r.policy.ClickToHide || e.Modifiers != 0 || e.Button != 0 {
				return nil
			}
		case platform.DockInputRightDown:
			if !r.policy.ModifiedRightClick || rightClickAction(e.Modifiers) == "" || e.Button != 1 {
				return nil
			}
		case platform.DockInputScroll:
			if !r.policy.ScrollShowHide || e.Modifiers != 0 || !scrollPhaseActive(e) {
				return nil
			}
			if e.Phase != "began" && !previousScroll.IsZero() && e.Timestamp.Sub(previousScroll) <= 250*time.Millisecond {
				return nil
			}
		default:
			return nil
		}
		r.start = e
		r.active = true
	}
	if !r.active {
		return nil
	}
	if !sameInputTarget(r.start.Item, e.Item) || e.Modifiers != r.start.Modifiers {
		r.Reset()
		return nil
	}
	if r.start.Kind == platform.DockInputScroll {
		if e.Kind != platform.DockInputScroll || e.Precise != r.start.Precise || !scrollPhaseActive(e) {
			r.Reset()
			return nil
		}
		if !previousScroll.IsZero() && e.Timestamp.Sub(previousScroll) > 250*time.Millisecond {
			r.accumulated = 0
		}
		if !finiteInput(e.DeltaX, e.DeltaY) || math.Abs(e.DeltaX) > math.Abs(e.DeltaY) {
			r.Reset()
			return nil
		}
		if e.DeltaY == 0 {
			return nil
		}
		if r.accumulated*e.DeltaY < 0 {
			r.accumulated = 0
		}
		r.accumulated += e.DeltaY
		threshold := 1.0
		if e.Precise {
			threshold = 80
		}
		if math.Abs(r.accumulated) < threshold {
			return nil
		}
		kind := "show"
		if r.accumulated < 0 {
			kind = "hide"
		}
		return r.emit(kind)
	}
	if e.Button != r.start.Button || math.Hypot(e.PointerX-r.start.PointerX, e.PointerY-r.start.PointerY) > platform.DockInputClickSlop || !r.start.Item.Bounds.ContainsPoint(e.PointerX, e.PointerY) {
		r.Reset()
		return nil
	}
	if e.Kind == r.start.Kind && fresh {
		return nil
	}
	if e.Kind == platform.DockInputMove {
		return nil
	}
	if r.start.Kind == platform.DockInputLeftDown && e.Kind == platform.DockInputLeftUp {
		return r.emit("hide")
	}
	if r.start.Kind == platform.DockInputRightDown && e.Kind == platform.DockInputRightUp {
		return r.emit(rightClickAction(e.Modifiers))
	}
	r.Reset()
	return nil
}

func (r *InputReducer) emit(kind string) []Intent {
	intent := Intent{Generation: r.generation, GestureID: r.gesture, Kind: kind, AppID: r.start.Item.AppID}
	r.Reset()
	return []Intent{intent}
}

func (r *InputReducer) validTarget(e platform.DockInputEvent) bool {
	item := e.Item
	return r.self > 0 && item.AppID > 0 && item.AppID != r.self && item.BundleID != "" && item.BundleID != "com.apple.finder" && item.Path != "" && item.ScreenID != 0 && validSize(item.Bounds.W, item.Bounds.H) && finiteInput(item.Bounds.X, item.Bounds.Y, e.PointerX, e.PointerY) && item.Bounds.ContainsPoint(e.PointerX, e.PointerY)
}

func sameInputTarget(a, b platform.DockItem) bool {
	return a.AppID == b.AppID && a.BundleID == b.BundleID && a.Path == b.Path && a.ScreenID == b.ScreenID && a.Edge == b.Edge
}

func rightClickAction(mods hotkey.ModSet) string {
	command := hotkey.ModSet(0).With(hotkey.ModCommand)
	switch mods {
	case command:
		return "quit"
	case command.With(hotkey.ModOption):
		return "forceQuit"
	default:
		return ""
	}
}

func scrollPhaseActive(e platform.DockInputEvent) bool {
	return (e.MomentumPhase == "" || e.MomentumPhase == "none") && (e.Phase == "" || e.Phase == "none" || e.Phase == "began" || e.Phase == "changed")
}

func finiteInput(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
