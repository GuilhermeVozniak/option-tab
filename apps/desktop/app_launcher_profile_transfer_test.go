package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"option-tab/internal/widgets"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func transferApp(t *testing.T) *App {
	t.Helper()
	a := newApp(fake.New(), config.Default(), "")
	a.wireLauncherItems(nil, nil)
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
	t.Cleanup(a.stopCapture)
	return a
}

func transferDocument(t *testing.T) string {
	t.Helper()
	p := config.DefaultReplacementDock().Profiles[0]
	p.Name = "Imported"
	p.Items = []config.LauncherItem{{ID: "app", Kind: "app", Label: "Owned", ReferenceID: strings.Repeat("a", 32), IconID: strings.Repeat("b", 64)}}
	p.Widgets[0].Enabled = true
	p.Widgets[0].Grants = []string{"clock.read"}
	b, err := json.Marshal(map[string]any{"format": "option-tab.launcher-profile", "version": 1, "profile": p})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLauncherProfileTransferReviewReadonlyAndImportUnassigned(t *testing.T) {
	a := transferApp(t)
	s := a.settingsSnapshot()
	s.Behavior.Language = "pt-BR"
	s.ReplacementDock.Enabled = true
	s.ReplacementDock.Rules = []config.LauncherProfileRule{{ID: "rule", Enabled: true, BundleID: "org.example.app", ProfileID: "default"}}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	a.settingsPath = filepath.Join(t.TempDir(), "settings.json")
	before := a.GetSettings()
	doc := transferDocument(t)
	review, err := a.PreviewLauncherProfileImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	if a.GetSettings() != before {
		t.Fatal("review wrote settings")
	}
	if _, err := os.Stat(a.settingsPath); !os.IsNotExist(err) {
		t.Fatal("review wrote disk")
	}
	result, err := a.ImportLauncherProfile(doc, review.Digest, review.Revision)
	if err != nil {
		t.Fatal(err)
	}
	got := a.settingsSnapshot()
	if len(got.ReplacementDock.Profiles) != 2 || !got.ReplacementDock.Enabled || !reflect.DeepEqual(got.ReplacementDock.Bindings, s.ReplacementDock.Bindings) || !reflect.DeepEqual(got.ReplacementDock.Rules, s.ReplacementDock.Rules) || got.Behavior.Language != "pt-BR" {
		t.Fatal("import changed active configuration")
	}
	p := got.ReplacementDock.Profiles[1]
	if p.ID == "default" || result.ProfileID != p.ID || !strings.HasPrefix(p.Items[0].ReferenceID, "selection-") || p.Items[0].ReferenceID == strings.Repeat("a", 32) || p.Items[0].IconID != "" || p.Widgets[0].Enabled || len(p.Widgets[0].Grants) > 0 {
		t.Fatal("authority retained", p)
	}
	if result.SettingsJSON != a.GetSettings() {
		t.Fatal("canonical fallback differs")
	}
	review, err = a.PreviewLauncherProfileImport(doc)
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ImportLauncherProfile(doc, review.Digest, review.Revision)
	if err != nil {
		t.Fatal(err)
	}
	again := a.settingsSnapshot().ReplacementDock.Profiles[2]
	if second.ProfileID == result.ProfileID || again.Items[0].ReferenceID == p.Items[0].ReferenceID {
		t.Fatal("repeat import reused authority placeholders")
	}
}

func TestLauncherProfileTransferRefusesStaleClosedAndSaveFailure(t *testing.T) {
	for _, kind := range []string{"digest", "revision", "prefs", "inactive", "save"} {
		t.Run(kind, func(t *testing.T) {
			a := transferApp(t)
			doc := transferDocument(t)
			review, err := a.PreviewLauncherProfileImport(doc)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "digest":
				doc += " "
			case "revision":
				a.settingsMu.Lock()
				a.settings.ReplacementDock.Profiles[0].Name = "Changed"
				a.settingsMu.Unlock()
			case "prefs":
				a.viewMu.Lock()
				a.prefsOpen = false
				a.viewMu.Unlock()
			case "inactive":
				a.viewMu.Lock()
				a.sessionInactive = true
				a.viewMu.Unlock()
			case "save":
				base := filepath.Join(t.TempDir(), "file")
				if err := os.WriteFile(base, []byte("owned"), 0o600); err != nil {
					t.Fatal(err)
				}
				a.settingsPath = filepath.Join(base, "settings.json")
			}
			before := a.GetSettings()
			if _, err := a.ImportLauncherProfile(doc, review.Digest, review.Revision); err == nil {
				t.Fatal("invalid import saved")
			}
			if a.GetSettings() != before {
				t.Fatal("refusal changed settings")
			}
		})
	}
}

func TestLauncherProfileTransferCapacityAndExportReadonly(t *testing.T) {
	a := transferApp(t)
	s := a.settingsSnapshot()
	for i := 1; i < 8; i++ {
		p := s.ReplacementDock.Profiles[0]
		p.ID = "profile-" + string(rune('a'+i))
		s.ReplacementDock.Profiles = append(s.ReplacementDock.Profiles, p)
	}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	if _, err := a.PreviewLauncherProfileImport(transferDocument(t)); err == nil {
		t.Fatal("ninth profile admitted")
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.viewMu.Unlock()
	before := a.GetSettings()
	if _, err := a.GetLauncherProfileExport("default"); err != nil {
		t.Fatal(err)
	}
	if a.GetSettings() != before {
		t.Fatal("export mutated config")
	}
}

func TestLauncherProfileTransferCannotPersistDockTooLargeToReload(t *testing.T) {
	a := transferApp(t)
	p := config.DefaultReplacementDock().Profiles[0]
	p.Widgets = nil
	for i := 0; i < 4; i++ {
		settings := map[string]widgets.Value{}
		for j := 0; j < 16; j++ {
			v := strings.Repeat("x", 1024)
			settings["setting"+string(rune('a'+j))] = widgets.Value{Text: &v}
		}
		p.Widgets = append(p.Widgets, config.WidgetInstance{ID: "widget" + string(rune('a'+i)), PackageID: "org.example.widget", Digest: strings.Repeat("f", 64), Settings: settings})
	}
	b, err := config.ExportLauncherProfile(p)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		review, err := a.PreviewLauncherProfileImport(string(b))
		if err != nil {
			return
		}
		before := a.GetSettings()
		_, err = a.ImportLauncherProfile(string(b), review.Digest, review.Revision)
		if err != nil {
			if a.GetSettings() != before {
				t.Fatal("capacity refusal changed settings")
			}
			return
		}
	}
	var persisted bytes.Buffer
	if err := config.Save(&persisted, a.settingsSnapshot()); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(bytes.NewReader(persisted.Bytes())); err != nil {
		t.Fatal("import saved profile collection rejected on restart", err)
	}
}
