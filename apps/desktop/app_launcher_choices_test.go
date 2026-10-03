package main

import (
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
)

type launcherChoicesSource struct {
	*fake.Fake
	apps             []domain.App
	calls            atomic.Int32
	entered, release chan struct{}
}

func (p *launcherChoicesSource) Apps() ([]domain.App, error) {
	p.calls.Add(1)
	if p.entered != nil {
		close(p.entered)
		<-p.release
	}
	return slices.Clone(p.apps), nil
}

func TestLauncherAppChoicesAreExplicitAndUseBundleIdentity(t *testing.T) {
	p := &launcherChoicesSource{Fake: fake.New(), apps: []domain.App{
		{ID: 1, Name: "Same name", BundleID: "com.example.second"},
		{ID: 2, Name: "Same name", BundleID: "com.example.first"},
		{ID: 3, Name: "Duplicate process", BundleID: "com.example.first"},
		{ID: 4, Name: "Alpha", BundleID: "com.example.alpha"},
		{ID: 5, Name: "No identity"},
		{ID: 6, Name: "Invalid identity", BundleID: "invalid/path"},
	}}
	a := newApp(p, config.Default(), "")
	t.Cleanup(a.stopCapture)
	if got := a.GetLauncherAppChoices(); len(got) != 0 || p.calls.Load() != 0 {
		t.Fatal("inventory ran outside an explicit preferences lifetime")
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.viewMu.Unlock()
	want := []LauncherAppChoice{{Name: "Alpha", BundleID: "com.example.alpha"}, {Name: "Same name", BundleID: "com.example.first"}, {Name: "Same name", BundleID: "com.example.second"}}
	if got := a.GetLauncherAppChoices(); !reflect.DeepEqual(got, want) {
		t.Fatalf("choices=%+v", got)
	}
	a.stopCapture()
	if got := a.GetLauncherAppChoices(); len(got) != 0 || p.calls.Load() != 1 {
		t.Fatal("shutdown admitted another inventory query")
	}
}

func TestLauncherAppChoicesDropAReadAfterPreferencesClose(t *testing.T) {
	p := &launcherChoicesSource{Fake: fake.New(), apps: []domain.App{{ID: 1, Name: "Fixture", BundleID: "com.example.fixture"}}, entered: make(chan struct{}), release: make(chan struct{})}
	a := newApp(p, config.Default(), "")
	t.Cleanup(a.stopCapture)
	a.viewMu.Lock()
	a.prefsOpen = true
	a.viewMu.Unlock()
	done := make(chan []LauncherAppChoice, 1)
	go func() { done <- a.GetLauncherAppChoices() }()
	select {
	case <-p.entered:
	case <-time.After(time.Second):
		t.Fatal("inventory did not start")
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.viewMu.Unlock()
	close(p.release)
	if got := <-done; len(got) != 0 {
		t.Fatal("retired preferences received an inventory")
	}
}
