package config

import "testing"

func TestDockValidationRejectsOutOfRangeValues(t *testing.T) {
	for name, mutate := range map[string]func(*Settings){
		"hover delay":    func(s *Settings) { s.Dock.HoverDelayMs = 2001 },
		"dismiss delay":  func(s *Settings) { s.Dock.DismissDelayMs = -1 },
		"hover slop":     func(s *Settings) { s.Dock.HoverSlopPx = 33 },
		"bridge padding": func(s *Settings) { s.Dock.BridgePaddingPx = 49 },
	} {
		t.Run(name, func(t *testing.T) {
			s := Default()
			mutate(&s)
			if s.Validate() == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeClampsDockAndDoesNotAlias(t *testing.T) {
	s := Default()
	s.Dock.HoverDelayMs = -1
	s.Dock.DismissDelayMs = 3000
	s.Dock.HoverSlopPx = 99
	s.Dock.BridgePaddingPx = -3
	s.Dock.Appearance.AccentColor = "#abcdef"
	got := s.Normalize()
	if got.Dock.HoverDelayMs != 0 || got.Dock.DismissDelayMs != 2000 || got.Dock.HoverSlopPx != 32 || got.Dock.BridgePaddingPx != 0 {
		t.Fatalf("dock clamps: %+v", got.Dock)
	}
	got.Dock.Appearance.AccentColor = "#000000"
	if s.Dock.Appearance.AccentColor != "#abcdef" {
		t.Fatal("Normalize aliased dock settings")
	}
}

func TestNormalizeDoesNotMutateAppBindings(t *testing.T) {
	s := Default()
	s.AppSwitcher.Behavior.ActionBindings["Tab"] = ActionClose
	got := s.Normalize()
	if _, exists := got.AppSwitcher.Behavior.ActionBindings["Tab"]; exists {
		t.Fatal("invalid app binding survived")
	}
	if s.AppSwitcher.Behavior.ActionBindings["Tab"] != ActionClose {
		t.Fatal("Normalize mutated app binding input")
	}
}
