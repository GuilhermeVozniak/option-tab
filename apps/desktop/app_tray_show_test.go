package main

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
)

func trayShowApp(t *testing.T, mutate func(*config.Settings)) *App {
	t.Helper()
	p := fake.New()
	p.SetWindows([]domain.Window{
		{ID: 101, AppID: 10, AppName: "Editor", BundleID: "editor.app", Title: "One", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 102, AppID: 20, AppName: "Notes", BundleID: "notes.app", Title: "Two", OnScreen: true, SpaceID: 1, ScreenID: 1},
	})
	s := config.Default()
	mutate(&s)
	a := newApp(p, s, "")
	t.Cleanup(a.stopCapture)
	return a
}

// The tray's "Show Option Tab" must not depend on shortcut 1 being enabled.
func TestTrayShowUsesTheFirstEnabledShortcut(t *testing.T) {
	a := trayShowApp(t, func(s *config.Settings) { s.Shortcuts[0].Enabled = false })
	a.showSwitcherFromTray()
	st := a.controller.State()
	if !st.Open || st.ShortcutID != 2 {
		t.Fatalf("tray show with shortcut 1 disabled: open=%v shortcut=%d", st.Open, st.ShortcutID)
	}
}

func TestTrayShowOpensWithoutAnyEnabledShortcut(t *testing.T) {
	a := trayShowApp(t, func(s *config.Settings) {
		for i := range s.Shortcuts {
			s.Shortcuts[i].Enabled = false
		}
	})
	a.showSwitcherFromTray()
	if st := a.controller.State(); !st.Open || st.Mode != config.ModeWindows {
		t.Fatalf("tray show with every shortcut disabled: open=%v mode=%s", st.Open, st.Mode)
	}
}

func TestTrayShowKeepsUsingShortcutOneWhenEnabled(t *testing.T) {
	a := trayShowApp(t, func(*config.Settings) {})
	a.showSwitcherFromTray()
	if st := a.controller.State(); !st.Open || st.ShortcutID != 1 {
		t.Fatalf("open=%v shortcut=%d", st.Open, st.ShortcutID)
	}
}
