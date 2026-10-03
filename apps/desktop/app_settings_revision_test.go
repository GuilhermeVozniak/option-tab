package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func TestSettingsRevisionSnapshotDoesNotWaitForNativeWriter(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	initial := a.GetSettingsState()
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	done := make(chan SettingsState, 1)
	go func() { done <- a.GetSettingsState() }()
	select {
	case state := <-done:
		if state != initial {
			t.Fatal("snapshot changed during uncommitted writer", state)
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot waited for writer; native menu callback could deadlock")
	}
}

func TestSettingsRevisionCASPreservesNewerSettings(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	initial := a.GetSettingsState()
	if initial.Revision == 0 || initial.JSON != a.GetSettings() {
		t.Fatal("incoherent initial snapshot")
	}
	next := a.settingsSnapshot()
	next.Order = config.OrderAlphabetical
	b, _ := json.Marshal(next)
	committed, err := a.SaveSettingsAtRevision(string(b), initial.Revision)
	if err != nil || committed.Revision != initial.Revision+1 || committed.JSON != a.GetSettings() {
		t.Fatal("canonical commit", committed, err)
	}
	if _, err = a.SaveSettingsAtRevision(initial.JSON, initial.Revision); err == nil {
		t.Fatal("stale save accepted")
	}
	if a.GetSettingsState() != committed {
		t.Fatal("stale save overwrote settings")
	}
	if err = a.SaveSettings(initial.JSON); err != nil {
		t.Fatal(err)
	}
	if a.GetSettingsState().Revision != committed.Revision+1 {
		t.Fatal("legacy writer failed to retire revision")
	}
}

func TestSettingsRevisionFailedPersistenceDoesNotAdvance(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newApp(fake.New(), config.Default(), filepath.Join(parent, "settings.json"))
	t.Cleanup(a.stopCapture)
	initial := a.GetSettingsState()
	if _, err := a.SaveSettingsAtRevision(initial.JSON, initial.Revision); err == nil {
		t.Fatal("disk failure missing")
	}
	if a.GetSettingsState() != initial {
		t.Fatal("failed save published revision")
	}
}

func TestSettingsRevisionConcurrentSavesHaveOneWinner(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	initial := a.GetSettingsState()
	start, results := make(chan struct{}), make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := a.SaveSettingsAtRevision(initial.JSON, initial.Revision)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	wins := 0
	for err := range results {
		if err == nil {
			wins++
		}
	}
	if wins != 1 || a.GetSettingsState().Revision != initial.Revision+1 {
		t.Fatal("CAS did not serialize competing writers", wins)
	}
}

func TestSettingsRevisionRefreshesRetainedPreferences(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	t.Cleanup(a.stopCapture)
	var events []string
	var snapshots []prefsSettingsSnapshot
	var generations []uint64
	a.eventSink = func(name string, value any) {
		if name == "prefs:settings-loading" {
			events = append(events, name)
			generations = append(generations, value.(prefsSettingsLoading).Generation)
		}
		if name == "prefs:settings" {
			events = append(events, name)
			snapshots = append(snapshots, value.(prefsSettingsSnapshot))
		}
	}
	a.OpenPreferences()
	a.ClosePreferences()
	initial := a.GetSettingsState()
	if err := a.SaveSettings(initial.JSON); err != nil {
		t.Fatal(err)
	}
	a.OpenPreferences()
	if len(events) != 4 || events[0] != "prefs:settings-loading" || events[1] != "prefs:settings" || events[2] != "prefs:settings-loading" || events[3] != "prefs:settings" {
		t.Fatal("refresh order", events)
	}
	if len(snapshots) != 2 || snapshots[1].Revision != initial.Revision+1 || snapshots[1].SettingsState != a.GetSettingsState() {
		t.Fatal("retained preferences received stale settings", snapshots)
	}
	if len(generations) != 2 || generations[0] == 0 || generations[1] <= generations[0] || snapshots[0].Generation != generations[0] || snapshots[1].Generation != generations[1] {
		t.Fatal("refresh events do not have distinct matching generations", generations, snapshots)
	}
	encoded, err := json.Marshal(snapshots[1])
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil || len(fields) != 3 || fields["generation"] == nil || fields["revision"] == nil || fields["json"] == nil {
		t.Fatal("canonical event is not the flat frontend envelope", string(encoded), err)
	}
}
