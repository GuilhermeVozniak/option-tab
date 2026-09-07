package config

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"option-tab/internal/widgets"
)

func widgetFixture(id, name string) WidgetInstance {
	p, ok := widgets.Builtin("org.optiontab." + name)
	if !ok {
		panic("missing builtin")
	}
	return WidgetInstance{ID: id, PackageID: p.Manifest().ID, Digest: p.Digest(), Grants: []string{}}
}

func TestReplacementWidgetSettingsAndStackCopies(t *testing.T) {
	s := DefaultReplacementDock()
	clock := widgetFixture("time", "clock")
	timezone := "Europe/Rome"
	clock.Settings = map[string]widgets.Value{"timezone": {Text: &timezone}}
	s.Profiles[0].Widgets = []WidgetInstance{clock, widgetFixture("power", "battery")}
	s.Profiles[0].Stacks = []WidgetStack{{ID: "status", Name: "Status", Members: []string{"time", "power"}, ActiveID: "time"}}
	if err := ValidateReplacementDock(s); err != nil {
		t.Fatal(err)
	}
	copy := CloneReplacementDock(s)
	*copy.Profiles[0].Widgets[0].Settings["timezone"].Text = "UTC"
	copy.Profiles[0].Stacks[0].Members[0] = "power"
	if *s.Profiles[0].Widgets[0].Settings["timezone"].Text != "Europe/Rome" || s.Profiles[0].Stacks[0].Members[0] != "time" {
		t.Fatal("widget settings/stack snapshot alias")
	}
	raw, _ := json.Marshal(s)
	loaded, err := Load(strings.NewReader(`{"replacementDock":` + string(raw) + `}`))
	if err != nil || loaded.ReplacementDock.Profiles[0].Stacks[0].ActiveID != "time" {
		t.Fatalf("stack round trip: %v", err)
	}
}

func TestReplacementWidgetRejectsUnknownSettingsAndAuthority(t *testing.T) {
	for _, change := range []func(*WidgetInstance){
		func(w *WidgetInstance) { w.Grants = []string{"process.execute"} },
		func(w *WidgetInstance) { w.Grants = []string{"clock.read", "clock.read"} },
		func(w *WidgetInstance) { w.Grants = []string{"audio.output.select"} },
		func(w *WidgetInstance) { w.PackageID = "../clock" },
		func(w *WidgetInstance) { w.Digest = "sha256:unverified" },
		func(w *WidgetInstance) {
			v := "not/a/zone"
			w.Settings = map[string]widgets.Value{"timezone": {Text: &v}}
		},
		func(w *WidgetInstance) { v := "run code"; w.Settings = map[string]widgets.Value{"format": {Text: &v}} },
		func(w *WidgetInstance) { v := true; w.Settings = map[string]widgets.Value{"extra": {Boolean: &v}} },
	} {
		s := DefaultReplacementDock()
		w := widgetFixture("clock", "clock")
		change(&w)
		s.Profiles[0].Widgets = []WidgetInstance{w}
		if ValidateReplacementDock(s) == nil {
			t.Fatalf("accepted invalid widget: %+v", w)
		}
	}
	// Uninstalled community references may be restored, but only as bounded
	// declarative configuration; runtime still requires the exact verified package.
	s := DefaultReplacementDock()
	s.Profiles[0].Widgets = []WidgetInstance{{ID: "community", PackageID: "com.example.status", Digest: strings.Repeat("a", 64), Grants: []string{"battery.read"}}}
	if err := ValidateReplacementDock(s); err != nil {
		t.Fatal(err)
	}
	badNumber := math.NaN()
	s.Profiles[0].Widgets[0].Settings = map[string]widgets.Value{"amount": {Number: &badNumber}}
	if ValidateReplacementDock(s) == nil {
		t.Fatal("non-finite community setting accepted")
	}
}

func TestReplacementWidgetStackReferencesAndVisibleLimit(t *testing.T) {
	base := DefaultReplacementDock()
	base.Profiles[0].Widgets = []WidgetInstance{widgetFixture("clock", "clock"), widgetFixture("power", "battery"), widgetFixture("network", "network"), widgetFixture("audio", "audio"), widgetFixture("clock2", "clock")}
	if ValidateReplacementDock(base) == nil {
		t.Fatal("more than four standalone slots accepted")
	}
	base.Profiles[0].Stacks = []WidgetStack{{ID: "status", Name: "Status", Members: []string{"clock", "power"}, ActiveID: "clock"}}
	if err := ValidateReplacementDock(base); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*LauncherProfile){
		func(p *LauncherProfile) { p.Stacks[0].Members = []string{"clock", "missing"} },
		func(p *LauncherProfile) { p.Stacks[0].Members = []string{"clock", "clock"} },
		func(p *LauncherProfile) { p.Stacks[0].ActiveID = "network" },
		func(p *LauncherProfile) {
			p.Stacks = append(p.Stacks, WidgetStack{ID: "duplicate", Name: "Duplicate", Members: []string{"power", "network"}, ActiveID: "power"})
		},
	} {
		s := CloneReplacementDock(base)
		change(&s.Profiles[0])
		if ValidateReplacementDock(s) == nil {
			t.Fatalf("accepted invalid stack: %+v", s.Profiles[0].Stacks)
		}
	}
}

func TestLegacyClockOnlyKeepsItsOriginalCapability(t *testing.T) {
	s := DefaultReplacementDock()
	s.Profiles[0].Widgets[0].Enabled = true
	s.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	if err := ValidateReplacementDock(s); err != nil {
		t.Fatal(err)
	}
	s.Profiles[0].Widgets[0].Grants = []string{"battery.read"}
	if ValidateReplacementDock(s) == nil {
		t.Fatal("legacy clock gained new authority")
	}
}
