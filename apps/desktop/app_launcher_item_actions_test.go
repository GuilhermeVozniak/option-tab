package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type launcherActionReferenceFixture struct {
	launcherReferenceFixture
	entered   chan platform.LauncherItemAction
	release   chan struct{}
	mutations atomic.Int32
}

func (s *launcherActionReferenceFixture) PerformLauncherItem(_ context.Context, target platform.LauncherItemAction, guard func() error) error {
	s.entered <- target
	if s.release != nil {
		<-s.release
	}
	if err := guard(); err != nil {
		return err
	}
	s.mutations.Add(1)
	return nil
}

func configuredActionFixture(t *testing.T) (*App, *launcherActionReferenceFixture, launcher.Presentation, launcher.ConfiguredTarget) {
	t.Helper()
	a, _, q, _ := launcherIntegrationApp(t)
	source := &launcherActionReferenceFixture{launcherReferenceFixture: launcherReferenceFixture{records: map[string]platform.LauncherReference{}}, entered: make(chan platform.LauncherItemAction, 1)}
	a.wireLauncherItems(source, nil)

	target := launcher.ConfiguredTarget{Item: config.LauncherItem{ID: "fixture", Kind: "app", ReferenceID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Label: "Fixture"}, Reference: platform.LauncherReference{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "app", State: "ready", Revision: 4, Process: platform.ProcessIdentity{PID: 4242, StartSeconds: 123}, BundleID: "test.launcher.fixture", Label: "Owned fixture"}}
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].Items = []config.LauncherItem{target.Item}
	a.saveMu.Lock()
	err := a.saveSettingsLocked(s)
	a.saveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p := launcherIntegrationVisible(t, a, q)
	return a, source, p, target
}

func TestConfiguredLauncherActionInitialGuardRefusalDoesNotDispatch(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	want := errors.New("retired controller")
	err := a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", func() error { return want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	select {
	case <-source.entered:
		t.Fatal("dispatched retired target")
	default:
	}
}

func TestConfiguredLauncherActionExactTargetAndLateGuard(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	source.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "relaunch", func() error { return nil })
	}()
	captured := <-source.entered
	if captured.ReferenceID != target.Reference.ID || captured.ReferenceRevision != 4 || captured.Process != target.Reference.Process || captured.BundleID != target.Reference.BundleID || captured.PanelToken != 99 || captured.DisplayUUID != p.DisplayUUID || captured.Kind != "relaunch" {
		t.Fatal("wrong native target", captured)
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncDockSuspensionLocked()
	a.viewMu.Unlock()
	close(source.release)
	if err := <-done; err == nil || source.mutations.Load() != 0 {
		t.Fatal("retired host mutated", err)
	}
}

func TestConfiguredLauncherActionOldSourceRefused(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	source.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", func() error { return nil })
	}()
	<-source.entered
	a.wireLauncherItems(&launcherReferenceFixture{records: map[string]platform.LauncherReference{}}, nil)
	close(source.release)
	if err := <-done; err == nil || source.mutations.Load() != 0 {
		t.Fatal("old source mutated", err)
	}
}

func TestConfiguredLauncherActionSettingsPublicationAndReferenceChangeRefuse(t *testing.T) {
	for _, mode := range []string{"settings", "reference"} {
		t.Run(mode, func(t *testing.T) {
			a, source, p, target := configuredActionFixture(t)
			source.release = make(chan struct{})
			var retired atomic.Bool
			guard := func() error {
				if retired.Load() {
					return launcher.ErrRetired
				}
				return nil
			}
			done := make(chan error, 1)
			go func() { done <- a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", guard) }()
			<-source.entered
			if mode == "settings" {
				a.settingsMu.Lock()
				a.settings.ReplacementDock.Profiles[0].Items[0].ReferenceID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
				a.settingsMu.Unlock()
			} else {
				retired.Store(true)
			}
			close(source.release)
			if err := <-done; err == nil || source.mutations.Load() != 0 {
				t.Fatal("changed authority dispatched", err)
			}
		})
	}
}

func TestConfiguredLauncherActionCurrentScopeDispatchesOnce(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	if err := a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if source.mutations.Load() != 1 {
		t.Fatal("current target not dispatched")
	}
}

func TestConfiguredLauncherAdmittedActionSurvivesContentRevision(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	source.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", func() error { return nil })
	}()
	<-source.entered
	a.viewMu.Lock()
	a.launcher.hosts[p.Session].presentation.Revision++
	a.viewMu.Unlock()
	close(source.release)
	if err := <-done; err != nil || source.mutations.Load() != 1 {
		t.Fatal("content revision retired admitted action", err)
	}
}

func TestConfiguredLauncherAdmittedActionRefusesReplacementHost(t *testing.T) {
	a, source, p, target := configuredActionFixture(t)
	source.release = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- a.performConfiguredLauncherItem(context.Background(), p.Scope, target, "open", func() error { return nil })
	}()
	<-source.entered
	a.viewMu.Lock()
	old := a.launcher.hosts[p.Session]
	copyHost := *old
	a.launcher.hosts[p.Session] = &copyHost
	a.viewMu.Unlock()
	close(source.release)
	if err := <-done; err == nil || source.mutations.Load() != 0 {
		t.Fatal("replacement host accepted", err)
	}
}
