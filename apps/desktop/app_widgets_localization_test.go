package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
	"option-tab/internal/widgets"
)

func TestWidgetBuiltinPresentationLocalesPreservePackageAuthority(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	a.wireWidgets(widgets.Providers{})
	a.prefsOpen = true
	before := a.settingsSnapshot()
	catalog := a.GetWidgetCatalog()
	if len(catalog) != 4 {
		t.Fatalf("built-in catalog has %d entries, want 4", len(catalog))
	}
	for _, item := range catalog {
		for _, language := range []string{"en", "pt-BR", "es"} {
			if item.Name[language] == "" || item.Description[language] == "" {
				t.Errorf("%s has no %s presentation metadata", item.PackageID, language)
			}
			for _, setting := range item.Settings {
				if setting.Name[language] == "" {
					t.Errorf("%s.%s has no %s setting label", item.PackageID, setting.ID, language)
				}
			}
			instance := config.WidgetInstance{PackageID: item.PackageID, Digest: item.Digest}
			if name := a.widgetNameLocked(instance); name[language] == "" {
				t.Errorf("live widget %s has no %s name", item.PackageID, language)
			}
		}
		original, ok := widgets.Builtin(item.PackageID)
		if !ok || !item.Builtin || item.Digest != original.Digest() {
			t.Fatal("localized metadata changed package identity")
		}
		manifest := original.Manifest()
		if manifest.Name["pt-BR"] != "" || manifest.Name["es"] != "" {
			t.Fatal("presentation localization mutated the immutable manifest")
		}
		item.Name["en"] = "Changed presentation copy"
		if a.widgetNameLocked(config.WidgetInstance{PackageID: item.PackageID, Digest: item.Digest})["en"] == item.Name["en"] {
			t.Fatal("catalog presentation aliases the live widget name")
		}
	}
	if !reflect.DeepEqual(before, a.settingsSnapshot()) {
		t.Fatal("localizing metadata changed saved widget grants or enablement")
	}
}

func TestWidgetPresentationLeavesCommunityMetadataWithBuiltinIDUnchanged(t *testing.T) {
	original, _ := widgets.Builtin("org.optiontab.clock")
	manifest := original.Manifest()
	manifest.Name = widgets.Localized{"en": "User clock", "pt-BR": "My unchanged title"}
	manifest.Description = widgets.Localized{"en": "A community literal"}
	manifest.Settings[0].Name = widgets.Localized{"en": "My custom zone"}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, err := writer.Create("widget.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	community, err := widgets.Preview(context.Background(), bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	item := widgetCatalogItem(community)
	if item.Builtin || item.Digest == original.Digest() || !reflect.DeepEqual(item.Name, manifest.Name) || !reflect.DeepEqual(item.Description, manifest.Description) || !reflect.DeepEqual(item.Settings, manifest.Settings) {
		t.Fatal("community metadata was localized using its claimed built-in package ID")
	}
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	a.wireWidgets(widgets.Providers{}, community)
	name := a.widgetNameLocked(config.WidgetInstance{PackageID: item.PackageID, Digest: item.Digest})
	if !reflect.DeepEqual(name, manifest.Name) {
		t.Fatal("live community widget name changed")
	}
}
