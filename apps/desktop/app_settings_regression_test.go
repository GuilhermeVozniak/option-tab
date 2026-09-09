package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func TestSettingsSnapshotDoesNotShareActionBindings(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	snapshot := a.settingsSnapshot()
	original := snapshot.Behavior.ActionBindings["KeyW"]
	delete(snapshot.Behavior.ActionBindings, "KeyW")
	if got := a.settingsSnapshot().Behavior.ActionBindings["KeyW"]; got != original {
		t.Fatalf("mutating a snapshot changed shared bindings: got %q, want %q", got, original)
	}
}

func TestSettingsSnapshotClonesAppModeActionBindings(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	defer a.stopCapture()
	snapshot := a.settingsSnapshot()
	snapshot.AppSwitcher.Behavior.ActionBindings["KeyW"] = config.ActionQuit
	if got := a.settingsSnapshot().AppSwitcher.Behavior.ActionBindings["KeyW"]; got != config.ActionClose {
		t.Fatalf("app binding escaped snapshot: %q", got)
	}
}

func TestSaveSettingsFailureKeepsPreviousSettings(t *testing.T) {
	s := config.Default()
	a := newApp(fake.New(), s, t.TempDir())
	next := config.Default()
	next.Order = config.OrderAlphabetical
	b, _ := json.Marshal(next)
	if err := a.SaveSettings(string(b)); err == nil {
		t.Fatal("save must report the failed disk write")
	}
	var got config.Settings
	if err := json.Unmarshal([]byte(a.GetSettings()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Order != s.Order {
		t.Fatal("failed save changed active settings")
	}
}

func TestSettingsConcurrentReadSavePause(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	b, _ := json.Marshal(config.Default())
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for range 50 {
			if err := a.SaveSettings(string(b)); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range 50 {
			var s config.Settings
			if err := json.Unmarshal([]byte(a.GetSettings()), &s); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 50 {
			a.SetPaused(i%2 == 0)
		}
	}()
	wg.Wait()
}

func TestStartupSettingsCorruptFileKeepsDefaultsAndOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	broken := []byte("{broken")
	if err := os.WriteFile(path, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	s := loadStartupSettings(path)
	if len(s.Shortcuts) == 0 || !s.Behavior.ShowMenubarIcon {
		t.Fatal("corrupt config must leave app accessible")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(broken) {
		t.Fatal("startup must preserve corrupt config for recovery")
	}
}

type failingLoginPlatform struct{ *fake.Fake }

func (p *failingLoginPlatform) SetEnabled(bool) error { return os.ErrPermission }

func TestSaveSettingsLoginFailureDoesNotPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a := newApp(&failingLoginPlatform{fake.New()}, config.Default(), path)
	next := config.Default()
	next.Behavior.StartAtLogin = true
	b, _ := json.Marshal(next)
	if err := a.SaveSettings(string(b)); err == nil {
		t.Fatal("login failure must reach caller")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed login update must not persist")
	}
	if a.settingsSnapshot().Behavior.StartAtLogin {
		t.Fatal("failed login update was applied")
	}
}

func TestSaveSettingsRestoresLoginItemOnDiskFailure(t *testing.T) {
	p := fake.New()
	a := newApp(p, config.Default(), t.TempDir())
	next := config.Default()
	next.Behavior.StartAtLogin = true
	b, _ := json.Marshal(next)
	if err := a.SaveSettings(string(b)); err == nil {
		t.Fatal("expected disk error")
	}
	if p.Enabled() {
		t.Fatal("login item was not restored after failed save")
	}
}
