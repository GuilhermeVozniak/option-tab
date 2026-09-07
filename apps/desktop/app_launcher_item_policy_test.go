package main

import (
	"testing"

	"option-tab/internal/config"

	"option-tab/internal/platform"
)

func TestPinnedItemIntegrationUsesSavedReferenceAndExactRunningMerge(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(map[bool]string{false: "stopped", true: "running"}[running], func(t *testing.T) {
			a, _, q, _ := launcherIntegrationApp(t)
			ref := platform.LauncherReference{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "app", Label: "Owned fixture", BundleID: "test.launcher.fixture", State: "ready", Revision: 8}
			if running {
				ref.Process = platform.ProcessIdentity{PID: 4242, StartSeconds: 123}
			}
			source := &launcherActionReferenceFixture{launcherReferenceFixture: launcherReferenceFixture{records: map[string]platform.LauncherReference{ref.ID: ref}}, entered: make(chan platform.LauncherItemAction, 1)}
			a.wireLauncherItems(source, nil)
			s := a.settingsSnapshot()
			s.ReplacementDock.Profiles[0].Items = []config.LauncherItem{{ID: "editor", Kind: "app", Label: "Editor", ReferenceID: ref.ID}}
			a.saveMu.Lock()
			err := a.saveSettingsLocked(s)
			a.saveMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			p := launcherIntegrationVisible(t, a, q)
			if len(p.Items) < 1 || p.Items[0].ID != "pin:editor" || p.Items[0].Status != "ready" || p.Items[0].Running != running {
				t.Fatalf("incorrect pin: %+v", p.Items)
			}
			if running && len(p.Items) != 1 {
				t.Fatal("exact running instance duplicated")
			}
			if !running && len(p.Items) != 2 {
				t.Fatal("different installation hidden")
			}
			if err = a.ActivateLauncherItem(p.Epoch, p.DisplayUUID, p.Session, p.Revision, "pin:editor"); err != nil {
				t.Fatal(err)
			}
			captured := <-source.entered
			if captured.ReferenceID != ref.ID || captured.ReferenceRevision != ref.Revision || captured.Process != ref.Process || source.mutations.Load() != 1 {
				t.Fatal("wrong selected reference dispatched", captured)
			}
		})
	}
}

func TestPinnedAppPolicyPermitsSelectedNonRunningApp(t *testing.T) {
	a := launcherItemsApp(t, &launcherReferenceFixture{records: map[string]platform.LauncherReference{}})
	ref := platform.LauncherReference{Kind: "app", BundleID: "org.example.editor", Label: "Editor", State: "ready", Revision: 1}
	if !a.launcherReferenceEligible(ref) {
		t.Fatal("selected stopped application excluded")
	}
	ref.BundleID = selfBundleID
	if a.launcherReferenceEligible(ref) {
		t.Fatal("self application admitted")
	}
}
