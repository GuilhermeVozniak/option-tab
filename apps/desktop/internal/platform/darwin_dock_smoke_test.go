//go:build darwin

package platform

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestDockObserverDisposableIconSmoke(t *testing.T) {
	rawPID := os.Getenv("OPTION_TAB_DOCK_FIXTURE_PID")
	if rawPID == "" {
		t.Skip("requires disposable org.optiontab.DockObserverFixture icon")
	}
	pid, err := strconv.Atoi(rawPID)
	if err != nil || pid <= 0 {
		t.Fatal("invalid fixture PID")
	}
	path := os.Getenv("OPTION_TAB_DOCK_FIXTURE_PATH")
	if !filepath.IsAbs(path) {
		t.Fatal("missing exact fixture bundle path")
	}
	var lastGeneration uint64
	for cycle := 0; cycle < 3; cycle++ {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var callbacks atomic.Uint64
		matched := false
		lastSequence := uint64(0)
		lastStatus := ""
		err := (&darwinPlatform{}).ObserveDock(ctx, func(o DockObservation) {
			callbacks.Add(1)
			lastStatus = o.Status
			if o.Sequence <= lastSequence {
				t.Errorf("sequence not increasing: %d then %d", lastSequence, o.Sequence)
			}
			lastSequence = o.Sequence
			if o.Item == nil || o.Item.Path != path {
				return
			}
			if int(o.Item.AppID) != pid || o.Item.BundleID != "org.optiontab.DockObserverFixture" {
				t.Errorf("wrong live fixture identity: %+v", o.Item)
				cancel()
				return
			}
			if o.Status != "ready" || o.Item.ScreenID == 0 || !validDockBounds(o.Item.Bounds) {
				t.Errorf("invalid native geometry: %+v", o)
				cancel()
				return
			}
			if o.Item.Edge != "left" && o.Item.Edge != "bottom" && o.Item.Edge != "right" {
				t.Errorf("invalid edge %q", o.Item.Edge)
			}
			if cycle > 0 && o.Generation == lastGeneration {
				t.Error("replacement observer reused cancelled generation")
			}
			t.Logf("cycle=%d sequence=%d generation=%d appPID=%d bundle=%s pointer=(%.1f,%.1f) bounds=%+v screen=%d edge=%s", cycle, o.Sequence, o.Generation, o.Item.AppID, o.Item.BundleID, o.PointerX, o.PointerY, o.Item.Bounds, o.Item.ScreenID, o.Item.Edge)
			lastGeneration = o.Generation
			matched = true
			cancel()
		})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if !matched {
			t.Fatalf("cycle %d never observed the hovered disposable icon; callbacks=%d status=%s", cycle, callbacks.Load(), lastStatus)
		}
		settled := callbacks.Load()
		time.Sleep(120 * time.Millisecond)
		if callbacks.Load() != settled {
			t.Fatal("callback after ObserveDock returned")
		}
	}
}
