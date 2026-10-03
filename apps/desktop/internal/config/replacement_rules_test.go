package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLauncherRulesValidationCopyAndLegacy(t *testing.T) {
	s := DefaultReplacementDock()
	s.Rules = []LauncherProfileRule{{ID: "focus", Enabled: true, BundleID: "org.example.Editor", ProfileID: "default", BindingID: "main"}}
	if err := ValidateReplacementDock(s); err != nil {
		t.Fatal(err)
	}
	c := CloneReplacementDock(s)
	c.Rules[0].BundleID = "changed"
	if s.Rules[0].BundleID != "org.example.Editor" {
		t.Fatal("rules alias")
	}
	for _, change := range []func(*ReplacementDockSettings){func(s *ReplacementDockSettings) { s.Rules = append(s.Rules, s.Rules[0]) }, func(s *ReplacementDockSettings) { s.Rules[0].BundleID = "bad id" }, func(s *ReplacementDockSettings) { s.Rules[0].BundleID = "/path" }, func(s *ReplacementDockSettings) { s.Rules[0].ProfileID = "missing" }, func(s *ReplacementDockSettings) { s.Rules[0].BindingID = "missing" }} {
		c = CloneReplacementDock(s)
		change(&c)
		if ValidateReplacementDock(c) == nil {
			t.Fatal("invalid rule accepted")
		}
	}
	legacy := legacyReplacement(t)
	legacy["rules"] = []any{}
	if _, err := loadReplacementMap(t, legacy); err == nil {
		t.Fatal("legacy rules accepted")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(strings.NewReader(`{"replacementDock":` + string(raw) + `}`))
	if err != nil || len(loaded.ReplacementDock.Rules) != 1 {
		t.Fatal("rule round trip", err)
	}
}

func TestLauncherRuleReferenceChangesAreAtomic(t *testing.T) {
	s := DefaultReplacementDock()
	other := s.Profiles[0]
	other.ID = "other"
	s.Profiles = append(s.Profiles, other)
	s.Rules = []LauncherProfileRule{{ID: "rule", Enabled: true, BundleID: "org.example.Editor", ProfileID: "other", BindingID: "main"}}
	c := CloneReplacementDock(s)
	c.Profiles = c.Profiles[:1]
	if ValidateReplacementDock(c) == nil {
		t.Fatal("deleted profile left dangling rule")
	}
	c.Rules[0].ProfileID = "default"
	if err := ValidateReplacementDock(c); err != nil {
		t.Fatal("atomic reassignment failed", err)
	}
	c.Bindings = nil
	if ValidateReplacementDock(c) == nil {
		t.Fatal("deleted binding left scoped rule")
	}
	c.Rules = nil
	if err := ValidateReplacementDock(c); err != nil {
		t.Fatal(err)
	}
	for i := range 9 {
		s.Rules = append(s.Rules, LauncherProfileRule{ID: string(rune('a' + i)), BundleID: "org.example.Other", ProfileID: "default"})
	}
	if ValidateReplacementDock(s) == nil {
		t.Fatal("unbounded rules accepted")
	}
}
