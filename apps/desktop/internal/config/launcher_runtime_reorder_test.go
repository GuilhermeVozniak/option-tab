package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLauncherRuntimeReorderStrict(t *testing.T) {
	s := DefaultReplacementDock()
	if s.Profiles[0].RuntimeReorder {
		t.Fatal("enabled by default")
	}
	raw, _ := json.Marshal(s)
	for _, value := range []string{"null", "1", "\"true\""} {
		text := strings.Replace(string(raw), `"profiles":[{`, `"profiles":[{"runtimeReorder":`+value+`,`, 1)
		if _, err := Load(strings.NewReader(`{"replacementDock":` + text + `}`)); err == nil {
			t.Fatal(value)
		}
	}
	legacy := legacyReplacement(t)
	legacy["profiles"].([]any)[0].(map[string]any)["runtimeReorder"] = false
	if _, err := loadReplacementMap(t, legacy); err == nil {
		t.Fatal("v1 admitted")
	}
	if LauncherItemsRevision(nil) != LauncherItemsRevision([]LauncherItem{}) {
		t.Fatal("nil hash differs")
	}
}
