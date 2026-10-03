package platform

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLauncherObserverCancellationJoinsRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var emissions atomic.Int32
	go func() {
		done <- observeLauncherEnvironment(ctx, func(LauncherEnvironment) { emissions.Add(1) }, launcherEnvironmentOps{
			Snapshot: func() LauncherEnvironment { close(entered); <-release; return LauncherEnvironment{Complete: true} },
			Pointer:  func() (float64, float64, bool) { return 1, 2, true },
		}, time.Millisecond)
	}()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("returned before native read joined")
	default:
	}
	close(release)
	if err := <-done; err != context.Canceled {
		t.Fatalf("cancel: %v", err)
	}
	if emissions.Load() != 0 {
		t.Fatal("late native result published")
	}
}

func TestLauncherObserverCopiesInventory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	shared := []LauncherDisplay{{UUID: "first", SpaceID: 1, SpaceKind: "ordinary", SpaceStatus: "known"}}
	count := 0
	done := make(chan error, 1)
	go func() {
		done <- observeLauncherEnvironment(ctx, func(e LauncherEnvironment) {
			count++
			if e.Generation == 0 || e.Sequence == 0 || e.ObservedAt.IsZero() {
				t.Error("unstamped environment")
			}
			e.Displays[0].UUID = "consumer mutation"
			if count == 2 {
				cancel()
			}
		}, launcherEnvironmentOps{Snapshot: func() LauncherEnvironment { return LauncherEnvironment{Complete: true, Displays: shared} }, Pointer: func() (float64, float64, bool) { return 0, 0, true }}, time.Millisecond)
	}()
	<-done
	if shared[0].UUID != "first" {
		t.Fatal("callback mutated native inventory")
	}
}
