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
	a := newApp(f, config.Default(), "")
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
