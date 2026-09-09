package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/widgets"
)

type appWidgetProvider struct {
	started  chan func(widgets.Sample)
	stopped  chan struct{}
	prepared chan struct{}
	release  chan struct{}
	before   func()
	actions  atomic.Int32
}

func newAppWidgetProvider() *appWidgetProvider {
	return &appWidgetProvider{started: make(chan func(widgets.Sample), 4), stopped: make(chan struct{}, 4), prepared: make(chan struct{}, 4)}
}

func (p *appWidgetProvider) Observe(ctx context.Context, _ []string, emit func(widgets.Sample)) error {
	p.started <- emit
	<-ctx.Done()
	p.stopped <- struct{}{}
	return ctx.Err()
}

func (p *appWidgetProvider) Perform(_ context.Context, _ widgets.ProviderAction, guard func() error) error {
	p.prepared <- struct{}{}
	if p.before != nil {
		p.before()
	}
	if p.release != nil {
		<-p.release
	}
	if err := guard(); err != nil {
		return err
	}
	p.actions.Add(1)
	return nil
}

func configuredWidget(id, name string, grants ...string) config.WidgetInstance {
	p, _ := widgets.Builtin("org.optiontab." + name)
	return config.WidgetInstance{ID: id, PackageID: p.Manifest().ID, Digest: p.Digest(), Enabled: true, Grants: grants}
}

func saveWidgetFixture(t *testing.T, a *App, instances []config.WidgetInstance, stacks ...config.WidgetStack) {
	t.Helper()
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].Widgets = instances
	s.ReplacementDock.Profiles[0].Stacks = stacks
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.SaveSettings(string(raw)); err != nil {
		t.Fatal(err)
	}
}

func appWidgetSample() widgets.Sample {
	name := "Fixture output"
	return widgets.Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Fields: map[string]widgets.Value{"outputName": {Text: &name}}, Actions: map[string]widgets.ActionSpec{"selectOutput": {Enabled: true, Options: []widgets.ProviderOption{{ID: "fixture-uid", Label: "Fixture output"}}}}}
}

func findWidgetAction(n widgets.RenderNode) string {
	if n.ActionToken != "" {
		return n.ActionToken
	}
	for _, child := range n.Children {
		if token := findWidgetAction(child); token != "" {
			return token
		}
	}
	return ""
}

func TestLauncherWidgetsGrantVisibilityAndRecovery(t *testing.T) {
	a, _, q, panel := launcherIntegrationApp(t)
	source := newAppWidgetProvider()
	a.wireWidgets(widgets.Providers{Audio: source})
	audio := configuredWidget("output", "audio")
	saveWidgetFixture(t, a, []config.WidgetInstance{audio})
	p := launcherIntegrationVisible(t, a, q)
	launcherEventually(t, func() bool {
		s := a.GetLauncherWidgets(p.Session)
		return len(s.Slots) == 1 && s.Slots[0].State != nil && s.Slots[0].State.Status == "grantRequired"
	})
	select {
	case <-source.started:
		t.Fatal("ungranted widget started provider")
	default:
	}
	audio.Grants = []string{"audio.status.read", "audio.output.select"}
	saveWidgetFixture(t, a, []config.WidgetInstance{audio})
	p = launcherIntegrationVisible(t, a, q)
	var emit func(widgets.Sample)
	select {
	case emit = <-source.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	emit(appWidgetSample())
	var instance widgets.InstanceState
	var token string
	launcherEventually(t, func() bool {
		s := a.GetLauncherWidgets(p.Session)
		if len(s.Slots) != 1 || s.Slots[0].State == nil {
			return false
		}
		instance = *s.Slots[0].State
		token = findWidgetAction(instance.Root)
		return token != ""
	})
	options, err := a.GetWidgetActionOptions(instance.Lease, token)
	if err != nil || len(options.Options) != 1 {
		t.Fatalf("options: %+v %v", options, err)
	}
	// Native visibility can disappear before the environment callback changes
	// logical state. Final dispatch must query the current native parent too.
	source.before = func() { panel.unavailable.Store(true) }
	if err := a.PerformWidgetAction(instance.Lease, token, options.Options[0].Token, nil); err == nil || source.actions.Load() != 0 {
		t.Fatal("native-hidden parent admitted widget control")
	}
	<-source.prepared
	source.before = nil
	panel.unavailable.Store(false)
	source.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- a.PerformWidgetAction(instance.Lease, token, options.Options[0].Token, nil) }()
	<-source.prepared
	if err := a.UseNativeDock(); err != nil {
		t.Fatal(err)
	}
	close(source.release)
	if err := <-done; err == nil {
		t.Fatal("retired widget control succeeded")
	}
	if source.actions.Load() != 0 || a.GetLauncherWidgets(p.Session).Visible {
		t.Fatal("recovery retained widget action/view")
	}
	select {
	case <-source.stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("recovery did not release widget provider")
	}
	if _, err := a.GetWidgetActionOptions(instance.Lease, token); !errors.Is(err, widgets.ErrRetired) {
		t.Fatalf("stale options: %v", err)
	}
}

func TestLauncherWidgetStackOnlySubscribesSelectedMember(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	source := newAppWidgetProvider()
	a.wireWidgets(widgets.Providers{Audio: source})
	saveWidgetFixture(t, a, []config.WidgetInstance{configuredWidget("time", "clock", "clock.read"), configuredWidget("output", "audio", "audio.status.read")}, config.WidgetStack{ID: "status", Name: "Status", Members: []string{"time", "output"}, ActiveID: "time"})
	p := launcherIntegrationVisible(t, a, q)
	launcherEventually(t, func() bool {
		s := a.GetLauncherWidgets(p.Session)
		return len(s.Slots) == 1 && s.Slots[0].SelectedID == "time" && s.Slots[0].State != nil
	})
	select {
	case <-source.started:
		t.Fatal("hidden stack member subscribed")
	default:
	}
	if err := a.SelectLauncherWidget(p.Epoch, p.DisplayUUID, p.Session, p.ProfileID, "status", "output"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.started:
	case <-time.After(3 * time.Second):
		t.Fatal("selected stack provider not started")
	}
	if err := a.SelectLauncherWidget(p.Epoch, p.DisplayUUID, p.Session, p.ProfileID, "status", "time"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-source.stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("hidden stack provider not stopped")
	}
	if err := a.SelectLauncherWidget(p.Epoch, p.DisplayUUID, p.Session, p.ProfileID, "status", "unknown"); err == nil {
		t.Fatal("nonmember selected")
	}
	if err := a.SelectLauncherWidget(p.Epoch+1, p.DisplayUUID, p.Session, p.ProfileID, "status", "output"); !errors.Is(err, widgets.ErrRetired) {
		t.Fatalf("stale parent accepted: %v", err)
	}
}

func TestLauncherLegacyClockUsesOnlyVerifiedBuiltinAndCatalogIsPreferencesOnly(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	if len(a.GetWidgetCatalog()) != 0 {
		t.Fatal("catalog outside preferences")
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.viewMu.Unlock()
	catalog := a.GetWidgetCatalog()
	if len(catalog) != 4 || !catalog[0].Builtin {
		t.Fatalf("builtin catalog: %+v", catalog)
	}
	a.viewMu.Lock()
	a.prefsOpen = false
	a.viewMu.Unlock()
	clock := a.settingsSnapshot().ReplacementDock.Profiles[0].Widgets[0]
	clock.Enabled, clock.Grants = true, []string{"clock.read"}
	saveWidgetFixture(t, a, []config.WidgetInstance{clock})
	p := launcherIntegrationVisible(t, a, q)
	verified, _ := widgets.Builtin(config.BuiltinClockPackage)
	launcherEventually(t, func() bool {
		s := a.GetLauncherWidgets(p.Session)
		return len(s.Slots) == 1 && s.Slots[0].State != nil && s.Slots[0].State.Lease.Digest == verified.Digest() && s.Slots[0].State.Status == "ready"
	})
}
