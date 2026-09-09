package launcher

import (
	"testing"
	"time"
)

func TestLetterCycleUnicodeAndRetirement(t *testing.T) {
	r := &LetterCycle{}
	scope := Scope{Epoch: 1, DisplayUUID: "display", Session: 1, Revision: 1}
	admission := r.SetScope(scope, true)
	items := []LetterItem{{ID: "a", Name: "Éditor", Visible: true, Actionable: true}, {ID: "b", Name: "école", Visible: true, Actionable: true}, {ID: "hidden", Name: "Écho", Actionable: true}}
	p := LetterPacket{Scope: scope, Admission: admission, Sequence: 1, Timestamp: time.Unix(1, 0), Key: "é"}
	if got := r.Step(p, items); got != "a" {
		t.Fatal(got)
	}
	p.Sequence++
	p.Timestamp = p.Timestamp.Add(10 * time.Millisecond)
	if got := r.Step(p, items); got != "b" {
		t.Fatal(got)
	}
	p.Sequence++
	p.Timestamp = p.Timestamp.Add(time.Second)
	if got := r.Step(p, items); got != "a" {
		t.Fatal(got)
	}
	p.Sequence++
	p.Modifiers = 1
	if got := r.Step(p, items); got != "" {
		t.Fatal(got)
	}
	next := scope
	next.Revision++
	r.SetScope(next, true)
	r.SetScope(scope, true)
	p.Sequence++
	p.Modifiers = 0
	if got := r.Step(p, items); got != "" {
		t.Fatal("scope ABA", got)
	}
}

func TestHapticLimiterTransitions(t *testing.T) {
	r := &HapticLimiter{}
	scope := Scope{Epoch: 1, DisplayUUID: "display", Session: 1, Revision: 1}
	now := time.Unix(1, 0)
	if r.Allow(scope, false, true, "a", now) || r.Allow(scope, true, false, "a", now) {
		t.Fatal("disabled")
	}
	if !r.Allow(scope, true, true, "a", now) {
		t.Fatal("first selection")
	}
	if r.Allow(scope, true, true, "a", now.Add(time.Second)) {
		t.Fatal("same target")
	}
	if r.Allow(scope, true, true, "b", now.Add(time.Millisecond)) {
		t.Fatal("rate")
	}
	if r.Allow(scope, true, true, "b", now.Add(time.Second)) {
		t.Fatal("delayed same transition")
	}
	if !r.Allow(scope, true, true, "c", now.Add(time.Second)) {
		t.Fatal("new transition")
	}
}
