package config

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"option-tab/internal/widgets"
)

func TestLauncherTransferNeverRestoresPrivateAuthority(t *testing.T) {
	p := DefaultReplacementDock().Profiles[0]
	p.Items = []LauncherItem{{ID: "app", Kind: "app", Label: "Owned", ReferenceID: strings.Repeat("a", 32), IconID: strings.Repeat("b", 64)}}
	p.Widgets[0].Enabled = true
	p.Widgets[0].Grants = []string{"clock.read"}
	b, err := ExportLauncherProfile(p)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(strings.Repeat("a", 32))) || bytes.Contains(b, []byte(strings.Repeat("b", 64))) {
		t.Fatal("private identifier exported")
	}
	got, err := ParseLauncherProfile(b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got.Items[0].ReferenceID, "selection-") || got.Widgets[0].Enabled || len(got.Widgets[0].Grants) != 0 || got.Items[0].IconID != "" {
		t.Fatal("authority retained", got)
	}
	if p.Items[0].IconID == "" || !p.Widgets[0].Enabled {
		t.Fatal("export mutated original")
	}
}

func TestLauncherTransferHostileImportIsInertAndStrict(t *testing.T) {
	p := DefaultReplacementDock().Profiles[0]
	p.Items = []LauncherItem{{ID: "app", Kind: "app", Label: "Owned", ReferenceID: strings.Repeat("a", 32)}}
	p.Widgets[0].Enabled = true
	p.Widgets[0].Grants = []string{"clock.read"}
	raw, _ := json.Marshal(launcherProfileEnvelope{Format: launcherProfileFormat, Version: 1, Profile: p})
	parsed, err := ParseLauncherProfile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Items[0].ReferenceID == p.Items[0].ReferenceID || parsed.Widgets[0].Enabled || len(parsed.Widgets[0].Grants) > 0 {
		t.Fatal("hostile authority restored")
	}
	for name, doc := range map[string][]byte{
		"trailing":        append(append([]byte{}, raw...), []byte(` {}`)...),
		"duplicate":       bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		"nestedDuplicate": bytes.Replace(raw, []byte(`"name":"Default"`), []byte(`"name":"Default","Name":"Other"`), 1),
		"unknown":         bytes.Replace(raw, []byte(`"alignment"`), []byte(`"untrusted"`), 1),
		"version":         bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":2`), 1),
		"oversize":        []byte(strings.Repeat(" ", LauncherProfileTransferLimit+1)),
		"badUTF8":         append(append([]byte{}, raw...), 0xff),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseLauncherProfile(doc); err == nil {
				t.Fatal("accepted invalid transfer")
			}
		})
	}
}

func TestLauncherTransferRelationshipsAndTypedValuesCopied(t *testing.T) {
	p := DefaultReplacementDock().Profiles[0]
	text := "local value"
	p.Widgets = []WidgetInstance{{ID: "one", PackageID: "org.example.widget", Digest: strings.Repeat("f", 64), Settings: map[string]widgets.Value{"label": {Text: &text}}}, {ID: "two", PackageID: BuiltinClockPackage, Digest: BuiltinClockDigest}}
	p.Stacks = []WidgetStack{{ID: "stack", Name: "Tools", Members: []string{"one", "two"}, ActiveID: "one"}}
	p.Items = []LauncherItem{{ID: "app", Kind: "app", Label: "App", ReferenceID: strings.Repeat("a", 32)}, {ID: "group", Kind: "group", Label: "Group", Members: []string{"app"}}, {ID: "folder", Kind: "folder", Label: "Folder", FolderView: "grid", ReferenceID: strings.Repeat("b", 32)}, {ID: "link", Kind: "link", Label: "Link", URL: "https://example.org/a?b=c"}}
	b, err := ExportLauncherProfile(p)
	if err != nil {
		t.Fatal(err)
	}
	other, err := ExportLauncherProfile(p)
	if err != nil || !bytes.Equal(b, other) {
		t.Fatal("nondeterministic export")
	}
	got, err := ParseLauncherProfile(b)
	if err != nil {
		t.Fatal(err)
	}
	got.Stacks[0].Members[0] = "changed"
	got.Items[1].Members[0] = "changed"
	*got.Widgets[0].Settings["label"].Text = "changed"
	if p.Stacks[0].Members[0] != "one" || p.Items[1].Members[0] != "app" || text != "local value" {
		t.Fatal("transfer aliased config")
	}
	for _, change := range []func(*LauncherProfile){func(p *LauncherProfile) { p.Items[3].URL = "file:///private/file" }, func(p *LauncherProfile) { p.Widgets[0].PackageID = "bad" }, func(p *LauncherProfile) { p.Widgets[0].Digest = "bad" }, func(p *LauncherProfile) { *p.Widgets[0].Settings["label"].Text = strings.Repeat("x", 1025) }} {
		q := CloneReplacementDock(ReplacementDockSettings{Profiles: []LauncherProfile{p}}).Profiles[0]
		change(&q)
		raw, _ := json.Marshal(launcherProfileEnvelope{Format: launcherProfileFormat, Version: 1, Profile: q})
		if _, err := ParseLauncherProfile(raw); err == nil {
			t.Fatal("invalid settings imported")
		}
	}
}
