package dock

import (
	"math"
	"testing"
	"time"

	"option-tab/internal/platform"
)

var shakeStart = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func shakeEvent(sequence uint64, x float64) platform.WindowDragEvent {
	kind := "moved"
	if sequence == 1 {
		kind = "candidate"
	}
	return platform.WindowDragEvent{Sequence: sequence, Generation: 1, GestureID: 1, Timestamp: shakeStart.Add(time.Duration(sequence-1) * 50 * time.Millisecond), Kind: kind, Window: platform.WindowRole{WindowID: 101, AppID: 10, Role: "AXWindow", Subrole: "AXStandardWindow"}, PointerX: 500 + x, PointerY: 50, WindowX: 100 + x, WindowY: 20}
}

func shakeTrace(r *ShakeRecognizer, mutate func(*platform.WindowDragEvent)) []*ShakeIntent {
	var intents []*ShakeIntent
	// Two correlated movement samples arm, then four 40pt reversals.
	for i, x := range []float64{0, 20, 40, 0, 40, 0, 40, 0, 40, 0, 40} {
		e := shakeEvent(uint64(i+1), x)
		if mutate != nil {
			mutate(&e)
		}
		if intent := r.Step(e); intent != nil {
			intents = append(intents, intent)
		}
	}
	return intents
}

func TestShakeExactWindowEvidenceFiresOnce(t *testing.T) {
	for _, action := range []string{"minimizeOthers", "closeOthers"} {
		t.Run(action, func(t *testing.T) {
			r := NewShakeRecognizer(action)
			r.SetGeneration(1)
			got := shakeTrace(r, nil)
			if len(got) != 1 {
				t.Fatalf("expected one action, got %+v", got)
			}
			i := got[0]
			if i.Kind != action || i.Generation != 1 || i.GestureID != 1 || i.WindowID != 101 || i.AppID != 10 || i.WindowX != 140 || i.WindowY != 20 || !i.Timestamp.Equal(shakeStart.Add(300*time.Millisecond)) {
				t.Fatalf("wrong exact intent %+v", i)
			}
		})
	}
}

func TestShakeRejectsUnprovenOrDisabledMovement(t *testing.T) {
	cases := map[string]func(*platform.WindowDragEvent){
		"pointer only":         func(e *platform.WindowDragEvent) { e.WindowX = 100 },
		"opposite AX movement": func(e *platform.WindowDragEvent) { e.WindowX = 600 - e.PointerX },
		"different speed":      func(e *platform.WindowDragEvent) { e.WindowX = 100 + (e.PointerX-500)*0.5 },
		"vertical":             func(e *platform.WindowDragEvent) { e.WindowY = e.WindowX * 2; e.PointerY = e.WindowY + 30 },
		"zero PID":             func(e *platform.WindowDragEvent) { e.Window.AppID = 0 },
		"zero window":          func(e *platform.WindowDragEvent) { e.Window.WindowID = 0 },
		"dialog":               func(e *platform.WindowDragEvent) { e.Window.Subrole = "AXDialog" },
		"not root":             func(e *platform.WindowDragEvent) { e.Window.Role = "AXButton" },
		"nonfinite":            func(e *platform.WindowDragEvent) { e.PointerX = math.NaN() },
		"infinite":             func(e *platform.WindowDragEvent) { e.WindowX = math.Inf(1) },
		"unknown generation":   func(e *platform.WindowDragEvent) { e.Generation = 2 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := NewShakeRecognizer("closeOthers")
			r.SetGeneration(1)
			if got := shakeTrace(r, mutate); len(got) != 0 {
				t.Fatalf("unsafe action %+v", got)
			}
		})
	}
	for _, action := range []string{"none", "", "invalid"} {
		r := NewShakeRecognizer(action)
		r.SetGeneration(1)
		if got := shakeTrace(r, nil); len(got) != 0 {
			t.Fatalf("disabled %q acted", action)
		}
	}
}

func TestShakeThresholdsAndBoundedTime(t *testing.T) {
	cases := []struct {
		name     string
		xs       []float64
		interval time.Duration
		want     int
	}{
		{"four reversals and path", []float64{0, 20, 40, 0, 40, 0, 40}, 50 * time.Millisecond, 1},
		{"three reversals", []float64{0, 20, 40, 0, 40, 0}, 50 * time.Millisecond, 0},
		{"small tremor", []float64{0, 10, 20, 0, 20, 0, 20, 0, 20, 0, 20}, 50 * time.Millisecond, 0},
		{"path below 180", []float64{0, 12, 24, 0, 24, 0, 24, 0, 24}, 50 * time.Millisecond, 0},
		{"idle above 250", []float64{0, 20, 40, 0, 40, 0, 40}, 251 * time.Millisecond, 0},
		{"outside 700 window", []float64{0, 20, 40, 0, 40, 0, 40}, 150 * time.Millisecond, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewShakeRecognizer("minimizeOthers")
			r.SetGeneration(1)
			count := 0
			for i, x := range tc.xs {
				e := shakeEvent(uint64(i+1), x)
				e.Timestamp = shakeStart.Add(time.Duration(i) * tc.interval)
				if r.Step(e) != nil {
					count++
				}
			}
			if count != tc.want {
				t.Fatalf("got %d actions want %d", count, tc.want)
			}
		})
	}
}

func TestShakeRetiresCancelledAndStaleGestures(t *testing.T) {
	for _, kind := range []string{"up", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			r := NewShakeRecognizer("closeOthers")
			r.SetGeneration(1)
			r.Step(shakeEvent(1, 0))
			r.Step(shakeEvent(2, 20))
			r.Step(shakeEvent(3, 40))
			e := shakeEvent(4, 40)
			e.Kind = kind
			r.Step(e)
			// Even a delayed candidate cannot revive this ID.
			got := shakeTrace(r, func(e *platform.WindowDragEvent) { e.Sequence += 10; e.Timestamp = e.Timestamp.Add(time.Second) })
			if len(got) != 0 {
				t.Fatal("retired gesture revived")
			}
			got = shakeTrace(r, func(e *platform.WindowDragEvent) {
				e.Sequence += 30
				e.GestureID = 2
				e.Timestamp = e.Timestamp.Add(2 * time.Second)
			})
			if len(got) != 1 {
				t.Fatal("fresh gesture failed")
			}
		})
	}
}

func TestShakeGenerationAndConfigurationInvalidatePendingEvidence(t *testing.T) {
	r := NewShakeRecognizer("closeOthers")
	r.SetGeneration(1)
	r.Step(shakeEvent(1, 0))
	r.Step(shakeEvent(2, 20))
	r.Step(shakeEvent(3, 40))
	r.SetGeneration(2)
	r.SetGeneration(1)
	if got := shakeTrace(r, nil); len(got) != 0 {
		t.Fatal("old generation acted")
	}
	got := shakeTrace(r, func(e *platform.WindowDragEvent) { e.Generation = 2 })
	if len(got) != 1 {
		t.Fatal("new generation failed")
	}
	r.Configure("none")
	r.Configure("minimizeOthers")
	if got := shakeTrace(r, func(e *platform.WindowDragEvent) { e.Generation = 2; e.Sequence += 20 }); len(got) != 0 {
		t.Fatal("configure revived old gesture")
	}
	r.Reset()
	if got := shakeTrace(r, func(e *platform.WindowDragEvent) {
		e.Generation = 2
		e.Sequence += 40
		e.GestureID = 2
		e.Timestamp = e.Timestamp.Add(3 * time.Second)
	}); len(got) != 1 {
		t.Fatal("fresh gesture failed after reset")
	}
}

func TestShakeRejectsIdentitySwitchAndOutOfOrderEvidence(t *testing.T) {
	for _, change := range []string{"window", "pid", "sequence", "time", "stationary", "gap"} {
		t.Run(change, func(t *testing.T) {
			r := NewShakeRecognizer("closeOthers")
			r.SetGeneration(1)
			var got []*ShakeIntent
			for i, x := range []float64{0, 20, 40, 0, 40, 0, 40} {
				e := shakeEvent(uint64(i+1), x)
				if i == 3 {
					switch change {
					case "window":
						e.Window.WindowID = 102
					case "pid":
						e.Window.AppID = 11
					case "sequence":
						e.Sequence = 2
					case "time":
						e.Timestamp = shakeStart
					case "stationary":
						e.WindowX = 140
					case "gap":
						e.Timestamp = e.Timestamp.Add(time.Second)
					}
				}
				if intent := r.Step(e); intent != nil {
					got = append(got, intent)
				}
			}
			if len(got) != 0 {
				t.Fatalf("unsafe %s trace acted %+v", change, got)
			}
		})
	}
}

func TestShakeArmingDoesNotCountItsOwnReversal(t *testing.T) {
	r := NewShakeRecognizer("closeOthers")
	r.SetGeneration(1)
	// The second movement establishes correlation, even if it reverses. Only
	// three subsequent reversals follow; total path alone cannot trigger.
	for i, x := range []float64{0, 40, 0, 40, 0, 40} {
		if got := r.Step(shakeEvent(uint64(i+1), x)); got != nil {
			t.Fatalf("arming counted as a reversal: %+v", got)
		}
	}
}

func TestShakeAccumulatesSmallSamplesIntoRealReversalLegs(t *testing.T) {
	r := NewShakeRecognizer("minimizeOthers")
	r.SetGeneration(1)
	count := 0
	xs := []float64{0, 20, 40, 30, 20, 10, 0, 10, 20, 30, 40, 30, 20, 10, 0, 10, 20, 30, 40}
	for i, x := range xs {
		e := shakeEvent(uint64(i+1), x)
		e.Timestamp = shakeStart.Add(time.Duration(i) * 30 * time.Millisecond)
		if got := r.Step(e); got != nil {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("accumulated legs produced %d actions", count)
	}
}
