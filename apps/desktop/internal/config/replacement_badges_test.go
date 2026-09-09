package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLauncherBadgesDefaultAndStrictDocuments(t *testing.T) {
	s := DefaultReplacementDock()
	if s.Profiles[0].ShowBadges {
		t.Fatal("badges enabled without opting in")
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"null", "1", `"true"`, "true,\"showBadges\":false"} {
		document := strings.Replace(string(raw), `"profiles":[{`, `"profiles":[{"showBadges":`+value+`,`, 1)
		if _, err := Load(strings.NewReader(`{"replacementDock":` + document + `}`)); err == nil {
			t.Fatal("accepted invalid badge setting", value)
		}
	}
	s.Profiles[0].ShowBadges = true
	raw, _ = json.Marshal(s)
	loaded, err := Load(strings.NewReader(`{"replacementDock":` + string(raw) + `}`))
	if err != nil || !loaded.ReplacementDock.Profiles[0].ShowBadges {
		t.Fatal("lost explicit badge preference", err)
	}
	legacy := legacyReplacement(t)
	legacy["profiles"].([]any)[0].(map[string]any)["showBadges"] = false
	if _, err := loadReplacementMap(t, legacy); err == nil {
		t.Fatal("version 1 accepted a later setting")
	}
}
