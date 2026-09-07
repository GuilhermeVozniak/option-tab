package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func launcherItemFixture(id, kind string) LauncherItem {
	x := LauncherItem{ID: id, Kind: kind}
	switch kind {
	case "app", "folder", "file":
		x.Label, x.ReferenceID = "Item", "ref-"+id
	case "link":
		x.Label, x.URL = "Site", "https://example.com/path"
	}
	if kind == "folder" {
		x.FolderView = "list"
	}
	return x
}

func TestReplacementItemsValidateCloneRoundTrip(t *testing.T) {
	s := DefaultReplacementDock()
	s.Profiles[0].Items = []LauncherItem{launcherItemFixture("app", "app"), launcherItemFixture("folder", "folder"), launcherItemFixture("file", "file"), launcherItemFixture("link", "link"), {ID: "gap", Kind: "spacer"}, {ID: "line", Kind: "separator"}, {ID: "group", Kind: "group", Label: "Work", Members: []string{"app"}}}
	s.Profiles[0].Items[0].IconID = strings.Repeat("a", 64)
	if err := ValidateReplacementDock(s); err != nil {
		t.Fatal(err)
	}
	c := CloneReplacementDock(s)
	c.Profiles[0].Items[6].Members[0] = "changed"
	if s.Profiles[0].Items[6].Members[0] != "app" {
		t.Fatal("members aliased")
	}
	raw, _ := json.Marshal(s)
	loaded, err := Load(strings.NewReader(`{"replacementDock":` + string(raw) + `}`))
	if err != nil || len(loaded.ReplacementDock.Profiles[0].Items) != 7 {
		t.Fatal(err)
	}
}

func TestReplacementItemsRejectMalformed(t *testing.T) {
	base := DefaultReplacementDock()
	base.Profiles[0].Items = []LauncherItem{launcherItemFixture("a", "app"), launcherItemFixture("b", "app"), {ID: "g", Kind: "group", Label: "G", Members: []string{"a"}}}
	cases := []func(*LauncherProfile){func(p *LauncherProfile) {
		p.Items[2].Members = []string{"a", "b"}
		p.Items = append(p.Items, LauncherItem{ID: "g2", Kind: "group", Label: "G2", Members: []string{"b"}})
	}, func(p *LauncherProfile) { p.Items[2].Members = []string{"g"} }, func(p *LauncherProfile) { p.Items[2].Members = nil }, func(p *LauncherProfile) { p.Items[0].URL = "https://example.com" }, func(p *LauncherProfile) {
		p.Items = append(p.Items, LauncherItem{ID: "bad", Kind: "link", Label: "Bad", URL: "file:///tmp/a"})
	}, func(p *LauncherProfile) {
		p.Items = append(p.Items, LauncherItem{ID: "bad", Kind: "link", Label: "Bad", URL: "https://u:p@example.com"})
	}}
	for i, f := range cases {
		s := CloneReplacementDock(base)
		f(&s.Profiles[0])
		if ValidateReplacementDock(s) == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestReplacementItemsRejectBoundsUnknownLegacy(t *testing.T) {
	s := DefaultReplacementDock()
	for i := 0; i < 17; i++ {
		s.Profiles[0].Items = append(s.Profiles[0].Items, LauncherItem{ID: "gap" + string(rune('a'+i)), Kind: "spacer"})
	}
	if ValidateReplacementDock(s) == nil {
		t.Fatal("17 accepted")
	}
	raw, _ := json.Marshal(DefaultReplacementDock())
	text := strings.Replace(string(raw), `"profiles":[{`, `"profiles":[{"items":[{"id":"x","kind":"spacer","unknown":true}],`, 1)
	if _, err := Load(strings.NewReader(`{"replacementDock":` + text + `}`)); err == nil {
		t.Fatal("unknown accepted")
	}
	legacy := legacyReplacement(t)
	legacy["profiles"].([]any)[0].(map[string]any)["items"] = []any{}
	if _, err := loadReplacementMap(t, legacy); err == nil {
		t.Fatal("legacy items accepted")
	}
}

func TestLauncherURLMatchesNativeAdmission(t *testing.T) {
	for _, raw := range []string{"https://:80", "https://example.com/#\u0085"} {
		if validLauncherURL(raw) {
			t.Errorf("accepted native-refused URL %q", raw)
		}
	}
	for _, raw := range []string{"https://example.com/path?q=one#two", "http://[::1]:80/", "https://example.com/%C2%85"} {
		if !validLauncherURL(raw) {
			t.Errorf("rejected canonical URL %q", raw)
		}
	}
}
