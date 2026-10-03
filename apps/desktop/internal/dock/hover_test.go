package dock

import (
	"testing"
	"time"

	"option-tab/internal/domain"
)

func TestShownIconMovementRepositionsWithoutNewHoverDelay(t *testing.T) {
	h := NewHover(0, 250*time.Millisecond)
	at := time.Unix(1, 0)
	item := Item{Kind: "app", AppID: 10, Bounds: domain.Bounds{X: 100, Y: 700, W: 50, H: 50}, Edge: "bottom"}
	if h.Step(at, &item, false).Show == nil {
		t.Fatal("not shown")
	}
	item.Bounds.X = 180
	change := h.Step(at.Add(time.Millisecond), &item, false)
	if change.Move == nil || change.Show != nil || change.Move.Bounds.X != 180 {
		t.Fatal("must reposition existing panel")
	}
	if h.Step(at.Add(2*time.Millisecond), &item, false).Move != nil {
		t.Fatal("unchanged geometry should not keep moving panel")
	}
}

func TestHoverDelayAndIconToPanelBridge(t *testing.T) {
	h := NewHover(300*time.Millisecond, 200*time.Millisecond)
	item := Item{AppID: 10, Path: "/Applications/Editor.app"}
	at := time.Unix(100, 0)
	if got := h.Step(at, &item, false); got.Show != nil {
		t.Fatal("opened before delay")
	}
	if got := h.Step(at.Add(299*time.Millisecond), &item, false); got.Show != nil {
		t.Fatal("opened early")
	}
	if got := h.Step(at.Add(300*time.Millisecond), &item, false); got.Show == nil || got.Show.AppID != 10 {
		t.Fatal("did not open hovered app")
	}
	h.Step(at.Add(310*time.Millisecond), nil, false)
	if got := h.Step(at.Add(450*time.Millisecond), nil, true); got.Hide {
		t.Fatal("closed while entering panel")
	}
	if got := h.Step(at.Add(time.Second), nil, true); got.Hide {
		t.Fatal("closed inside panel")
	}
	h.Step(at.Add(1100*time.Millisecond), nil, false)
	if got := h.Step(at.Add(1300*time.Millisecond), nil, false); !got.Hide {
		t.Fatal("did not close after leave delay")
	}
}

func TestHoverSwitchAndCancelPending(t *testing.T) {
	h := NewHover(100*time.Millisecond, 50*time.Millisecond)
	at := time.Unix(100, 0)
	a, b := Item{AppID: 1}, Item{AppID: 2}
	h.Step(at, &a, false)
	h.Step(at.Add(100*time.Millisecond), &a, false)
	h.Step(at.Add(110*time.Millisecond), &b, false)
	h.Step(at.Add(150*time.Millisecond), &a, false)
	if got := h.Step(at.Add(250*time.Millisecond), &a, false); got.Show != nil {
		t.Fatal("stale hover replaced app")
	}
	if !h.Reset().Hide {
		t.Fatal("reset must close panel")
	}
}

func TestPanelPlacementAtEveryDockEdge(t *testing.T) {
	screen := domain.Bounds{X: -1000, Y: 0, W: 1000, H: 800}
	for _, tc := range []struct {
		edge         string
		anchor, want domain.Bounds
	}{
		{"bottom", domain.Bounds{X: -550, Y: 750, W: 50, H: 50}, domain.Bounds{X: -675, Y: 540, W: 300, H: 200}},
		{"left", domain.Bounds{X: -1000, Y: 375, W: 50, H: 50}, domain.Bounds{X: -940, Y: 300, W: 300, H: 200}},
		{"right", domain.Bounds{X: -50, Y: 375, W: 50, H: 50}, domain.Bounds{X: -360, Y: 300, W: 300, H: 200}},
	} {
		if got := PlacePanel(tc.anchor, screen, tc.edge, 300, 200, 10); got != tc.want {
			t.Errorf("%s got %+v want %+v", tc.edge, got, tc.want)
		}
	}
	got := PlacePanel(domain.Bounds{X: -999, Y: 750, W: 50, H: 50}, screen, "bottom", 2000, 900, 10)
	if got.X < screen.X || got.Y < screen.Y || got.X+got.W > 0 || got.Y+got.H > 800 {
		t.Fatalf("panel outside monitor: %+v", got)
	}
}
