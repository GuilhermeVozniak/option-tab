package main

import (
	"errors"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

type appModePlatform struct {
	*fake.Fake
	apps      []domain.App
	activated []domain.AppID
}

func (p *appModePlatform) Apps() ([]domain.App, error) {
	return append([]domain.App(nil), p.apps...), nil
}

func (p *appModePlatform) ActivateApp(id domain.AppID) error {
	p.activated = append(p.activated, id)
	return nil
}
func (p *appModePlatform) AppIcon(pid, maxPx int) string { return "icon" }

func TestAppModeRuntimeWiresWindowlessInventoryAndExactActivation(t *testing.T) {
	p := &appModePlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, Name: "Editor", BundleID: "editor.app"}, {ID: 30, Name: "Empty", BundleID: "empty.app"}}}
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, AppName: "Editor", BundleID: "editor.app", Title: "Document"}})
	a := newApp(p, config.Default(), "")
	defer a.stopCapture()
	var shown switcher.State
	a.eventSink = func(name string, data any) {
		if name == "switcher:show" {
			shown = data.(switcher.State)
		}
	}
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if shown.Mode != config.ModeApps || len(shown.Apps) != 2 || shown.Apps[1].Icon != "icon" {
		t.Fatalf("native app inventory/icons not wired: %+v", shown)
	}
	a.SelectApp(30)
	if err := a.ConfirmApp(30); err != nil {
		t.Fatal(err)
	}
	if len(p.activated) != 1 || p.activated[0] != 30 {
		t.Fatalf("activation targets=%v", p.activated)
	}
}

func TestAppModeCaptureUsesSelectedWindowIDInsteadOfAppIndex(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	defer a.stopCapture()
	a.Show(switcher.State{Mode: config.ModeApps, Style: config.StyleAppIcons, Appearance: config.Default().AppSwitcher.Appearance, Selected: 2, SelectedWindowID: 101, Entries: []switcher.Entry{{WindowID: 101, AppID: 10}, {WindowID: 102, AppID: 10}}})
	if got := a.captureSelected.Load(); got != 101 {
		t.Fatalf("capture selection=%d want window101", got)
	}
}

func TestRuntimeConfirmFailureRemainsVisible(t *testing.T) {
	p := fake.New()
	p.FocusErr = errors.New("refused")
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "Document"}})
	s := config.Default()
	s.Shortcuts[0].Mode = config.ModeWindows
	a := newApp(p, s, "")
	defer a.stopCapture()
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if err := a.Confirm(); err == nil || !a.controller.IsOpen() {
		t.Fatal("confirmation failure lost at runtime bridge")
	}
	var notice string
	a.eventSink = func(name string, data any) {
		if name == "switcher:error" {
			notice = data.(string)
		}
	}
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
	if notice == "" {
		t.Fatal("native key-path failure was not surfaced")
	}
}
