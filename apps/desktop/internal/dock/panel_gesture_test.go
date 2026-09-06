package dock

import (
	"math"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func panelInput() config.DockInputSettings {
	return config.DockInputSettings{
		SwipeTowardDock: config.PointerClose, SwipeAwayFromDock: config.PointerMinimize,
		SwipePrevious: config.PointerHide, SwipeNext: config.PointerQuit,
	}
}

func TestPanelGestureNeverRevivesRetiredIDs(t *testing.T) {
	for _, reason := range []string{"fired", "ended", "cancelled", "momentum", "unowned", "target", "idle", "reset"} {
		t.Run(reason, func(t *testing.T) {
			r := NewPanelGestureRecognizer("bottom", panelInput())
			r.SetPresentation(4, 2)
			start := wheelEvent(1, 7, 40, 0)
			if reason == "fired" {
				start.DeltaX = 90
			}
			_ = r.Step(start)
			terminal := wheelEvent(2, 7, 0, 0)
			switch reason {
			case "ended", "cancelled":
				terminal.Phase = reason
			case "momentum":
				terminal.MomentumPhase = "changed"
			case "unowned":
				terminal.Owned = false
			case "target":
				terminal.WindowID = 99
			case "idle":
				terminal.Timestamp = start.Timestamp.Add(251 * time.Millisecond)
			case "reset":
				r.Reset()
			}
			_ = r.Step(terminal)
			late := wheelEvent(3, 7, 100, 0)
			late.Timestamp = terminal.Timestamp.Add(time.Millisecond)
			if got := r.Step(late); got != nil {
				t.Fatalf("retired gesture fired again: %+v", got)
			}
			fresh := wheelEvent(4, 8, 100, 0)
			fresh.Timestamp = late.Timestamp.Add(time.Millisecond)
			if got := r.Step(fresh); got == nil || got.GestureID != 8 {
				t.Fatalf("fresh gesture did not rearm: %+v", got)
			}
		})
	}
}

func TestPanelGestureRejectsMalformedInputAndRetiresItsID(t *testing.T) {
	mutations := map[string]func(*platform.DockPanelWheelEvent){
		"nan delta":           func(e *platform.DockPanelWheelEvent) { e.DeltaY = math.NaN() },
		"infinite delta":      func(e *platform.DockPanelWheelEvent) { e.DeltaX = math.Inf(1) },
		"missing window":      func(e *platform.DockPanelWheelEvent) { e.WindowID = 0 },
		"missing app":         func(e *platform.DockPanelWheelEvent) { e.AppID = 0 },
		"missing timestamp":   func(e *platform.DockPanelWheelEvent) { e.Timestamp = time.Time{} },
		"backwards timestamp": func(e *platform.DockPanelWheelEvent) { e.Timestamp = time.UnixMilli(1) },
		"unknown phase":       func(e *platform.DockPanelWheelEvent) { e.Phase = "unknown" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			r := NewPanelGestureRecognizer("bottom", panelInput())
			r.SetPresentation(4, 2)
			_ = r.Step(wheelEvent(1, 7, 10, 0))
			bad := wheelEvent(2, 7, 100, 0)
			mutate(&bad)
			if got := r.Step(bad); got != nil {
				t.Fatalf("malformed input emitted %+v", got)
			}
			if got := r.Step(wheelEvent(3, 7, 100, 0)); got != nil {
				t.Fatalf("malformed gesture revived: %+v", got)
			}
		})
	}
}

func wheelEvent(sequence, gesture uint64, dx, dy float64) platform.DockPanelWheelEvent {
	return platform.DockPanelWheelEvent{
		Session: 4, Revision: 2, Sequence: sequence, GestureID: gesture,
		Timestamp: time.UnixMilli(int64(sequence * 10)), WindowID: domain.WindowID(11),
		AppID: domain.AppID(12), DeltaX: dx, DeltaY: dy, Owned: true, Precise: true,
		Phase: "changed", MomentumPhase: "none",
	}
}

func TestPanelGestureMapsEveryEdgeRelativeDirection(t *testing.T) {
	tests := []struct {
		edge   string
		dx, dy float64
		want   config.PointerAction
	}{
		{"bottom", 0, -90, config.PointerClose},
		{"bottom", 0, 90, config.PointerMinimize},
		{"bottom", -90, 0, config.PointerHide},
		{"bottom", 90, 0, config.PointerQuit},
		{"left", -90, 0, config.PointerClose},
		{"left", 90, 0, config.PointerMinimize},
		{"left", 0, 90, config.PointerHide},
		{"left", 0, -90, config.PointerQuit},
		{"right", 90, 0, config.PointerClose},
		{"right", -90, 0, config.PointerMinimize},
		{"right", 0, 90, config.PointerHide},
		{"right", 0, -90, config.PointerQuit},
	}
	for _, tt := range tests {
		t.Run(tt.edge+string(tt.want), func(t *testing.T) {
			r := NewPanelGestureRecognizer(tt.edge, panelInput())
			r.SetPresentation(4, 2)
			got := r.Step(wheelEvent(1, 7, tt.dx, tt.dy))
			if got == nil || got.Action != tt.want || got.WindowID != 11 || got.AppID != 12 || got.GestureID != 7 {
				t.Fatalf("intent=%+v want action %q", got, tt.want)
			}
			if again := r.Step(wheelEvent(2, 7, tt.dx, tt.dy)); again != nil {
				t.Fatalf("gesture fired twice: %+v", again)
			}
		})
	}
}

func TestPanelGestureAccumulatesLocksReversesAndRearmsAfterIdle(t *testing.T) {
	r := NewPanelGestureRecognizer("bottom", panelInput())
	r.SetPresentation(4, 2)
	for sequence := uint64(1); sequence <= 7; sequence++ {
		if got := r.Step(wheelEvent(sequence, 9, -10, -2)); got != nil {
			t.Fatalf("fired before 80 locked horizontal points: %+v", got)
		}
	}
	if got := r.Step(wheelEvent(8, 9, -10, -2)); got == nil || got.Action != config.PointerHide {
		t.Fatalf("locked horizontal intent=%+v", got)
	}

	r.Reset()
	r.SetPresentation(4, 2)
	_ = r.Step(wheelEvent(10, 10, 50, 0))
	_ = r.Step(wheelEvent(11, 10, -50, 0))
	if got := r.Step(wheelEvent(12, 10, -40, 0)); got == nil || got.Action != config.PointerHide {
		t.Fatalf("reversal did not reset signed accumulation: %+v", got)
	}

	r.Reset()
	r.SetPresentation(4, 2)
	first := wheelEvent(20, 11, 40, 0)
	first.Timestamp = time.Unix(1, 0)
	_ = r.Step(first)
	late := wheelEvent(21, 12, 80, 0)
	late.Timestamp = first.Timestamp.Add(251 * time.Millisecond)
	if got := r.Step(late); got == nil || got.GestureID != 12 {
		t.Fatalf("idle did not rearm new gesture: %+v", got)
	}
}

func TestPanelGestureRejectsStaleUnownedMomentumAndChangedTargets(t *testing.T) {
	r := NewPanelGestureRecognizer("bottom", panelInput())
	r.SetPresentation(4, 2)
	bad := []platform.DockPanelWheelEvent{
		{Session: 3, Revision: 2, Sequence: 1, Owned: true, Precise: true},
		{Session: 4, Revision: 1, Sequence: 2, Owned: true, Precise: true},
		{Session: 4, Revision: 2, Sequence: 3, Owned: false, Precise: true},
		{Session: 4, Revision: 2, Sequence: 4, Owned: true, Precise: false},
	}
	for _, event := range bad {
		if got := r.Step(event); got != nil {
			t.Fatalf("invalid event emitted %+v", got)
		}
	}
	momentum := wheelEvent(5, 20, 0, -100)
	momentum.MomentumPhase = "changed"
	if got := r.Step(momentum); got != nil {
		t.Fatalf("momentum emitted %+v", got)
	}
	ended := wheelEvent(6, 21, 0, -100)
	ended.Phase = "ended"
	if got := r.Step(ended); got != nil {
		t.Fatalf("ended phase emitted %+v", got)
	}
	start := wheelEvent(7, 22, 0, -50)
	_ = r.Step(start)
	changed := wheelEvent(8, 22, 0, -50)
	changed.WindowID = 99
	if got := r.Step(changed); got != nil {
		t.Fatalf("changed target emitted %+v", got)
	}
	if got := r.Step(start); got != nil {
		t.Fatalf("stale sequence emitted %+v", got)
	}
}
