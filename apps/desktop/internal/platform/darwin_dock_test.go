//go:build darwin

package platform

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/domain"
)

func dockRawFixture() dockRaw {
	return dockRaw{Generation: 3, Status: "ready", DockPID: 20, PointerX: -995, PointerY: 400, Screens: []dockScreen{{ID: 2, Bounds: domain.Bounds{X: -1000, Y: 0, W: 1000, H: 800}, Scale: 2}}, Item: &dockRawItem{PID: 20, Role: "AXDockItem", Subrole: "AXApplicationDockItem", URL: "file:///Applications/One.app", Path: "/Applications/One.app", BundleID: "fixture.one", Title: "Same name", Bounds: domain.Bounds{X: -1000, Y: 350, W: 60, H: 60}, Apps: []dockApp{{PID: 10, Path: "/Applications/One.app", BundleID: "fixture.one"}, {PID: 11, Path: "/Applications/Other.app", BundleID: "fixture.other"}}}}
}

func TestDockMappingUsesURLAndGlobalPoints(t *testing.T) {
	raw := dockRawFixture()
	o := mapDockObservation(raw, 1, 5)
	if o.Item == nil || o.Item.Kind != "app" || o.Item.AppID != 10 || o.Item.ScreenID != 2 || o.Item.Edge != "left" || o.Item.Bounds.X != -1000 || o.Item.Bounds.W != 60 {
		t.Fatalf("wrong identity/point geometry: %+v", o)
	}
}

func TestDockFolderMappingUsesExactURLInsteadOfTitle(t *testing.T) {
	r := dockRawFixture()
	r.Item.Subrole = "AXFolderDockItem"
	r.Item.URL = "file:///tmp/Actual%20Folder"
	r.Item.Path = "/tmp/Actual Folder"
	r.Item.Title = "/tmp/Invented From Title"
	r.Item.BundleID = ""
	r.Item.Apps = nil
	directory := true
	r.Item.Directory = &directory
	o := mapDockObservation(r, 1, 5)
	if o.Item == nil || o.Item.Kind != "folder" || o.Item.Path != "/tmp/Actual Folder" || o.Item.AppID != 0 || o.Item.BundleID != "" {
		t.Fatalf("wrong folder identity: %+v", o.Item)
	}
}

func TestDockFolderMappingRejectsUnsafeIdentity(t *testing.T) {
	for name, mutate := range map[string]func(*dockRawItem){
		"malformed URL":     func(i *dockRawItem) { i.URL = "file:///%zz" },
		"non-file URL":      func(i *dockRawItem) { i.URL = "https://example.com/folder" },
		"URL path mismatch": func(i *dockRawItem) { i.URL = "file:///tmp/Other" },
		"confirmed non-directory": func(i *dockRawItem) {
			no := false
			i.Directory = &no
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := dockRawFixture()
			r.Item.Subrole = "AXFolderDockItem"
			r.Item.URL = "file:///tmp/Folder"
			r.Item.Path = "/tmp/Folder"
			r.Item.BundleID = ""
			r.Item.Apps = nil
			mutate(r.Item)
			if got := mapDockObservation(r, 1, 5).Item; got != nil {
				t.Fatalf("unsafe folder accepted: %+v", got)
			}
		})
	}
}

func TestDockFolderMappingRetainsUnknownDirectoryForExplicitAccess(t *testing.T) {
	r := dockRawFixture()
	r.Item.Subrole = "AXFolderDockItem"
	r.Item.URL = "file:///Users/test/Protected"
	r.Item.Path = "/Users/test/Protected"
	r.Item.BundleID = ""
	r.Item.Apps = nil
	r.Item.Directory = nil
	if got := mapDockObservation(r, 1, 5).Item; got == nil || got.Kind != "folder" {
		t.Fatalf("inaccessible exact folder identity was lost: %+v", got)
	}
}

func TestDockMappingRejectsNonAppsAmbiguityAndStaleDockPID(t *testing.T) {
	for _, kind := range []string{"unsupported", "ambiguousPath", "ambiguousBundle", "stalePID"} {
		t.Run(kind, func(t *testing.T) {
			r := dockRawFixture()
			switch kind {
			case "unsupported":
				r.Item.Subrole = "AXDocumentDockItem"
			case "ambiguousPath":
				r.Item.Apps = append(r.Item.Apps, dockApp{PID: 12, Path: r.Item.Path, BundleID: r.Item.BundleID})
			case "ambiguousBundle":
				r.Item.Path = "/Applications/Absent.app"
				r.Item.URL = "file:///Applications/Absent.app"
				r.Item.Apps = append(r.Item.Apps, dockApp{PID: 12, Path: "/Volumes/Other/One.app", BundleID: r.Item.BundleID})
			case "stalePID":
				r.Item.PID = 99
			}
			if o := mapDockObservation(r, 1, 5); o.Item != nil {
				t.Fatalf("unsafe item accepted: %+v", o.Item)
			}
		})
	}
}

func TestDockPinnedAppAndScreenIntersection(t *testing.T) {
	r := dockRawFixture()
	r.Item.Apps = nil
	r.Item.Bounds = domain.Bounds{X: -20, Y: 770, W: 80, H: 60}
	r.Screens = append(r.Screens, dockScreen{ID: 3, Bounds: domain.Bounds{X: 0, Y: 0, W: 1200, H: 800}, Scale: 1})
	o := mapDockObservation(r, 1, 5)
	if o.Item == nil || o.Item.AppID != 0 || o.Item.ScreenID != 3 || o.Item.Edge != "bottom" {
		t.Fatalf("pinned/intersection: %+v", o.Item)
	}
}

type dockBlockedPoller struct {
	entered chan struct{}
	release chan struct{}
	closed  atomic.Bool
}

func (p *dockBlockedPoller) Poll() (dockRaw, error) {
	close(p.entered)
	<-p.release
	return dockRawFixture(), nil
}
func (p *dockBlockedPoller) Close() { p.closed.Store(true) }
func TestDockCancelDuringClassificationDropsLateObservation(t *testing.T) {
	p := &dockBlockedPoller{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	var emitted atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- runDockObservation(ctx, func(DockObservation) { emitted.Add(1) }, func() (dockPoller, error) { return p, nil }, time.Millisecond)
	}()
	<-p.entered
	cancel()
	close(p.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation stuck")
	}
	if emitted.Load() != 0 || !p.closed.Load() {
		t.Fatalf("late callback or retained worker: callbacks=%d closed=%v", emitted.Load(), p.closed.Load())
	}
}

func TestDockGenerationClearsUnavailableItem(t *testing.T) {
	r := dockRawFixture()
	r.Status = "dockUnavailable"
	r.Generation++
	o := mapDockObservation(r, 9, 4)
	if o.Item != nil || o.Generation == mapDockObservation(dockRawFixture(), 8, 4).Generation {
		t.Fatalf("stale dock item survived: %+v", o)
	}
}

type dockFastPoller struct {
	count   atomic.Uint64
	reached chan struct{}
	closed  atomic.Bool
}

func (p *dockFastPoller) Poll() (dockRaw, error) {
	n := p.count.Add(1)
	if n == 8 {
		close(p.reached)
	}
	return dockRawFixture(), nil
}
func (p *dockFastPoller) Close() { p.closed.Store(true) }
func TestDockObserverCoalescesBehindSlowConsumer(t *testing.T) {
	p := &dockFastPoller{reached: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	var sequences []uint64
	go func() {
		done <- runDockObservation(ctx, func(o DockObservation) {
			sequences = append(sequences, o.Sequence)
			if len(sequences) == 1 {
				close(first)
				<-release
			} else {
				cancel()
			}
		}, func() (dockPoller, error) { return p, nil }, time.Millisecond)
	}()
	<-first
	select {
	case <-p.reached:
	case <-time.After(time.Second):
		t.Fatal("slow consumer blocked native polling")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not settle")
	}
	// Poll eight may still be mapping when the consumer unblocks; at least
	// seven polls have completed, so a bounded latest-value queue yields >=7.
	if len(sequences) != 2 || sequences[1] < 7 || !p.closed.Load() {
		t.Fatalf("not coalesced/closed: %v %v", sequences, p.closed.Load())
	}
}

func TestDockEdgeUsesDisplayContainerAndPreferenceTie(t *testing.T) {
	screen := domain.Bounds{X: 0, Y: 0, W: 1000, H: 800}
	cases := []struct {
		name             string
		icon, container  domain.Bounds
		preference, want string
	}{
		{"right", domain.Bounds{X: 930, Y: 300, W: 60, H: 60}, domain.Bounds{}, "bottom", "right"},
		{"bottomContainer", domain.Bounds{X: 0, Y: 740, W: 60, H: 60}, domain.Bounds{X: 0, Y: 730, W: 1000, H: 70}, "left", "bottom"},
		{"cornerTie", domain.Bounds{X: 0, Y: 740, W: 60, H: 60}, domain.Bounds{}, "bottom", "bottom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dockItemEdge(c.icon, c.container, screen, c.preference); got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}
