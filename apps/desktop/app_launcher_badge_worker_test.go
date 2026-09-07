package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLauncherBadgeWorkerJoinsRetiredReadsBeforeSuccessor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan uint64, 4)
	release := make(chan struct{})
	var active, peak atomic.Int32
	w := newLauncherBadgeWorker(func(ctx context.Context, generation uint64) {
		n := active.Add(1)
		defer active.Add(-1)
		if n > peak.Load() {
			peak.Store(n)
		}
		entered <- generation
		if generation == 1 {
			<-release
		} else {
			<-ctx.Done()
		}
	})
	go w.run(ctx)
	first := w.update()
	if got := <-entered; got != first {
		t.Fatal(got)
	}
	w.update()
	latest := w.update()
	if w.current(first) {
		t.Fatal("retired read still owns publication")
	}
	select {
	case <-entered:
		t.Fatal("successor overlapped blocked native read")
	default:
	}
	close(release)
	select {
	case got := <-entered:
		if got != latest {
			t.Fatal("did not coalesce superseded plans", got, latest)
		}
	case <-time.After(time.Second):
		t.Fatal("successor did not start after receipt")
	}
	cancel()
	select {
	case <-w.done:
	case <-time.After(time.Second):
		t.Fatal("worker did not drain")
	}
	if peak.Load() != 1 || active.Load() != 0 || w.current(latest) {
		t.Fatal("unfinished source or live publication after shutdown")
	}
}

func TestLauncherBadgeWorkerShutdownWaitsForCallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	w := newLauncherBadgeWorker(func(context.Context, uint64) { close(entered); <-release })
	go w.run(ctx)
	generation := w.update()
	<-entered
	cancel()
	select {
	case <-w.done:
		t.Fatal("receipt before blocked callback returned")
	default:
	}
	close(release)
	select {
	case <-w.done:
	case <-time.After(time.Second):
		t.Fatal("missing joined receipt")
	}
	if w.current(generation) {
		t.Fatal("shutdown retained authority")
	}
}
