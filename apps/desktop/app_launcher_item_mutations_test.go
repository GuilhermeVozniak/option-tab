package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"option-tab/internal/platform"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
)

func runtimeMutationFixture(t *testing.T) (*App, launcher.Presentation) {
	t.Helper()
	a, _, q, _ := launcherIntegrationApp(t)
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].RuntimeReorder = true
	s.ReplacementDock.Profiles[0].Widgets = nil
	s.ReplacementDock.Profiles[0].Items = []config.LauncherItem{{ID: "a", Kind: "app", Label: "A", ReferenceID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, {ID: "b", Kind: "app", Label: "B", ReferenceID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}
	a.saveMu.Lock()
	err := a.saveSettingsLocked(s)
	a.saveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return a, launcherIntegrationVisible(t, a, q)
}

func mutateRuntime(a *App, p launcher.Presentation, kind string) error {
	return a.MutateLauncherItems(p.Epoch, p.DisplayUUID, p.Session, p.Revision, p.ItemsRevision, config.LauncherItemMutation{Kind: kind, ItemID: "pin:a", TargetID: "pin:b"})
}

func TestRuntimeItemMutationCreatesGroup(t *testing.T) {
	a, p := runtimeMutationFixture(t)
	before := a.GetSettingsState()
	if err := mutateRuntime(a, p, "addToGroup"); err != nil {
		t.Fatal(err)
	}
	items := a.settingsSnapshot().ReplacementDock.Profiles[0].Items
	if len(items) != 3 || items[1].Kind != "group" {
		t.Fatal(items)
	}
	if _, err := a.SaveSettingsAtRevision(before.JSON, before.Revision); err == nil {
		t.Fatal("old preferences overwrote runtime commit")
	}
	if err := mutateRuntime(a, p, "moveBefore"); err == nil {
		t.Fatal("stale scope reused")
	}
}

func TestRuntimeItemMutationRefusals(t *testing.T) {
	for _, mode := range []string{"hash", "scope", "off", "preferences", "disk", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			a, p := runtimeMutationFixture(t)
			before := a.settingsSnapshot()
			kind := "addToGroup"
			switch mode {
			case "hash":
				p.ItemsRevision = "wrong"
			case "scope":
				p.Revision++
			case "off":
				a.settingsMu.Lock()
				a.settings.ReplacementDock.Profiles[0].RuntimeReorder = false
				a.settingsMu.Unlock()
			case "preferences":
				a.viewMu.Lock()
				a.prefsOpen = true
				a.viewMu.Unlock()
			case "disk":
				path := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(path, []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
				a.settingsPath = filepath.Join(path, "settings.json")
			case "invalid":
				kind = "unknown"
			}
			if err := mutateRuntime(a, p, kind); err == nil {
				t.Fatal("refusal missing")
			}
			if mode != "off" && !reflect.DeepEqual(before, a.settingsSnapshot()) {
				t.Fatal("failed mutation changed settings")
			}
		})
	}
}

func TestRuntimeItemMutationNativeSwapAndSaveWait(t *testing.T) {
	for _, mode := range []string{"nativeSwap", "saveWait"} {
		t.Run(mode, func(t *testing.T) {
			a, p := runtimeMutationFixture(t)
			before := a.settingsSnapshot()
			entered, release := make(chan struct{}), make(chan struct{})
			a.viewMu.Lock()
			d := a.launcher.hosts[p.Session].window
			a.viewMu.Unlock()
			d.mu.Lock()
			old := d.current.panel.(*launcherIntegrationPanel)
			d.current.panel = &mutationValidatingPanel{launcherIntegrationPanel: old, validate: func() { close(entered); <-release }}
			d.mu.Unlock()
			if mode == "saveWait" {
				a.saveMu.Lock()
			}
			done := make(chan error, 1)
			go func() { done <- mutateRuntime(a, p, "addToGroup") }()
			<-entered
			if mode == "nativeSwap" {
				d.mu.Lock()
				d.current.panel = &launcherIntegrationPanel{token: old.token + 1}
				d.mu.Unlock()
			} else {
				a.viewMu.Lock()
				a.prefsOpen = true
				a.viewMu.Unlock()
			}
			close(release)
			if mode == "saveWait" {
				a.saveMu.Unlock()
			}
			if err := <-done; err == nil {
				t.Fatal("obsolete preparation committed")
			}
			if !reflect.DeepEqual(before, a.settingsSnapshot()) {
				t.Fatal("changed settings")
			}
		})
	}
}

type mutationValidatingPanel struct {
	*launcherIntegrationPanel
	validate func()
}

func (p *mutationValidatingPanel) ValidateLauncherPanel(ctx context.Context, display string) error {
	p.validate()
	return p.launcherIntegrationPanel.ValidateLauncherPanel(ctx, display)
}

var _ platform.LauncherPanelValidator = (*mutationValidatingPanel)(nil)

func TestRuntimeItemMutationFinalNativeRefusal(t *testing.T) {
	a, p := runtimeMutationFixture(t)
	before := a.settingsSnapshot()
	a.viewMu.Lock()
	d := a.launcher.hosts[p.Session].window
	a.viewMu.Unlock()
	var calls atomic.Int32
	d.mu.Lock()
	old := d.current.panel.(*launcherIntegrationPanel)
	d.current.panel = &mutationValidatingPanel{launcherIntegrationPanel: old, validate: func() {
		if calls.Add(1) == 2 {
			old.unavailable.Store(true)
		}
	}}
	d.mu.Unlock()
	if err := mutateRuntime(a, p, "addToGroup"); err == nil {
		t.Fatal("final native refusal ignored")
	}
	if calls.Load() != 2 || !reflect.DeepEqual(before, a.settingsSnapshot()) {
		t.Fatal("final preparation changed settings")
	}
}

func TestRuntimeItemMutationPreferenceCommitDuringNativePreparation(t *testing.T) {
	a, p := runtimeMutationFixture(t)
	before := a.GetSettingsState()
	entered, release := make(chan struct{}), make(chan struct{})
	a.viewMu.Lock()
	d := a.launcher.hosts[p.Session].window
	a.viewMu.Unlock()
	d.mu.Lock()
	old := d.current.panel.(*launcherIntegrationPanel)
	d.current.panel = &mutationValidatingPanel{launcherIntegrationPanel: old, validate: func() { close(entered); <-release }}
	d.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- mutateRuntime(a, p, "addToGroup") }()
	<-entered
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].RuntimeReorder = false
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.SaveSettingsAtRevision(string(data), before.Revision); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err = <-done; err == nil {
		t.Fatal("runtime overwrote preferences winner")
	}
	if len(a.settingsSnapshot().ReplacementDock.Profiles[0].Items) != 2 {
		t.Fatal("loser saved")
	}
}
