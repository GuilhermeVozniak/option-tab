package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func legacyReplacement(t *testing.T) map[string]any {
	t.Helper()
	s := DefaultReplacementDock()
	s.Enabled = true
	s.Profiles[0].IconPx = 48
	s.Profiles[0].Widgets[0].Enabled = true
	s.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err = json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	raw["version"] = float64(1)
	for _, v := range raw["profiles"].([]any) {
		p := v.(map[string]any)
		delete(p, "alignment")
		delete(p, "appearance")
		delete(p, "magnification")
		delete(p, "interactions")
	}
	return raw
}

func loadReplacementMap(t *testing.T, raw map[string]any) (Settings, error) {
	t.Helper()
	b, err := json.Marshal(map[string]any{"replacementDock": raw})
	if err != nil {
		t.Fatal(err)
	}
	return Load(strings.NewReader(string(b)))
}

func TestReplacementV1MigrationSurvivesTopLevelDecode(t *testing.T) {
	s, err := loadReplacementMap(t, legacyReplacement(t))
	if err != nil {
		t.Fatal(err)
	}
	if s.ReplacementDock.Version != 2 || !s.ReplacementDock.Enabled || s.ReplacementDock.Profiles[0].IconPx != 48 || s.ReplacementDock.Profiles[0].Alignment != "center" || s.ReplacementDock.Profiles[0].Appearance != DefaultLauncherAppearance() || len(s.ReplacementDock.Profiles[0].Widgets[0].Grants) != 1 {
		t.Fatalf("migration lost values: %+v", s.ReplacementDock)
	}
}

func TestReplacementLegacyValidationBeforeDefaults(t *testing.T) {
	for _, field := range []string{"edge", "iconPx", "alignment", "appearance"} {
		raw := legacyReplacement(t)
		p := raw["profiles"].([]any)[0].(map[string]any)
		switch field {
		case "edge":
			p[field] = "left"
		case "iconPx":
			p[field] = 0
		case "alignment":
			p[field] = "center"
		case "appearance":
			p[field] = map[string]any{}
		}
		if _, err := loadReplacementMap(t, raw); err == nil {
			t.Fatalf("invalid legacy %s accepted", field)
		}
	}
	raw := legacyReplacement(t)
	raw["version"] = float64(3)
	if _, err := loadReplacementMap(t, raw); err == nil {
		t.Fatal("future version accepted")
	}
}

func TestReplacementAppearanceValidationAndCopy(t *testing.T) {
	s := DefaultReplacementDock()
	if s.Version != 2 {
		t.Fatal("wrong default version")
	}
	for _, edit := range []func(*LauncherAppearance){func(a *LauncherAppearance) { a.Opacity = .34 }, func(a *LauncherAppearance) { a.BorderOpacity = .51 }, func(a *LauncherAppearance) { a.Tint = "red" }, func(a *LauncherAppearance) { a.Material = "script" }, func(a *LauncherAppearance) { a.ItemSpacingPx = 21 }} {
		c := CloneReplacementDock(s)
		edit(&c.Profiles[0].Appearance)
		if ValidateReplacementDock(c) == nil {
			t.Fatal("invalid appearance accepted")
		}
		if s.Profiles[0].Appearance != DefaultLauncherAppearance() {
			t.Fatal("profile appearance alias")
		}
	}
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		for _, layout := range []string{"floating", "fullWidth"} {
			for _, alignment := range []string{"start", "center", "end"} {
				c := CloneReplacementDock(s)
				c.Profiles[0].Edge = edge
				c.Profiles[0].Layout = layout
				c.Profiles[0].Alignment = alignment
				if err := ValidateReplacementDock(c); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
