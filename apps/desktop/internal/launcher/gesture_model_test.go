package launcher

import (
	"math"
	"testing"
	"time"
)

func gestureFixture() (*GestureReducer, GesturePacket) {
	r := &GestureReducer{}
	s := Scope{Epoch: 1, DisplayUUID: "d", Session: 1, Revision: 1}
	a := r.SetScope(s, GesturePolicy{Enabled: true, Edge: "bottom", Pinch: "show", Primary: "next", Toward: "hide"})
	return r, GesturePacket{Scope: s, Admission: a, Sequence: 1, GestureID: 1, Timestamp: time.Unix(1, 0), Kind: "scroll", Phase: "began", Owned: true, Precise: true}
}

func TestGestureModelAxesThresholdAndOnce(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		t.Run(edge, func(t *testing.T) {
			r, e := gestureFixture()
			p := r.policy
			p.Edge = edge
			e.Admission = r.SetScope(e.Scope, p)
			e.DeltaX = 80
			if edge == "left" || edge == "right" {
				e.DeltaX = 0
				e.DeltaY = 80
			}
			got := r.Step(e)
			if got == nil || got.Action != "next" {
				t.Fatal(got)
			}
			e.Sequence++
			if r.Step(e) != nil {
				t.Fatal("twice")
			}
			e.Sequence++
			e.Phase = "ended"
			r.Step(e)
			e.Sequence++
			e.Phase = "began"
			if r.Step(e) != nil {
				t.Fatal("terminal replay")
			}
		})
	}
}

func TestGestureModelRefusalReversalABA(t *testing.T) {
	r, e := gestureFixture()
	e.DeltaX = 60
	if r.Step(e) != nil {
		t.Fatal("early")
	}
	e.Sequence++
	e.DeltaX = -30
	if r.Step(e) != nil {
		t.Fatal("reversal accumulated")
	}
	e.Sequence++
	e.DeltaX = -50
	if got := r.Step(e); got == nil || got.Action != "previous" {
		t.Fatal(got)
	}
	for _, mode := range []string{"momentum", "unowned", "coarse", "nan", "timestamp", "idle", "ABA"} {
		t.Run(mode, func(t *testing.T) {
			r, e := gestureFixture()
			e.DeltaX = 10
			r.Step(e)
			e.Sequence++
			e.DeltaX = 80
			switch mode {
			case "momentum":
				e.MomentumPhase = "began"
			case "unowned":
				e.Owned = false
			case "coarse":
				e.Precise = false
			case "nan":
				e.PanelX = math.NaN()
			case "timestamp":
				e.Timestamp = time.Time{}
			case "idle":
				e.Timestamp = e.Timestamp.Add(time.Second)
			case "ABA":
				s := e.Scope
				s.Revision++
				r.SetScope(s, r.policy)
				r.SetScope(e.Scope, r.policy)
			}
			if r.Step(e) != nil {
				t.Fatal("refusal missing")
			}
			e.Sequence++
			e.MomentumPhase = "none"
			e.Owned = true
			e.Precise = true
			e.PanelX = 0
			e.Timestamp = time.Unix(3, 0)
			if r.Step(e) != nil {
				t.Fatal("retired ID revived")
			}
		})
	}
}

func TestGestureModelPinchSwipeToward(t *testing.T) {
	for _, kind := range []string{"magnify", "swipe"} {
		r, e := gestureFixture()
		e.Kind = kind
		if kind == "magnify" {
			e.Magnification = -.15
		} else {
			e.DeltaY = 1
		}
		got := r.Step(e)
		if got == nil || got.Action != "hide" {
			t.Fatal(kind, got)
		}
	}
}

func TestGestureModelTowardEdgesAndPolicyRetirement(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		r, e := gestureFixture()
		p := r.policy
		p.Edge = edge
		e.Admission = r.SetScope(e.Scope, p)
		switch edge {
		case "top":
			e.DeltaY = -80
		case "bottom":
			e.DeltaY = 80
		case "left":
			e.DeltaX = -80
		case "right":
			e.DeltaX = 80
		}
		if got := r.Step(e); got == nil || got.Action != "hide" {
			t.Fatal(edge, got)
		}
	}
	r, e := gestureFixture()
	admission := e.Admission
	if r.SetScope(e.Scope, r.policy) != admission {
		t.Fatal("same policy changed admission")
	}
	p := r.policy
	p.Enabled = false
	r.SetScope(e.Scope, p)
	e.DeltaX = 80
	if r.Step(e) != nil {
		t.Fatal("old policy accepted")
	}
	e.Admission = r.admission
	if r.Step(e) != nil {
		t.Fatal("disabled action")
	}
}
