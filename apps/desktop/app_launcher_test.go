package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func TestLauncherSettingsCopyAndDefaultAdmission(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	defer a.stopCapture()
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	s.ReplacementDock.Bindings[0].ProfileID = "mutated"
	if original := a.settingsSnapshot().ReplacementDock; len(original.Profiles[0].Widgets[0].Grants) != 0 || original.Bindings[0].ProfileID != "default" {
		t.Fatal("launcher settings snapshot aliases widget grants or bindings")
	}
	status := a.GetLauncherStatus()
	if status.Enabled || status.ClockPackageID != config.BuiltinClockPackage || status.ClockDigest != config.BuiltinClockDigest {
		t.Fatalf("default launcher status: %+v", status)
	}
	if err := a.ActivateLauncherItem(1, "display", 1, 1, "item-1"); err == nil {
		t.Fatal("disabled launcher accepted an action")
	}
	a.stopCapture()
	if state := a.GetLauncherState(1); state.Visible || len(state.Items) != 0 {
		t.Fatal("shutdown exposed a live launcher")
	}
}

func TestLauncherRecoveryStaysDisabledWhenPersistenceFails(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("owned fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := config.Default()
	s.ReplacementDock.Enabled = true
	s.Dock.Enabled = true
	s.Dock.Input.ClickToHide = true
	s.Dock.MonitorLock.Enabled = true
	a := newApp(fake.New(), s, filepath.Join(parent, "settings.json"))
	defer a.stopCapture()
	if err := a.UseNativeDock(); err == nil {
		t.Fatal("persistence failure was hidden")
	}
	status := a.GetLauncherStatus()
	if !status.RecoveryLatched || status.Reason != "saveFailed" || a.launcherWantedLocked() {
		t.Fatalf("failed save restored launcher: %+v", status)
	}
	if got := a.settingsSnapshot(); !got.ReplacementDock.Enabled || got.Dock != s.Dock {
		t.Fatal("recovery mutated persisted settings before a successful save")
	}
	// A later unrelated successful save must not undo failed recovery.
	a.settingsPath = ""
	s.Behavior.Language = "pt-BR"
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(string(b)); err != nil {
		t.Fatal(err)
	}
	if !a.GetLauncherStatus().RecoveryLatched {
		t.Fatal("unrelated settings save cleared recovery latch")
	}
	if err := a.UseNativeDock(); err != nil {
		t.Fatal(err)
	}
	if a.settingsSnapshot().ReplacementDock.Enabled {
		t.Fatal("successful recovery did not persist disabled")
	}
	s = a.settingsSnapshot()
	s.ReplacementDock.Enabled = true
	b, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(string(b)); err != nil {
		t.Fatal(err)
	}
	if a.GetLauncherStatus().RecoveryLatched {
		t.Fatal("explicit re-enable after saved disable stayed latched")
	}
}
