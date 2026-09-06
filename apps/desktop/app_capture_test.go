package main

import (
	"context"
	"encoding/json"
	"testing"
	"testing/synctest"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

func TestSwitcherPacketsKeepTheirPresentationIdentity(t *testing.T) {
	type packet struct {
		Session, Revision uint64
		Frames            map[string]string
	}
	p := fake.New()
	p.SetWindows([]domain.Window{{ID: 10, AppID: 20, Title: "Fixture"}})
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	a := newApp(p, s, "")
	defer a.stopCapture()
	events := map[string][]packet{}
	a.eventSink = func(name string, data any) {
		encoded, err := json.Marshal(data)
		if err != nil {
			t.Fatal(err)
		}
		var value packet
		if err := json.Unmarshal(encoded, &value); err != nil {
			t.Fatal(err)
		}
		events[name] = append(events[name], value)
	}
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	a.controller.HandleHotkey(activate)
	a.emitCaptureFrame(10, "frame")
	a.controller.Cancel()
	a.controller.HandleHotkey(activate)
	shows, hides, frames := events["switcher:show"], events["switcher:hide"], events["switcher:thumbnails"]
	if len(shows) != 2 || len(hides) != 1 || len(frames) != 1 {
		t.Fatalf("events=%+v", events)
	}
	if shows[0].Session == 0 || shows[0].Revision == 0 || shows[1].Session <= shows[0].Session || hides[0].Session != shows[0].Session || shows[0].Revision >= hides[0].Revision || hides[0].Revision >= shows[1].Revision {
		t.Fatalf("reordered delivery cannot distinguish presentations: %+v", events)
	}
	if frames[0].Session != shows[0].Session || frames[0].Frames["10"] != "frame" {
		t.Fatalf("unscoped frames: %+v", frames)
	}
}

type unavailableStreamPlatform struct{ *fake.Fake }

func (p *unavailableStreamPlatform) StreamWindow(context.Context, domain.WindowID, int, func(string)) error {
	return platform.WindowUnavailableError{}
}

type delayedBackgroundPlatform struct {
	*fake.Fake
	entered, release chan struct{}
}

func (p *delayedBackgroundPlatform) ThumbnailDataURL(domain.WindowID, int) string {
	close(p.entered)
	<-p.release
	return "old-background-frame"
}

func TestBackgroundCaptureDiscardsBatchAfterVisibleSessionChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := &delayedBackgroundPlatform{Fake: fake.New(), entered: make(chan struct{}), release: make(chan struct{})}
		p.SetWindows([]domain.Window{{ID: 10, AppID: 20, Title: "Fixture"}})
		s := config.Default()
		s.Behavior.CaptureInBackground = true
		a := newApp(p, s, "")
		defer a.stopCapture()
		go a.backgroundCaptureLoop()
		time.Sleep(4 * time.Second)
		synctest.Wait()
		<-p.entered
		a.Show(switcher.State{Style: config.StyleTitles, Appearance: s.Appearance})
		a.Hide()
		close(p.release)
		synctest.Wait()
		a.thumbCacheMu.Lock()
		defer a.thumbCacheMu.Unlock()
		if len(a.thumbCache) != 0 {
			t.Fatalf("old background batch published after view changed: %v", a.thumbCache)
		}
	})
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
