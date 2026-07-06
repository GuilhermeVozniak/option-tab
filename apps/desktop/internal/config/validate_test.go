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
	if WindowVisibility("x").Valid() || SizePreset("x").Valid() || TruncationMode("x").Valid() ||
		ReleaseAction("x").Valid() || MenubarIconStyle("x").Valid() || UpdatePolicy("x").Valid() ||
		CrashPolicy("x").Valid() || BlacklistHide("x").Valid() {
		t.Error("unknown enum values should be invalid")
	}
	if !VisShowAtEnd.Valid() || !SizeLarge.Valid() || !TruncateMiddle.Valid() ||
		!ReleaseDoNothing.Valid() || !MenubarIconOutline.Valid() || !UpdatesAuto.Valid() ||
		!CrashAlways.Valid() || !HideWhenNoWindow.Valid() || !ThemeLight.Valid() {
		t.Error("known enum values should be valid")
	}
}

func TestNormalize_DropsInvalidShortcutsKeepsValid(t *testing.T) {
	// Out-of-range ids and empty chords are dropped individually; a valid
	// sibling in the same list survives untouched.
	s := Default()
	s.Shortcuts = []Shortcut{
		{ID: 99, Chord: "option+tab"}, // id out of 1..MaxShortcuts
		{ID: 1, Chord: ""},            // empty chord
		{ID: 2, Chord: "option+tab"},  // valid
	}
	out := s.Normalize()
	if len(out.Shortcuts) != 1 {
		t.Fatalf("len(Shortcuts) = %d, want 1: %+v", len(out.Shortcuts), out.Shortcuts)
	}
	if out.Shortcuts[0].ID != 2 || out.Shortcuts[0].Chord != "option+tab" {
		t.Errorf("survivor = %+v, want ID=2 chord=option+tab", out.Shortcuts[0])
	}
}

func TestNormalize_ClearsInvalidShortcutOrderAndWhenReleased(t *testing.T) {
	// Invalid per-shortcut order/release overrides are cleared so the shortcut
	// inherits the global order and the default focus-on-release behavior.
	s := Default()
	s.Shortcuts = []Shortcut{{
		ID:           1,
		Chord:        "option+tab",
		Scope:        ShortcutScope{AppScope: AppScopeAll, Order: "bogus"},
		WhenReleased: "bogus",
	}}
	out := s.Normalize()
	if out.Shortcuts[0].Scope.Order != "" {
		t.Errorf("Scope.Order = %q, want empty (inherit global)", out.Shortcuts[0].Scope.Order)
	}
	if out.Shortcuts[0].WhenReleased != "" {
		t.Errorf("WhenReleased = %q, want empty (default focus)", out.Shortcuts[0].WhenReleased)
	}
}

func TestNormalize_ClampsRemainingRanges(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*Settings)
		got  func(Settings) int
		want int
	}{
		{
			"TitleMaxWidthPx low", func(s *Settings) { s.Appearance.TitleMaxWidthPx = 5 },
			func(s Settings) int { return s.Appearance.TitleMaxWidthPx }, 60,
		},
		{
			"TitleMaxWidthPx high", func(s *Settings) { s.Appearance.TitleMaxWidthPx = 5000 },
			func(s Settings) int { return s.Appearance.TitleMaxWidthPx }, 1000,
		},
		{
			"CornerRadiusPx low", func(s *Settings) { s.Appearance.CornerRadiusPx = -1 },
			func(s Settings) int { return s.Appearance.CornerRadiusPx }, 0,
		},
		{
			"CornerRadiusPx high", func(s *Settings) { s.Appearance.CornerRadiusPx = 999 },
			func(s Settings) int { return s.Appearance.CornerRadiusPx }, 64,
		},
		{
			"IconSizePx low", func(s *Settings) { s.Appearance.IconSizePx = 1 },
			func(s Settings) int { return s.Appearance.IconSizePx }, minIconPx,
		},
		{
			"ApparitionDelayMs low", func(s *Settings) { s.Appearance.ApparitionDelayMs = -5 },
			func(s Settings) int { return s.Appearance.ApparitionDelayMs }, 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Default()
			tt.mut(&s)
			if got := tt.got(s.Normalize()); got != tt.want {
				t.Errorf("clamped value = %d, want %d", got, tt.want)
			}
		})
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
