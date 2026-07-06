package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDefault_ShortcutChords(t *testing.T) {
	// Regression: the shipped defaults are Command+Tab (all windows) and
	// Option+Tab (active app only), matching AltTab and the app's UI.
	s := Default()
	if len(s.Shortcuts) < 2 {
		t.Fatalf("expected >=2 default shortcuts, got %d", len(s.Shortcuts))
	}
	if s.Shortcuts[0].Chord != "command+tab" || s.Shortcuts[0].Scope.AppScope != AppScopeAll {
		t.Errorf("shortcut 1 = %q/%q, want command+tab/all",
			s.Shortcuts[0].Chord, s.Shortcuts[0].Scope.AppScope)
	}
	if s.Shortcuts[1].Chord != "option+tab" || s.Shortcuts[1].Scope.AppScope != AppScopeActiveApp {
		t.Errorf("shortcut 2 = %q/%q, want option+tab/activeApp",
			s.Shortcuts[1].Chord, s.Shortcuts[1].Scope.AppScope)
	}
}

func TestDefault_NewBehaviorAndAppearanceFields(t *testing.T) {
	// Regression: defaults for the fields added this session.
	s := Default()
	if !s.Appearance.PreviewFade {
		t.Error("PreviewFade should default true")
	}
	if !s.Behavior.ArrowKeys {
		t.Error("ArrowKeys should default true")
	}
	if !s.Behavior.HapticFeedback {
		t.Error("HapticFeedback should default true")
	}
	if s.Behavior.CaptureInBackground {
		t.Error("CaptureInBackground should default false")
	}
}

func TestSaveLoad_RoundTripsNewFields(t *testing.T) {
	// Regression: the session's new fields survive a save/load round-trip
	// (non-default values must not be lost).
	want := Default()
	want.Appearance.PreviewFade = false
	want.Behavior.ArrowKeys = false
	want.Behavior.HapticFeedback = false
	want.Behavior.CaptureInBackground = true

	var buf bytes.Buffer
	if err := Save(&buf, want); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if got.Appearance.PreviewFade || got.Behavior.ArrowKeys ||
		got.Behavior.HapticFeedback || !got.Behavior.CaptureInBackground {
		t.Errorf("round-trip lost new fields: previewFade=%v arrowKeys=%v haptic=%v capture=%v",
			got.Appearance.PreviewFade, got.Behavior.ArrowKeys,
			got.Behavior.HapticFeedback, got.Behavior.CaptureInBackground)
	}
}

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

func TestSettingsJSON_AppBlacklistNeverNull(t *testing.T) {
	// The preferences UI maps over appBlacklist; a nil slice marshals as JSON
	// null and crashes the whole panel.
	for name, s := range map[string]Settings{
		"default":    Default(),
		"normalized": (Settings{}).Normalize(),
	} {
		b, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		if !strings.Contains(string(b), `"appBlacklist":[]`) {
			t.Errorf("%s: appBlacklist must marshal as [], got: %s", name, b)
		}
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

func TestLoad_LegacyBoolFiltersAndStringBlacklist(t *testing.T) {
	// v1 documents stored bools for the window filters and plain strings for
	// the blacklist; both must still parse.
	doc := `{"version":1,"filters":{"showMinimized":false,"showHiddenApps":true,"appBlacklist":["com.legacy",{"match":"Game","hide":"whenNoWindow","ignoreShortcuts":true}]}}`
	s, err := Load(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if s.Filters.ShowMinimized != VisHide || s.Filters.ShowHiddenApps != VisShow {
		t.Errorf("legacy bools mapped wrong: min=%q hidden=%q", s.Filters.ShowMinimized, s.Filters.ShowHiddenApps)
	}
	if len(s.Filters.AppBlacklist) != 2 ||
		s.Filters.AppBlacklist[0] != (BlacklistEntry{Match: "com.legacy", Hide: HideAlways}) ||
		!s.Filters.AppBlacklist[1].IgnoreShortcuts {
		t.Errorf("blacklist parsed wrong: %+v", s.Filters.AppBlacklist)
	}
	if s.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", s.Version, CurrentVersion)
	}
}

func TestNormalize_NewEnumFallbacksAndClamps(t *testing.T) {
	s := Default()
	s.Filters.ShowFullscreen = "bogus"
	s.Appearance.TitleTruncation = "bogus"
	s.Appearance.ApparitionDelayMs = 99999
	s.Behavior.MenubarIconStyle = "bogus"
	s.Behavior.UpdatePolicy = "bogus"
	s.Behavior.CrashReports = "bogus"
	got := s.Normalize()
	if got.Filters.ShowFullscreen != VisShow || got.Appearance.TitleTruncation != TruncateEnd {
		t.Errorf("enum fallback wrong: fs=%q trunc=%q", got.Filters.ShowFullscreen, got.Appearance.TitleTruncation)
	}
	if got.Appearance.ApparitionDelayMs != 2000 {
		t.Errorf("delay clamp = %d, want 2000", got.Appearance.ApparitionDelayMs)
	}
	if got.Behavior.MenubarIconStyle != MenubarIconDefault || got.Behavior.UpdatePolicy != UpdatesCheck || got.Behavior.CrashReports != CrashAsk {
		t.Errorf("behavior enum fallback wrong: %+v", got.Behavior)
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
