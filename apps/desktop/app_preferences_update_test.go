package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

type capturingHotkeyEngine struct {
	platform.HotkeyEngine
	open bool
}

func (e *capturingHotkeyEngine) SetOpen(open bool) { e.open = open }

type capturingHotkeyPlatform struct {
	*fake.Fake
	engine *capturingHotkeyEngine
}

func (p *capturingHotkeyPlatform) Hotkeys() platform.HotkeyEngine { return p.engine }

func TestPreferencesRetireSwitcherKeyboardOwner(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	p := &capturingHotkeyPlatform{Fake: f, engine: &capturingHotkeyEngine{HotkeyEngine: f.Hotkeys()}}
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	s.Behavior.HoldToCycle = false
	a := newApp(p, s, "")
	t.Cleanup(a.stopCapture)
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if !p.engine.open {
		t.Fatal("fixture failed to activate native capture")
	}
	a.OpenPreferences()
	if p.engine.open || a.controller.IsOpen() || a.switcherVisible {
		t.Fatalf("preferences opened but switcher still owns input: tap=%v controller=%v visible=%v", p.engine.open, a.controller.IsOpen(), a.switcherVisible)
	}
}

type updateTestTransport func(*http.Request) (*http.Response, error)

func TestSearchPresentationRejectsDelayedSnapshot(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	a := newApp(f, config.Default(), "")
	t.Cleanup(a.stopCapture)
	var shown switcher.State
	a.eventSink = func(name string, payload any) {
		if name == "switcher:update" {
			shown = payload.(switcher.State)
		}
	}
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	session := a.controller.State().Session
	a.SetSearchForSession("a", session, 1)
	delayed := a.controller.State()
	a.SetSearchForSession("ab", session, 2)
	a.Update(delayed)
	if shown.Search != "ab" || shown.SearchSequence != 2 {
		t.Fatalf("delayed snapshot replaced newer visible search: %+v", shown)
	}
}

func (f updateTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDisablingAutoUpdatesRetiresPendingCheck(t *testing.T) {
	s := config.Default()
	s.Behavior.UpdatePolicy = config.UpdatesAuto
	a := newApp(fake.New(), s, "")
	t.Cleanup(a.stopCapture)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	old := http.DefaultTransport
	http.DefaultTransport = updateTestTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v99.0.0","html_url":"https://example.invalid/release","assets":[]}`)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	var stages []string
	a.eventSink = func(name string, data any) {
		if name == "update:progress" {
			stages = append(stages, data.(map[string]string)["stage"])
		}
	}
	go func() { defer close(done); a.checkForUpdate(true) }()
	<-entered
	s.Behavior.UpdatePolicy = config.UpdatesOff
	document, _ := json.Marshal(s)
	err := a.SaveSettings(string(document))
	close(release)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if len(stages) != 0 {
		t.Fatalf("disabled auto-update still started install stages: %v", stages)
	}
}
