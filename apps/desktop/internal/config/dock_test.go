package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestDockValidationRejectsOutOfRangeValues(t *testing.T) {
	for name, mutate := range map[string]func(*Settings){
		"hover delay":    func(s *Settings) { s.Dock.HoverDelayMs = 2001 },
		"dismiss delay":  func(s *Settings) { s.Dock.DismissDelayMs = -1 },
		"hover slop":     func(s *Settings) { s.Dock.HoverSlopPx = 33 },
		"bridge padding": func(s *Settings) { s.Dock.BridgePaddingPx = 49 },
		"card spacing":   func(s *Settings) { s.Dock.CardSpacingPx = 25 },
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

func TestDockMonitorLockDefaultsAndNormalization(t *testing.T) {
	s := Default()
	if s.Dock.MonitorLock.Enabled || s.Dock.MonitorLock.Target != "main" || s.Dock.MonitorLock.BypassModifier != "option" {
		t.Fatalf("defaults=%+v", s.Dock.MonitorLock)
	}
	s.Dock.Enabled = false
	s.Dock.MonitorLock = DockMonitorLockSettings{Enabled: true, Target: "display", DisplayUUID: "11111111-1111-1111-1111-111111111111", BypassModifier: "command"}
	got := s.Normalize()
	if !got.Dock.MonitorLock.Enabled || got.Dock.MonitorLock.DisplayUUID != "11111111-1111-1111-1111-111111111111" || got.Dock.Enabled {
		t.Fatalf("independence lost: %+v", got.Dock)
	}
	s.Dock.MonitorLock.Target, s.Dock.MonitorLock.BypassModifier = "bad", "bad"
	got = s.Normalize()
	if got.Dock.MonitorLock.Target != "main" || got.Dock.MonitorLock.BypassModifier != "option" || got.Dock.MonitorLock.DisplayUUID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("normalized=%+v", got.Dock.MonitorLock)
	}
}

func TestDockInputDefaultsAndMissingSchema3Object(t *testing.T) {
	for name, s := range map[string]Settings{
		"default": Default(),
		"loaded": func() Settings {
			got, err := Load(strings.NewReader(`{"version":3,"dock":{"enabled":true}}`))
			if err != nil {
				t.Fatal(err)
			}
			return got
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if s.Dock.Input.ClickToHide || s.Dock.Input.ScrollShowHide || s.Dock.Input.ModifiedRightClick || s.Dock.Input.PreviewDrag ||
				s.Dock.Input.SwipeTowardDock != PointerNone || s.Dock.Input.SwipeAwayFromDock != PointerNone ||
				s.Dock.Input.SwipePrevious != PointerNone || s.Dock.Input.SwipeNext != PointerNone || s.Dock.Input.AeroShakeAction != "none" {
				t.Fatalf("input is not disabled by default: %+v", s.Dock.Input)
			}
		})
	}
}

func TestDockInputValidationNormalizationAndCopy(t *testing.T) {
	s := Default()
	s.Dock.Input.SwipeTowardDock = "invalid"
	s.Dock.Input.SwipeAwayFromDock = "invalid"
	s.Dock.Input.SwipePrevious = "invalid"
	s.Dock.Input.SwipeNext = "invalid"
	s.Dock.Input.AeroShakeAction = "deleteOthers"
	if s.Validate() == nil {
		t.Fatal("Validate accepted invalid Dock input")
	}
	got := s.Normalize()
	if got.Dock.Input.SwipeTowardDock != PointerNone ||
		got.Dock.Input.SwipeAwayFromDock != PointerNone ||
		got.Dock.Input.SwipePrevious != PointerNone ||
		got.Dock.Input.SwipeNext != PointerNone ||
		got.Dock.Input.AeroShakeAction != "none" {
		t.Fatalf("invalid input survived normalization: %+v", got.Dock.Input)
	}
	got.Dock.Input.ClickToHide = true
	if s.Dock.Input.ClickToHide {
		t.Fatal("Normalize mutated or aliased Dock input")
	}
}

func TestDockInputExplicitDisabledRoundTrip(t *testing.T) {
	s := Default()
	s.Dock.Input = DockInputSettings{AeroShakeAction: "none", SwipeTowardDock: PointerNone, SwipeAwayFromDock: PointerNone, SwipePrevious: PointerNone, SwipeNext: PointerNone}
	var buf bytes.Buffer
	if err := Save(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dock.Input != s.Dock.Input {
		t.Fatalf("disabled input changed: got=%+v want=%+v", got.Dock.Input, s.Dock.Input)
	}
}

func TestNormalizeClampsDockAndDoesNotAlias(t *testing.T) {
	s := Default()
	s.Dock.HoverDelayMs = -1
	s.Dock.DismissDelayMs = 3000
	s.Dock.HoverSlopPx = 99
	s.Dock.BridgePaddingPx = -3
	s.Dock.CardSpacingPx = 99
	s.Dock.Appearance.AccentColor = "#abcdef"
	got := s.Normalize()
	if got.Dock.HoverDelayMs != 0 || got.Dock.DismissDelayMs != 2000 || got.Dock.HoverSlopPx != 32 || got.Dock.BridgePaddingPx != 0 || got.Dock.CardSpacingPx != 24 {
		t.Fatalf("dock clamps: %+v", got.Dock)
	}
	got.Dock.Appearance.AccentColor = "#000000"
	if s.Dock.Appearance.AccentColor != "#abcdef" {
		t.Fatal("Normalize aliased dock settings")
	}
}

func TestDockCardSpacingPreservesZero(t *testing.T) {
	s := Default()
	if s.Dock.CardSpacingPx != 7 {
		t.Fatalf("default spacing=%d want 7", s.Dock.CardSpacingPx)
	}
	s.Dock.CardSpacingPx = 0
	var buf bytes.Buffer
	if err := Save(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dock.CardSpacingPx != 0 {
		t.Fatalf("zero spacing changed to %d", got.Dock.CardSpacingPx)
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
