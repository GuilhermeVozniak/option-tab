package main

import (
	"encoding/json"
	"errors"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
)

func TestLauncherFocusRuleReplacesAppHostAndRejectsPreviousClicks(t *testing.T) {
	a, backend, q, panel := launcherIntegrationApp(t)
	settings := a.settingsSnapshot()
	work := settings.ReplacementDock.Profiles[0]
	work.ID, work.Name = "work", "Work"
	work.Appearance.CornerRadiusPx = 6
	settings.ReplacementDock.Profiles = append(settings.ReplacementDock.Profiles, work)
	settings.ReplacementDock.Rules = []config.LauncherProfileRule{{ID: "focus", Enabled: true, BundleID: "test.launcher.fixture", ProfileID: "work", BindingID: "main"}}
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(string(raw)); err != nil {
		t.Fatal(err)
	}
	base := launcherIntegrationVisible(t, a, q)
	backend.focus("test.launcher.fixture")
	if a.GetLauncherState(base.Session).Visible {
		t.Fatal("old host admission survived focused profile change")
	}
	if err := activateIntegration(a, base); !errors.Is(err, launcher.ErrRetired) {
		t.Fatalf("old profile click=%v", err)
	}
	focused := launcherIntegrationVisible(t, a, q)
	if focused.ProfileID != "work" || focused.Session == base.Session || panel.style.Load().CornerRadiusPx != 6 {
		t.Fatalf("focused presentation=%+v", focused)
	}
	backend.focus("test.unmatched")
	restored := launcherIntegrationVisible(t, a, q)
	if restored.ProfileID != "default" || restored.Session == base.Session || restored.Session == focused.Session {
		t.Fatal("base profile reused a retired host")
	}
	if backend.mutations.Load() != 0 {
		t.Fatal("profile switching performed an application action")
	}
}
