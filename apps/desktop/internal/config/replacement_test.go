package config

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestReplacementDefaultsAndStrictDecode(t *testing.T) {
	s, err := Load(strings.NewReader(`{}`))
	if err != nil || s.ReplacementDock.Enabled || len(s.ReplacementDock.Bindings) != 1 {
		t.Fatalf("defaults %+v %v", s.ReplacementDock, err)
	}
	d := DefaultReplacementDock()
	d.Bindings = []LauncherBinding{}
	raw, _ := json.Marshal(d)
	s, err = Load(strings.NewReader(`{"replacementDock":` + string(raw) + `}`))
	if err != nil || len(s.ReplacementDock.Bindings) != 0 {
		t.Fatal("empty bindings replaced")
	}
	for _, field := range []string{`"version":2`, `"version":1,"script":"anything"`, `"version":1,"version":1`} {
		if _, err := Load(strings.NewReader(`{"replacementDock":{` + field + `}}`)); err == nil {
			t.Fatalf("unsafe schema accepted: %s", field)
		}
	}
}

func TestReplacementGrantsAndCopies(t *testing.T) {
	d := DefaultReplacementDock()
	d.Profiles[0].Widgets[0].Enabled = true
	d.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	if err := ValidateReplacementDock(d); err != nil {
		t.Fatal(err)
	}
	copied := CloneReplacementDock(d)
	copied.Profiles[0].Widgets[0].Grants[0] = "process.execute"
	if d.Profiles[0].Widgets[0].Grants[0] != "clock.read" {
		t.Fatal("aliased grants")
	}
	if ValidateReplacementDock(copied) == nil {
		t.Fatal("unknown capability admitted")
	}
	copied = CloneReplacementDock(d)
	copied.Profiles[0].Widgets[0].Digest = "changed"
	if ValidateReplacementDock(copied) == nil {
		t.Fatal("old grants authorize changed package")
	}
	copied = CloneReplacementDock(d)
	copied.Bindings = append(copied.Bindings, copied.Bindings[0])
	if ValidateReplacementDock(copied) == nil {
		t.Fatal("duplicate binding admitted")
	}
}

func TestClockManifestDigestBindsImmutableDeclaration(t *testing.T) {
	if got := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(BuiltinClockManifest))); got != BuiltinClockDigest {
		t.Fatalf("manifest digest %s != %s", got, BuiltinClockDigest)
	}
	var declaration map[string]any
	if err := json.Unmarshal([]byte(BuiltinClockManifest), &declaration); err != nil {
		t.Fatal(err)
	}
	if declaration["packageID"] != BuiltinClockPackage {
		t.Fatal("wrong manifest package")
	}
}
