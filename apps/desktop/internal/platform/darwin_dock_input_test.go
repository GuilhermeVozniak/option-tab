//go:build darwin

package platform

import (
	"context"
	"testing"
	"time"

	"option-tab/internal/domain"
)

func TestNativeDockInputOwnershipAndReplay(t *testing.T) {
	got := nativeDockInputOwnershipProbe()
	for name, ok := range got {
		if !ok {
			t.Errorf("native callback contract failed: %s", name)
		}
	}
}

func TestNativeDockInputFreshnessPreservesObservationAge(t *testing.T) {
	now := time.Now()
	item := &DockItem{AppID: 20, Path: "/Fixture.app", BundleID: "fixture.app", ScreenID: 1, Bounds: domain.Bounds{X: 1, Y: 1, W: 50, H: 50}}
	for _, age := range []time.Duration{151 * time.Millisecond, -time.Millisecond} {
		got := nativeInputTarget(DockInputTarget{Generation: 1, DockPID: 10, ObservedAt: now.Add(-age), Item: item}, now)
		if got.app_pid != 0 {
			t.Fatalf("stale/future target admitted age=%s", age)
		}
	}
	got := nativeInputTarget(DockInputTarget{Generation: 1, DockPID: 10, ObservedAt: now.Add(-100 * time.Millisecond), Item: item}, now)
	if got.app_pid != 20 {
		t.Fatal("fresh identity dropped")
	}
}

func TestNativeDockInputIsolatedLifecycleJoinsAndDisabledDoesNothing(t *testing.T) {
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		targets := make(chan DockInputTarget, 1)
		done := make(chan error, 1)
		go func() {
			done <- runNativeDockInput(ctx, DockInputPolicy{ClickToHide: true}, targets, func(DockInputEvent) { t.Error("empty cache emitted input") }, true)
		}()
		targets <- DockInputTarget{}
		deadline := time.Now().Add(time.Second)
		for nativeDockInputTestAlive() == 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if nativeDockInputTestAlive() != 1 {
			cancel()
			t.Fatal("isolated source did not start")
		}

		if i == 1 {
			close(targets)
		} else {
			cancel()
		}
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("isolated tap failed to join")
		}
		cancel()
		if nativeDockInputTestAlive() != 0 {
			t.Fatal("native owner leaked after return")
		}
	}
	if err := runNativeDockInput(context.Background(), DockInputPolicy{}, make(chan DockInputTarget), func(DockInputEvent) { t.Fatal("disabled emission") }, true); err != nil {
		t.Fatal(err)
	}
}

func TestDockInputObservationTimestampPredatesBlockingPoll(t *testing.T) {
	poller := &dockBlockedPoller{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan DockObservation, 1)
	done := make(chan error, 1)
	go func() {
		done <- runDockObservation(ctx, func(o DockObservation) { observed <- o; cancel() }, func() (dockPoller, error) { return poller, nil }, time.Hour)
	}()
	<-poller.entered
	entered := time.Now()
	time.Sleep(20 * time.Millisecond)
	close(poller.release)
	select {
	case o := <-observed:
		if o.ObservedAt.IsZero() || o.ObservedAt.After(entered) || o.DockPID != 20 {
			t.Fatalf("queue re-stamped observation: %+v", o)
		}
	case <-time.After(time.Second):
		t.Fatal("no observation")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
