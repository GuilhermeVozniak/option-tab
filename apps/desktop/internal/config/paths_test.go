package config

import (
	"path/filepath"
	"testing"
)

func TestLoadFile_MissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	s, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile(missing) error: %v", err)
	}
	if s.Version != CurrentVersion || len(s.Shortcuts) == 0 {
		t.Errorf("LoadFile(missing) should return defaults, got %+v", s)
	}
}

func TestSaveFile_ThenLoadFile_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "settings.json")
	want := Default()
	want.Order = OrderAlphabetical
	if err := SaveFile(path, want); err != nil {
		t.Fatalf("SaveFile error: %v", err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}
	if got.Order != OrderAlphabetical {
		t.Errorf("round-trip Order = %q, want alphabetical", got.Order)
	}
}

func TestDefaultPath_NonEmpty(t *testing.T) {
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath error: %v", err)
	}
	if p == "" {
		t.Error("DefaultPath returned empty path")
	}
	if filepath.Base(p) != "settings.json" {
		t.Errorf("DefaultPath base = %q, want settings.json", filepath.Base(p))
	}
}
