package main

import (
	"context"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

type unavailableStreamPlatform struct{ *fake.Fake }

func (p *unavailableStreamPlatform) StreamWindow(context.Context, domain.WindowID, int, func(string)) error {
	return platform.WindowUnavailableError{}
}

func TestClosedUnselectedWindowClearsBothImagePaths(t *testing.T) {
	p := &unavailableStreamPlatform{fake.New()}
	a := newApp(p, config.Default(), "")
	defer a.stopCapture()
	events := make(chan string, 4)
	a.eventSink = func(name string, data any) {
		if frame, ok := data.(map[string]string); ok && frame["10"] == "" {
			events <- name
		}
	}
	// Window 10 previously supplied a large preview but is no longer selected.
	// Clearing only the thumbnail would revive its stale enlarged image later.
	a.captureSelected.Store(20)
	a.capturePreviewEnabled.Store(true)
	a.captureThumbnailsEnabled.Store(true)
	a.captures.Update([]domain.WindowID{10}, 10, 256)
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case name := <-events:
			seen[name] = true
		case <-time.After(time.Second):
			t.Fatalf("closed target must invalidate thumbnail and preview; received %v", seen)
		}
	}
	if !seen["switcher:thumbnails"] || !seen["switcher:preview"] {
		t.Fatalf("unexpected invalidations: %v", seen)
	}
}

type lifecycleStreamPlatform struct {
	*fake.Fake
	started chan domain.WindowID
	stopped chan domain.WindowID
}

func (p *lifecycleStreamPlatform) StreamWindow(ctx context.Context, id domain.WindowID, px int, frame func(string)) error {
	p.started <- id
	<-ctx.Done()
	p.stopped <- id
	return ctx.Err()
}

func TestCaptureDelayedUpdateAfterHideDoesNotRestart(t *testing.T) {
	p := &lifecycleStreamPlatform{Fake: fake.New(), started: make(chan domain.WindowID, 8), stopped: make(chan domain.WindowID, 8)}
	a := newApp(p, config.Default(), "")
	defer a.stopCapture()
	st := switcher.State{Style: config.StyleThumbnails, Appearance: config.Default().Appearance, Entries: []switcher.Entry{{WindowID: 10}}, Selected: 0}
	a.Show(st)
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("capture did not start")
	}
	// Represents the controller's already-created state delivered after Cancel.
	deliver := make(chan struct{})
	done := make(chan struct{})
	go func() { <-deliver; a.Update(st); close(done) }()
	a.Hide()
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("capture did not stop")
	}
	close(deliver)
	<-done
	select {
	case id := <-p.started:
		t.Fatalf("hidden update restarted window %d", id)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCaptureShutdownRejectsDelayedUpdateAndShow(t *testing.T) {
	p := &lifecycleStreamPlatform{Fake: fake.New(), started: make(chan domain.WindowID, 8), stopped: make(chan domain.WindowID, 8)}
	a := newApp(p, config.Default(), "")
	st := switcher.State{Style: config.StyleThumbnails, Appearance: config.Default().Appearance, Entries: []switcher.Entry{{WindowID: 10}}, Selected: 0}
	a.Show(st)
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("capture did not start")
	}
	a.stopCapture()
	a.stopCapture()
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("capture did not stop")
	}
	a.Update(st)
	a.Show(st)
	select {
	case id := <-p.started:
		t.Fatalf("shutdown restarted window %d", id)
	case <-time.After(50 * time.Millisecond):
	}
}
