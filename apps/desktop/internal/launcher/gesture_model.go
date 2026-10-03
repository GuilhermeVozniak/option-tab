package launcher

import (
	"math"
	"time"
)

// Thresholds match Dock panel scroll semantics; this model has no window/action dependency.
const (
	GestureLockDistance          = 8.0
	GestureActionDistance        = 80.0
	GestureIdle                  = 250 * time.Millisecond
	GestureMagnificationDistance = .15
)

type (
	GesturePolicy struct {
		Enabled                      bool
		Edge, Pinch, Primary, Toward string
	}
	GesturePacket struct {
		Scope
		Admission, Sequence, GestureID                uint64
		Timestamp                                     time.Time
		PanelX, PanelY, DeltaX, DeltaY, Magnification float64
		Kind, Phase, MomentumPhase                    string
		Precise, Owned                                bool
	}
	GestureIntent struct {
		Scope
		Admission, GestureID uint64
		Action               string
	}
	GestureReducer struct {
		scope                        Scope
		policy                       GesturePolicy
		admission, sequence, gesture uint64
		last                         time.Time
		kind                         string
		x, y                         float64
		axis                         byte
		active, fired                bool
	}
)

func (r *GestureReducer) SetScope(scope Scope, p GesturePolicy) uint64 {
	if r.admission != 0 && r.scope == scope && r.policy == p {
		return r.admission
	}
	r.scope = scope
	r.policy = p
	r.admission++
	r.sequence, r.gesture = 0, 0
	r.active = false
	r.last = time.Time{}
	return r.admission
}

func validGestureScope(s Scope) bool {
	return s.Epoch != 0 && s.Session != 0 && s.Revision != 0 && s.DisplayUUID != ""
}

func signedGestureSum(total, delta float64) float64 {
	if total != 0 && delta != 0 && math.Signbit(total) != math.Signbit(delta) {
		return delta
	}
	return total + delta
}

func gestureFinite(v ...float64) bool {
	for _, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

func gestureInverse(s string) string {
	switch s {
	case "previous":
		return "next"
	case "next":
		return "previous"
	case "show":
		return "hide"
	case "hide":
		return "show"
	}
	return "none"
}

func validGesturePolicy(p GesturePolicy) bool {
	return (p.Edge == "top" || p.Edge == "bottom" || p.Edge == "left" || p.Edge == "right") && (p.Pinch == "none" || p.Pinch == "show" || p.Pinch == "hide") && (p.Primary == "none" || p.Primary == "previous" || p.Primary == "next") && (p.Toward == "none" || p.Toward == "show" || p.Toward == "hide")
}

func (r *GestureReducer) Step(e GesturePacket) *GestureIntent {
	if e.Scope != r.scope || e.Admission != r.admission || !validGestureScope(e.Scope) || e.Sequence == 0 || e.Sequence <= r.sequence {
		return nil
	}
	r.sequence = e.Sequence
	if e.GestureID == 0 || e.GestureID < r.gesture {
		return nil
	}
	fresh := e.GestureID > r.gesture
	if fresh {
		r.gesture = e.GestureID
		r.active = false
		r.fired = false
		r.x, r.y, r.axis = 0, 0, 0
		r.kind = e.Kind
	}
	if !r.policy.Enabled || !validGesturePolicy(r.policy) || !e.Owned || e.Timestamp.IsZero() || (!r.last.IsZero() && e.Timestamp.Before(r.last)) || !gestureFinite(e.PanelX, e.PanelY, e.DeltaX, e.DeltaY, e.Magnification) || (e.Kind != "scroll" && e.Kind != "swipe" && e.Kind != "magnify") || (e.Kind == "scroll" && !e.Precise) || (e.MomentumPhase != "" && e.MomentumPhase != "none") || (e.Phase != "began" && e.Phase != "changed") || e.Kind != r.kind {
		r.active = false
		return nil
	}
	if !fresh && !r.last.IsZero() && e.Timestamp.Sub(r.last) > GestureIdle {
		r.active = false
	}
	r.last = e.Timestamp
	if fresh {
		r.active = true
	}
	if !r.active || r.fired {
		return nil
	}
	action := "none"
	if e.Kind == "magnify" {
		r.x = signedGestureSum(r.x, e.Magnification)
		if math.Abs(r.x) < GestureMagnificationDistance {
			return nil
		}
		action = r.policy.Pinch
		if r.x < 0 {
			action = gestureInverse(action)
		}
	} else {
		lock, threshold := GestureLockDistance, GestureActionDistance
		if e.Kind == "swipe" {
			lock, threshold = 1, 1
		}
		switch r.axis {
		case 0:
			r.x = signedGestureSum(r.x, e.DeltaX)
			r.y = signedGestureSum(r.y, e.DeltaY)
			if math.Max(math.Abs(r.x), math.Abs(r.y)) < lock {
				return nil
			}
			r.axis = 'y'
			if math.Abs(r.x) > math.Abs(r.y) {
				r.axis = 'x'
			}
		case 'x':
			r.x = signedGestureSum(r.x, e.DeltaX)
		default:
			r.y = signedGestureSum(r.y, e.DeltaY)
		}
		value := r.y
		if r.axis == 'x' {
			value = r.x
		}
		if math.Abs(value) < threshold {
			return nil
		}
		vertical := r.policy.Edge == "left" || r.policy.Edge == "right"
		if (vertical && r.axis == 'y') || (!vertical && r.axis == 'x') {
			action = r.policy.Primary
			if value < 0 {
				action = gestureInverse(action)
			}
		} else {
			toward := (r.policy.Edge == "left" || r.policy.Edge == "top") && value < 0 || (r.policy.Edge == "right" || r.policy.Edge == "bottom") && value > 0
			if toward {
				action = r.policy.Toward
			}
		}
	}
	r.fired = true
	if action == "none" {
		return nil
	}
	return &GestureIntent{Scope: r.scope, Admission: r.admission, GestureID: r.gesture, Action: action}
}
