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

type appSessionFixture struct {
	*fake.Fake
	events    chan platform.SessionState
	started   chan struct{}
	delivered chan struct{}
	stopped   chan struct{}
}

func (p *appSessionFixture) ObserveSession(ctx context.Context, emit func(platform.SessionState)) error {
	close(p.started)
	defer close(p.stopped)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case state := <-p.events:
			emit(state)
			p.delivered <- struct{}{}
		}
	}
}

func sessionWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("session fixture timed out")
	}
}

func TestAppSessionSourceRunsWithDockDisabledAndStops(t *testing.T) {
	p := &appSessionFixture{fake.New(), make(chan platform.SessionState), make(chan struct{}), make(chan struct{}, 4), make(chan struct{})}
	settings := config.Default()
	settings.Dock.Enabled = false
	a := newApp(p, settings, "")
	defer a.stopCapture()
	a.startSessionObservation()
	a.startSessionObservation()
	sessionWait(t, p.started)
	p.events <- platform.SessionState{Generation: 10, Inactive: true, Reason: "sessionAway"}
	sessionWait(t, p.delivered)
	a.viewMu.Lock()
	inactive := a.sessionInactive
	a.viewMu.Unlock()
	if !inactive {
		t.Fatal("Dock-disabled app ignored session suspension")
	}
	p.events <- platform.SessionState{Generation: 11}
	sessionWait(t, p.delivered)
	a.viewMu.Lock()
	inactive = a.sessionInactive
	a.viewMu.Unlock()
	if inactive {
		t.Fatal("session did not resume")
	}
	a.stopCapture()
	sessionWait(t, p.stopped)
	a.acceptSessionState(platform.SessionState{Generation: 12, Inactive: true})
	a.viewMu.Lock()
	inactive = a.sessionInactive
	a.viewMu.Unlock()
	if inactive {
		t.Fatal("late callback mutated terminal App")
	}
}

func TestAppSessionCancelsCaptureAndRejectsStaleActivity(t *testing.T) {
	p := &lifecycleStreamPlatform{Fake: fake.New(), started: make(chan domain.WindowID, 8), stopped: make(chan domain.WindowID, 8)}
	a := newApp(p, config.Default(), "")
	defer a.stopCapture()
	a.acceptSessionState(platform.SessionState{Generation: 1})
	st := switcher.State{Style: config.StyleThumbnails, Appearance: config.Default().Appearance, Entries: []switcher.Entry{{WindowID: 10}}, Selected: 0}
	a.Show(st)
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("capture never started")
	}
	a.acceptSessionState(platform.SessionState{Generation: 2, Inactive: true})
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("session inactivity did not cancel capture")
	}
	if a.captureActive || a.switcherVisible {
		t.Fatal("session inactivity left keyboard view active")
	}
	a.acceptSessionState(platform.SessionState{Generation: 3})
	a.acceptSessionState(platform.SessionState{Generation: 2, Inactive: true})
	if a.sessionInactive {
		t.Fatal("old native generation overrode resume")
	}
	a.Show(st)
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("capture did not resume")
	}
	// A coalesced active state still invalidates work from the preceding epoch.
	a.acceptSessionState(platform.SessionState{Generation: 5})
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("active generation retained preceding capture")
	}
	if a.captureActive || a.sessionInactive {
		t.Fatal("coalesced active transition was not reset then resumed")
	}
}

func TestAppSessionInactiveRejectsLateShowAndUpdate(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	defer a.stopCapture()
	a.acceptSessionState(platform.SessionState{Generation: 1, Inactive: true})
	events := 0
	a.eventSink = func(string, any) { events++ }
	st := switcher.State{Appearance: config.Default().Appearance}
	a.Show(st)
	a.Update(st)
	if events != 0 || a.captureActive || a.switcherVisible {
		t.Fatal("stale View callback admitted during inactivity")
	}
}

func TestAppSessionResumeRejectsPriorPresentation(t *testing.T) {
	settings := config.Default()
	for i := range settings.Shortcuts {
		settings.Shortcuts[i].Mode = config.ModeWindows
	}
	p := fake.New()
	p.SetWindows([]domain.Window{{ID: 10, AppID: 20, Title: "Window", OnScreen: true, SpaceID: 1, ScreenID: 1}})
	a := newApp(p, settings, "")
	defer a.stopCapture()
	a.acceptSessionState(platform.SessionState{Generation: 1})
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	a.controller.HandleHotkey(activate)
	old := a.controller.State()
	if old.Session == 0 || !old.Open {
		t.Fatal("fixture presentation did not activate")
	}
	a.acceptSessionState(platform.SessionState{Generation: 2, Inactive: true})
	a.acceptSessionState(platform.SessionState{Generation: 3})
	events := 0
	a.eventSink = func(string, any) { events++ }
	a.Show(old)
	a.Update(old)
	if events != 0 || a.captureActive || a.switcherVisible {
		t.Fatal("pre-suspension presentation reopened after resume")
	}
	a.controller.HandleHotkey(activate)
	fresh := a.controller.State()
	if !fresh.Open || fresh.Session == old.Session || !a.switcherVisible {
		t.Fatal("new presentation did not resume")
	}
	// User pause survives another native session round trip.
	a.controller.SetPaused(true)
	a.acceptSessionState(platform.SessionState{Generation: 4, Inactive: true})
	a.acceptSessionState(platform.SessionState{Generation: 5})
	a.controller.HandleHotkey(activate)
	if a.controller.IsOpen() || !a.controller.Paused() {
		t.Fatal("native resume overrode user pause")
	}
}

func TestAppSessionCoalescedActiveInvalidatesDockCapture(t *testing.T) {
	p := &lifecycleStreamPlatform{Fake: fake.New(), started: make(chan domain.WindowID, 8), stopped: make(chan domain.WindowID, 8)}
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	a.acceptSessionState(platform.SessionState{Generation: 1})
	a.showDock(dockFixtureState(7, 10, 20), true)
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("Dock capture did not start")
	}
	a.acceptSessionState(platform.SessionState{Generation: 3})
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("coalesced active retained Dock capture")
	}
	if a.dockState.Session != 0 || a.captureDockSession.Load() != 0 || a.sessionInactive {
		t.Fatal("Dock generation was not invalidated before resume")
	}
}
