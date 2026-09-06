package main

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

func TestPerformActionTargetsClickedWindowAndRefreshes(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	a := newApp(f, s, "")
	defer a.stopCapture()
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	st := a.controller.State()
	target := st.Entries[0]
	if st.Selected == 0 {
		target = st.Entries[1]
	}
	r, err := a.PerformAction("close", uint64(target.WindowID), int(target.AppID))
	if err != nil || r.Succeeded != 1 || len(f.CloseCalls) != 1 || f.CloseCalls[0] != target.WindowID {
		t.Fatalf("result=%+v err=%v calls=%v", r, err, f.CloseCalls)
	}
	for _, e := range a.controller.State().Entries {
		if e.WindowID == target.WindowID {
			t.Fatal("closed entry remained after refresh")
		}
	}
}

func TestPerformActionReturnsIdentityError(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	a := newApp(f, config.Default(), "")
	_, err := a.PerformAction("close", 1, 2)
	if err == nil || len(f.CloseCalls) != 0 {
		t.Fatal("mismatched target accepted")
	}
}

func TestShutdownRetiresSwitcherAndRejectsActions(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	a := newApp(f, s, "")
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	a.controller.HandleHotkey(activate)
	a.stopCapture()
	a.controller.HandleHotkey(activate)
	if a.controller.IsOpen() {
		t.Error("shutdown controller admitted a presentation")
	}
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
	if len(f.FocusCalls) != 0 {
		t.Error("shutdown controller focused a window")
	}
	if _, err := a.PerformAction("close", 1, 1); err == nil {
		t.Error("shutdown accepted a window action")
	}
	if len(f.CloseCalls) != 0 {
		t.Error("shutdown action reached platform")
	}
}
