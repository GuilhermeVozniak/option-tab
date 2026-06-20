package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestDefault_IsValid(t *testing.T) {
	s := Default()
	if err := s.Validate(); err != nil {
		t.Fatalf("Default() must be valid, got error: %v", err)
	}
	if len(s.Shortcuts) == 0 {
		t.Fatal("Default() must define at least one shortcut")
	}
	if s.Shortcuts[0].Chord == "" {
		t.Error("first default shortcut must have a chord")
	}
	if s.Version != CurrentVersion {
		t.Errorf("Default().Version = %d, want %d", s.Version, CurrentVersion)
	}
}

func TestNormalize_ClampsRanges(t *testing.T) {
	s := Default()
	s.Appearance.ThumbnailMaxPx = 5    // below floor
	s.Appearance.MaxColumns = 0        // below floor
	s.Appearance.BackgroundOpacity = 9 // above ceiling
	s.Appearance.FontSizePx = -3       // below floor
	out := s.Normalize()
	if out.Appearance.ThumbnailMaxPx < minThumbnailPx {
		t.Errorf("ThumbnailMaxPx not clamped: %d", out.Appearance.ThumbnailMaxPx)
	}
	if out.Appearance.MaxColumns < 1 {
		t.Errorf("MaxColumns not clamped: %d", out.Appearance.MaxColumns)
	}
	if out.Appearance.BackgroundOpacity > 1 {
		t.Errorf("BackgroundOpacity not clamped: %v", out.Appearance.BackgroundOpacity)
	}
	if out.Appearance.FontSizePx < 1 {
		t.Errorf("FontSizePx not clamped: %d", out.Appearance.FontSizePx)
	}
}

func TestNormalize_DedupesAndCapsShortcuts(t *testing.T) {
	s := Default()
	s.Shortcuts = []Shortcut{
		{ID: 1, Chord: "option+tab", Enabled: true},
		{ID: 1, Chord: "control+tab", Enabled: true}, // duplicate id
	}
	out := s.Normalize()
	seen := map[int]bool{}
	for _, sc := range out.Shortcuts {
		if seen[sc.ID] {
			t.Errorf("duplicate shortcut id %d after Normalize", sc.ID)
		}
		seen[sc.ID] = true
		if sc.ID < 1 || sc.ID > MaxShortcuts {
			t.Errorf("shortcut id %d out of range", sc.ID)
		}
	}
}

func TestNormalize_InvalidStyleFallsBack(t *testing.T) {
	s := Default()
	s.Appearance.Style = "bogus"
	out := s.Normalize()
	if !out.Appearance.Style.Valid() {
		t.Errorf("invalid style not normalized: %q", out.Appearance.Style)
	}
}

func TestLoad_MergesOntoDefaultsAndIgnoresUnknown(t *testing.T) {
	// Only overrides Order; everything else should come from defaults.
	in := `{"version":1,"order":"alphabetical","unknownField":42}`
	s, err := Load(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Order != OrderAlphabetical {
		t.Errorf("Order = %q, want alphabetical", s.Order)
	}
	if s.Appearance.Style == "" {
		t.Error("Appearance should be filled from defaults when omitted")
	}
}

func TestLoad_RejectsMalformedJSON(t *testing.T) {
	if _, err := Load(strings.NewReader("{not json")); err == nil {
		t.Error("Load() should error on malformed JSON")
	}
}

func TestSaveLoad_RoundTrips(t *testing.T) {
	want := Default()
	want.Order = OrderAlphabetical
	want.Behavior.HoldToCycle = false
	want.Appearance.Blur = false
	want.Appearance.AccentColor = "#ff8800"

	var buf bytes.Buffer
	if err := Save(&buf, want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Order != want.Order || got.Behavior.HoldToCycle != want.Behavior.HoldToCycle ||
		got.Appearance.AccentColor != want.Appearance.AccentColor || got.Appearance.Blur != want.Appearance.Blur {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, want)
	}
}

func TestLoad_MigratesOldVersion(t *testing.T) {
	// A version-0 document (no version field) should be migrated to CurrentVersion.
	s, err := Load(strings.NewReader(`{"order":"recent"}`))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Version != CurrentVersion {
		t.Errorf("migrated version = %d, want %d", s.Version, CurrentVersion)
	}
}

func TestShortcutScope_Defaults(t *testing.T) {
	s := Default()
	// At least one shortcut should scope to the active app (AltTab parity: 2nd shortcut).
	var hasActiveApp bool
	for _, sc := range s.Shortcuts {
		if sc.Scope.AppScope == AppScopeActiveApp {
			hasActiveApp = true
		}
	}
	if !hasActiveApp {
		t.Error("expected a default shortcut scoped to the active app")
	}
}
