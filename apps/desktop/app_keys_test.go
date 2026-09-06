package main

import (
	"sync/atomic"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type sessionKeys struct {
	platform.HotkeyEngine
	keys    chan platform.KeyEvent
	session atomic.Uint64
}

func (k *sessionKeys) Keys() <-chan platform.KeyEvent { return k.keys }
func (k *sessionKeys) SetKeySession(session uint64)   { k.session.Store(session) }

type sessionKeysPlatform struct {
	*fake.Fake
	engine *sessionKeys
}

func (p *sessionKeysPlatform) Hotkeys() platform.HotkeyEngine { return p.engine }

func TestNativeKeyQueueCannotCrossPresentationSessions(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	k := &sessionKeys{HotkeyEngine: f.Hotkeys(), keys: make(chan platform.KeyEvent, 2)}
	p := &sessionKeysPlatform{Fake: f, engine: k}
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	a := newApp(p, s, "")
	defer a.stopCapture()
	forwarded := make(chan platform.KeyEvent, 2)
	a.eventSink = func(name string, data any) {
		if name == "switcher:key" {
			forwarded <- data.(platform.KeyEvent)
		}
	}
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	a.controller.HandleHotkey(activate)
	old := a.controller.State().Session
	a.controller.Cancel()
	a.controller.HandleHotkey(activate)
	current := a.controller.State().Session
	done := make(chan struct{})
	go func() { a.keyLoop(); close(done) }()
	k.keys <- platform.KeyEvent{Session: old, Key: "a"}
	k.keys <- platform.KeyEvent{Session: current, Key: "b"}
	if got := <-forwarded; got.Session != current || got.Key != "b" {
		t.Fatalf("forwarded retired key: %+v", got)
	}
	if k.session.Load() != current {
		t.Error("native capture did not receive current session")
	}
	a.stopCapture()
	<-done
	if k.session.Load() != 0 {
		t.Error("shutdown kept native key capture armed")
	}
}
