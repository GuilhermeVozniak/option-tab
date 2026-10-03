package platform

import (
	"testing"

	"option-tab/internal/domain"
)

func TestDockLockExposedEdges(t *testing.T) {
	displays := []DockLockDisplay{{UUID: "left", Bounds: domain.Bounds{X: -100, Y: 0, W: 100, H: 100}}, {UUID: "main", Bounds: domain.Bounds{X: 0, Y: 0, W: 100, H: 100}}, {UUID: "lower", Bounds: domain.Bounds{X: 25, Y: 100, W: 50, H: 100}}}
	edges, ok := dockLockEdges(displays, "bottom")
	if !ok || len(edges["main"]) != 2 || len(edges["left"]) != 1 {
		t.Fatalf("partial/negative edges: %+v", edges)
	}
	edges, ok = dockLockEdges(displays, "left")
	if !ok || len(edges["main"]) != 0 {
		t.Fatal("covered left edge admitted")
	}
	edges, ok = dockLockEdges(displays, "right")
	if !ok || len(edges["left"]) != 0 {
		t.Fatal("covered right edge admitted")
	}
	displays[0].Mirrored = true
	if _, ok = dockLockEdges(displays, "bottom"); ok {
		t.Fatal("mirroring admitted")
	}
}

func TestDockLockContainerAuthority(t *testing.T) {
	d := []DockLockDisplay{{UUID: "one", Bounds: domain.Bounds{X: -100, Y: 0, W: 100, H: 100}}}
	for _, tt := range []struct {
		edge string
		b    domain.Bounds
	}{{"bottom", domain.Bounds{X: -80, Y: 90, W: 60, H: 20}}, {"left", domain.Bounds{X: -110, Y: 20, W: 20, H: 60}}, {"right", domain.Bounds{X: -10, Y: 20, W: 20, H: 60}}} {
		if dockLockActual(d, tt.edge, tt.b) != "one" {
			t.Fatal(tt.edge)
		}
	}
	if dockLockActual(d, "bottom", domain.Bounds{X: -80, Y: 40, W: 60, H: 20}) != "" {
		t.Fatal("interior container invented edge")
	}
}

func TestDockLockSemanticStatusCoalescing(t *testing.T) {
	a := DockMonitorLockState{Session: 1, Revision: 2, Sequence: 3, ObservedAtMs: 4, Status: "protected", Generation: 5}
	b := a
	b.Sequence++
	b.ObservedAtMs++
	if dockLockStateChanged(a, b) {
		t.Fatal("poll metadata caused status event")
	}
	for _, change := range []func(*DockMonitorLockState){func(s *DockMonitorLockState) { s.Generation++ }, func(s *DockMonitorLockState) { s.Revision++ }, func(s *DockMonitorLockState) { s.ActualUUID = "other" }, func(s *DockMonitorLockState) { s.Status = "bypassed" }} {
		b = a
		change(&b)
		if !dockLockStateChanged(a, b) {
			t.Fatal("semantic change swallowed")
		}
	}
}

func TestDockLockMissingPreferenceUsesPositiveGeometry(t *testing.T) {
	d := []DockLockDisplay{{UUID: "one", Bounds: domain.Bounds{W: 3440, H: 1440}}}
	if got := dockLockInferredEdge(d, domain.Bounds{X: 903, Y: 1440, W: 1634, H: 72}); got != "bottom" {
		t.Fatal(got)
	}
	if got := dockLockInferredEdge(d, domain.Bounds{X: 900, Y: 900, W: 1600, H: 72}); got != "" {
		t.Fatal("unknown interior geometry inferred")
	}
}

func TestDockLockPlacementApproachesThenPushesOutward(t *testing.T) {
	for _, s := range []dockLockSegment{{X: 1, Y: 97, W: 98, H: 4, DY: -6}, {X: -1, Y: 1, W: 4, H: 98, DX: 6}, {X: 97, Y: 1, W: 4, H: 98, DX: -6}} {
		x, y, dx, dy := dockLockPlacementPoint(s, 0)
		ex, ey, ox, oy := dockLockPlacementPoint(s, 1)
		if (x-ex)*s.DX+(y-ey)*s.DY <= 0 || dx != 0 || dy != 0 {
			t.Fatal("first step does not approach inward")
		}
		if ox*s.DX+oy*s.DY >= 0 {
			t.Fatal("edge movement lacks outward pressure")
		}
	}
}
