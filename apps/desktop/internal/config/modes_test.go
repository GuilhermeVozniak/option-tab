package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestLegacyModeSettingsPreserveBindingsWithoutAliasing(t *testing.T) {
	s, err := Load(strings.NewReader(`{
		"version":2,
		"appearance":{"theme":"dark","accentColor":"#123456","backgroundOpacity":0.2,"previewSelected":false},
		"behavior":{"holdToCycle":false,"vimKeys":true,"actionBindings":{"KeyX":"close"}},
		"shortcuts":[{"id":1,"chord":"command+shift+tab","enabled":true,"scope":{"appScope":"all"}}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 3 || s.Shortcuts[0].Mode != ModeWindows || s.Shortcuts[0].Chord != "command+shift+tab" || s.Dock.Enabled {
		t.Fatalf("legacy behavior changed: %+v", s)
	}
	if s.AppSwitcher.Behavior.HoldToCycle || !s.AppSwitcher.Behavior.VimKeys || s.AppSwitcher.Behavior.ActionBindings["KeyX"] != ActionClose {
		t.Fatalf("legacy behavior not copied: %+v", s.AppSwitcher.Behavior)
	}
	if s.AppSwitcher.Appearance.Style != StyleAppIcons || !s.AppSwitcher.Appearance.PreviewSelected || s.AppSwitcher.Appearance.Theme != ThemeDark {
		t.Fatalf("app appearance not derived: %+v", s.AppSwitcher.Appearance)
	}
	if s.Dock.Appearance.Theme != ThemeDark || s.Dock.Appearance.AccentColor != "#123456" {
		t.Fatalf("dock appearance did not copy theme/accent: %+v", s.Dock.Appearance)
	}
	if s.Dock.Appearance.BackgroundOpacity != 0.85 {
		t.Fatalf("dock copied unrelated legacy appearance: %+v", s.Dock.Appearance)
	}
	s.AppSwitcher.Behavior.ActionBindings["KeyX"] = ActionMinimize
	if s.Behavior.ActionBindings["KeyX"] != ActionClose {
		t.Fatal("mode maps alias")
	}
}

func TestDefault_NewInstallUsesAppsThenWindows(t *testing.T) {
	s := Default()
	if s.Version != 3 || s.Shortcuts[0].Mode != ModeApps || s.Shortcuts[1].Mode != ModeWindows {
		t.Fatalf("new shortcut modes: %+v", s.Shortcuts)
	}
	if s.AppSwitcher.Appearance.Style != StyleAppIcons || !s.AppSwitcher.Appearance.PreviewSelected {
		t.Fatalf("app defaults: %+v", s.AppSwitcher.Appearance)
	}
	if s.Dock.Enabled || s.Dock.HoverDelayMs != 300 || s.Dock.DismissDelayMs != 250 || s.Dock.HoverSlopPx != 8 || s.Dock.BridgePaddingPx != 12 {
		t.Fatalf("dock defaults: %+v", s.Dock)
	}
	if s.Dock.Appearance.ThumbnailMaxPx != 240 || s.Dock.Appearance.MaxRows != 2 || s.Dock.Appearance.MaxColumns != 5 || s.Dock.Appearance.FadeOutAnimation {
		t.Fatalf("dock appearance defaults: %+v", s.Dock.Appearance)
	}
}

func TestPreferencesReturnsIndependentSnapshots(t *testing.T) {
	s := Default()
	w := s.Preferences(ModeWindows)
	a := s.Preferences(ModeApps)
	w.Behavior.ActionBindings["KeyW"] = ActionQuit
	a.Behavior.ActionBindings["KeyW"] = ActionHide
	if s.Behavior.ActionBindings["KeyW"] != ActionClose || s.AppSwitcher.Behavior.ActionBindings["KeyW"] != ActionClose {
		t.Fatal("Preferences returned aliased maps")
	}
	if w.Appearance.Style != s.Appearance.Style || a.Appearance.Style != StyleAppIcons {
		t.Fatalf("wrong mode preferences: window=%+v app=%+v", w, a)
	}
}

func TestLegacyExplicitValuesSurviveModeMigration(t *testing.T) {
	s, err := Load(strings.NewReader(`{"version":2,"appearance":{"apparitionDelayMs":0},"behavior":{"holdToCycle":false,"arrowKeys":false,"actionBindings":{}},"order":"alphabetical","placement":"activeScreen"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.AppSwitcher.Behavior.HoldToCycle || s.AppSwitcher.Behavior.ArrowKeys || s.AppSwitcher.Appearance.ApparitionDelayMs != 0 || len(s.AppSwitcher.Behavior.ActionBindings) != 0 || s.AppSwitcher.Behavior.ActionBindings == nil {
		t.Fatalf("explicit legacy values lost: %+v", s.AppSwitcher)
	}
	if s.AppSwitcher.Order != OrderAlphabetical || s.AppSwitcher.Placement != PlaceActiveScreen {
		t.Fatalf("legacy mode preferences lost: %+v", s.AppSwitcher)
	}
}

func TestDefaultModeBindingsDoNotAlias(t *testing.T) {
	s := Default()
	s.AppSwitcher.Behavior.ActionBindings["KeyW"] = ActionQuit
	if s.Behavior.ActionBindings["KeyW"] != ActionClose {
		t.Fatal("default app/window maps alias")
	}
}

func TestVersion3ExplicitEmptyAppBindingsSurviveLoad(t *testing.T) {
	s, err := Load(strings.NewReader(`{"version":3,"appSwitcher":{"behavior":{"actionBindings":{}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.AppSwitcher.Behavior.ActionBindings == nil || len(s.AppSwitcher.Behavior.ActionBindings) != 0 {
		t.Fatalf("explicit empty app bindings lost: %#v", s.AppSwitcher.Behavior.ActionBindings)
	}
}

func TestSaveRejectsInvalidModeSettings(t *testing.T) {
	s := Default()
	s.Shortcuts[0].Mode = "invalid"
	if err := Save(&bytes.Buffer{}, s); err == nil {
		t.Fatal("Save accepted invalid shortcut mode")
	}
}

func TestLegacyExplicitValidShortcutModeSurvivesMigration(t *testing.T) {
	s, err := Load(strings.NewReader(`{"version":2,"shortcuts":[{"id":1,"chord":"command+tab","enabled":true,"scope":{"appScope":"all"},"mode":"apps"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Shortcuts[0].Mode != ModeApps {
		t.Fatalf("explicit legacy mode overwritten: %+v", s.Shortcuts[0])
	}
}

func TestVersion3AbsentModeObjectsDeriveFromLoadedWindowSettings(t *testing.T) {
	s, err := Load(strings.NewReader(`{"version":3,"appearance":{"theme":"dark","accentColor":"#654321"},"behavior":{"holdToCycle":false,"vimKeys":true,"actionBindings":{"KeyX":"hide"}},"order":"space","placement":"focusedWindowScreen"}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.AppSwitcher.Appearance.Theme != ThemeDark || s.AppSwitcher.Behavior.HoldToCycle || !s.AppSwitcher.Behavior.VimKeys || s.AppSwitcher.Behavior.ActionBindings["KeyX"] != ActionHide || s.AppSwitcher.Order != OrderSpace || s.AppSwitcher.Placement != PlaceFocusedWindowScreen {
		t.Fatalf("absent appSwitcher did not derive from loaded window settings: %+v", s.AppSwitcher)
	}
	if s.Dock.Appearance.Theme != ThemeDark || s.Dock.Appearance.AccentColor != "#654321" {
		t.Fatalf("absent dock did not derive theme/accent: %+v", s.Dock)
	}
}

func TestVersion3ExplicitModeObjectsPreserveZeroFalseAndEmpty(t *testing.T) {
	s, err := Load(strings.NewReader(`{"version":3,"behavior":{"holdToCycle":true},"appSwitcher":{"appearance":{"apparitionDelayMs":0},"behavior":{"holdToCycle":false,"actionBindings":{}},"order":"alphabetical","placement":"activeScreen"},"dock":{"enabled":false,"hoverDelayMs":0,"dismissDelayMs":0}}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.AppSwitcher.Behavior.HoldToCycle || s.AppSwitcher.Behavior.ActionBindings == nil || len(s.AppSwitcher.Behavior.ActionBindings) != 0 || s.AppSwitcher.Appearance.ApparitionDelayMs != 0 {
		t.Fatalf("explicit app object lost values: %+v", s.AppSwitcher)
	}
	if s.Dock.Enabled || s.Dock.HoverDelayMs != 0 || s.Dock.DismissDelayMs != 0 {
		t.Fatalf("explicit dock object lost values: %+v", s.Dock)
	}
}
