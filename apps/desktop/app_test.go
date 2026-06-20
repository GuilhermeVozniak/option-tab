package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func TestGetSettings_ReturnsValidJSON(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	var s config.Settings
	if err := json.Unmarshal([]byte(a.GetSettings()), &s); err != nil {
		t.Fatalf("GetSettings not valid JSON: %v", err)
	}
	if len(s.Shortcuts) == 0 {
		t.Error("expected shortcuts in serialized settings")
	}
}

func TestSaveSettings_PersistsAndApplies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a := newApp(fake.New(), config.Default(), path)

	changed := config.Default()
	changed.Order = config.OrderAlphabetical
	b, _ := json.Marshal(changed)
	if err := a.SaveSettings(string(b)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if a.settings.Order != config.OrderAlphabetical {
		t.Errorf("settings not applied: order = %q", a.settings.Order)
	}
	reloaded, err := config.LoadFile(path)
	if err != nil || reloaded.Order != config.OrderAlphabetical {
		t.Errorf("settings not persisted: %+v err=%v", reloaded.Order, err)
	}
}

func TestSaveSettings_RejectsBadJSON(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	if err := a.SaveSettings("{not json"); err == nil {
		t.Error("expected error on malformed settings JSON")
	}
}

func TestRegisterHotkeys_DoesNotPanicWithFake(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	a.registerHotkeys()
	a.reRegisterHotkeys()
}
