package config

import "testing"

func TestValidate_Errors(t *testing.T) {
	base := Default()
	mk := func(mut func(*Settings)) Settings {
		s := Default()
		mut(&s)
		return s
	}
	tests := []struct {
		name string
		s    Settings
	}{
		{"no shortcuts", mk(func(s *Settings) { s.Shortcuts = nil })},
		{"id out of range", mk(func(s *Settings) { s.Shortcuts = []Shortcut{{ID: 99, Chord: "x"}} })},
		{"duplicate id", mk(func(s *Settings) {
			s.Shortcuts = []Shortcut{{ID: 1, Chord: "a"}, {ID: 1, Chord: "b"}}
		})},
		{"empty chord", mk(func(s *Settings) { s.Shortcuts = []Shortcut{{ID: 1, Chord: ""}} })},
		{"bad style", mk(func(s *Settings) { s.Appearance.Style = "nope" })},
		{"bad order", mk(func(s *Settings) { s.Order = "nope" })},
		{"bad placement", mk(func(s *Settings) { s.Placement = "nope" })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.s.Validate(); err == nil {
				t.Errorf("Validate() = nil, want error")
			}
		})
	}
	if err := base.Validate(); err != nil {
		t.Errorf("Default must validate: %v", err)
	}
}

func TestEnums_ValidRejectsUnknown(t *testing.T) {
	if VisualStyle("x").Valid() || Theme("x").Valid() || OrderMode("x").Valid() ||
		Placement("x").Valid() || SpaceScope("x").Valid() || ScreenScope("x").Valid() ||
		AppScopeMode("x").Valid() {
		t.Error("unknown enum values should be invalid")
	}
	if !StyleAppIcons.Valid() || !ThemeDark.Valid() || !OrderSpace.Valid() ||
		!PlaceActiveScreen.Valid() || !SpacesActive.Valid() || !ScreensCursor.Valid() ||
		!AppScopeActiveApp.Valid() {
		t.Error("known enum values should be valid")
	}
}

func TestNormalize_FallsBackInvalidScopesAndStyleOverride(t *testing.T) {
	s := Default()
	s.Filters.Spaces = "bogus"
	s.Filters.Screens = "bogus"
	s.Appearance.Theme = "bogus"
	s.Order = "bogus"
	s.Placement = "bogus"
	s.Shortcuts = []Shortcut{{ID: 1, Chord: "option+tab", Scope: ShortcutScope{AppScope: "weird"}, StyleOverride: "weird"}}
	out := s.Normalize()
	if !out.Filters.Spaces.Valid() || !out.Filters.Screens.Valid() || !out.Appearance.Theme.Valid() ||
		!out.Order.Valid() || !out.Placement.Valid() {
		t.Error("invalid enums not normalized")
	}
	if out.Shortcuts[0].Scope.AppScope != AppScopeAll {
		t.Errorf("invalid AppScope not normalized: %q", out.Shortcuts[0].Scope.AppScope)
	}
	if out.Shortcuts[0].StyleOverride != "" {
		t.Errorf("invalid StyleOverride not cleared: %q", out.Shortcuts[0].StyleOverride)
	}
}

func TestNormalize_EmptyShortcutsRestoresDefaults(t *testing.T) {
	s := Default()
	s.Shortcuts = []Shortcut{{ID: 0, Chord: ""}} // all dropped
	out := s.Normalize()
	if len(out.Shortcuts) == 0 {
		t.Error("Normalize should restore default shortcuts when all are dropped")
	}
}

func TestNormalize_ClampHighEnds(t *testing.T) {
	s := Default()
	s.Appearance.ThumbnailMaxPx = 99999
	s.Appearance.IconSizePx = 99999
	s.Appearance.FontSizePx = 99999
	s.Appearance.MaxRows = 99999
	s.Appearance.BackgroundOpacity = -5
	out := s.Normalize()
	if out.Appearance.ThumbnailMaxPx != maxThumbnailPx || out.Appearance.IconSizePx != maxIconPx ||
		out.Appearance.FontSizePx != maxFontPx || out.Appearance.MaxRows != 20 ||
		out.Appearance.BackgroundOpacity != 0 {
		t.Errorf("high/low clamps wrong: %+v", out.Appearance)
	}
}
