package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type launcherBadgeFixture struct {
	calls   atomic.Int32
	entered chan []platform.LauncherBadgeTarget
	release chan struct{}
	updates chan platform.LauncherBadgeSnapshot
	stopped chan struct{}
}

func (f *launcherBadgeFixture) ObserveLauncherBadges(ctx context.Context, targets []platform.LauncherBadgeTarget, emit func(platform.LauncherBadgeSnapshot)) error {
	if f.stopped != nil {
		defer close(f.stopped)
	}
	f.calls.Add(1)
	f.entered <- targets
	if f.release != nil {
		<-f.release
	}
	count := uint32(7)
	entries := make([]platform.LauncherBadgeEntry, len(targets))
	for i, v := range targets {
		entries[i] = platform.LauncherBadgeEntry{ItemKey: v.ItemKey, TargetRevision: v.TargetRevision, State: platform.BadgeKnown, Kind: platform.BadgeCount, Count: &count}
	}
	emit(platform.LauncherBadgeSnapshot{Generation: 1, Sequence: 1, Status: platform.BadgeReady, Entries: entries})
	for {
		select {
		case snapshot := <-f.updates:
			emit(snapshot)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestLauncherBadgesDisabledDoesNotObserve(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 2)}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	state := a.GetLauncherBadges(p.Session)
	if state.Visible || source.calls.Load() != 0 {
		t.Fatal("disabled badges observed", state)
	}
}

func (p *launcherIntegrationPlatform) ResolveRunningLauncherBadgeTarget(ctx context.Context, process platform.ProcessIdentity, bundle string) (platform.LauncherBadgeTarget, error) {
	if err := ctx.Err(); err != nil {
		return platform.LauncherBadgeTarget{}, err
	}
	return platform.LauncherBadgeTarget{Process: process, BundleID: bundle, CanonicalAppPath: "/fixture/Owned.app"}, nil
}

func enableIntegrationBadges(t *testing.T, a *App) {
	t.Helper()
	a.settingsMu.Lock()
	a.settings.ReplacementDock.Profiles[0].ShowBadges = true
	s := a.settings.ReplacementDock
	a.settingsMu.Unlock()
	a.launcher.configuration = config.CloneReplacementDock(s)
	if err := a.launcher.core.Configure(s); err != nil {
		t.Fatal(err)
	}
}

func TestLauncherBadgesPublishCopyClockAndDisable(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	enableIntegrationBadges(t, a)
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 4)}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	v := a.GetLauncherBadges(p.Session)
	owner := v.Owner
	*v.Entries[0].Count = 99
	if *a.GetLauncherBadges(p.Session).Entries[0].Count != 7 {
		t.Fatal("mutable badge DTO")
	}
	// A content-only publication must not replace the native source owner.
	a.publishLauncher(a.launcher.core.Snapshot())
	if a.GetLauncherBadges(p.Session).Owner != owner || source.calls.Load() != 1 {
		t.Fatal("content publication restarted source")
	}
	a.settingsMu.Lock()
	a.settings.ReplacementDock.Profiles[0].ShowBadges = false
	a.settingsMu.Unlock()
	if state := a.GetLauncherBadges(p.Session); state.Visible || len(state.Entries) != 0 {
		t.Fatal("disabled kept badges", state)
	}
}

func TestLauncherBadgesBlockedRetiredReadCannotPublish(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	enableIntegrationBadges(t, a)
	release := make(chan struct{})
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 4), release: release}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	<-source.entered
	a.viewMu.Lock()
	a.stopLauncherAdmissionLocked()
	a.viewMu.Unlock()
	close(release)
	state := a.GetLauncherBadges(p.Session)
	if state.Visible || len(state.Entries) != 0 {
		t.Fatal("retired callback published", state)
	}
	a.stopCapture()
	select {
	case <-a.launcher.badges.worker.done:
	case <-time.After(time.Second):
		t.Fatal("badge source did not join")
	}
}

type badgeTwoDisplayPlatform struct{ *launcherIntegrationPlatform }

func (p *badgeTwoDisplayPlatform) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	return p.launcherIntegrationPlatform.ObserveLauncherEnvironment(ctx, func(e platform.LauncherEnvironment) {
		second := e.Displays[0]
		second.UUID = "22222222-2222-2222-2222-222222222222"
		second.Main = false
		second.Frame.X = 1000
		second.UsableFrame.X = 1000
		second.SpaceID = 2
		e.Displays = append(e.Displays, second)
		emit(e)
	})
}

type badgeDisplayPanel struct {
	launcherIntegrationPanel
	display string
}

func (p *badgeDisplayPanel) ValidateLauncherPanel(ctx context.Context, display string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if display != p.display || p.unavailable.Load() {
		return platform.ErrDockPanelHostClosed
	}
	return nil
}

type badgePanelHost struct{ panel *badgeDisplayPanel }

func (h badgePanelHost) CreateLauncherPanel(unsafe.Pointer, string) (platform.LauncherPanel, error) {
	return h.panel, nil
}

func TestLauncherBadgesTwoHostsShareRunningTarget(t *testing.T) {
	backend := &badgeTwoDisplayPlatform{&launcherIntegrationPlatform{Fake: fake.New(), entered: make(chan platform.LauncherAppTarget, 4), stopped: make(chan struct{}, 8)}}
	s := config.Default()
	s.ReplacementDock.Enabled = true
	s.ReplacementDock.Profiles[0].ShowBadges = true
	s.ReplacementDock.Bindings = append(s.ReplacementDock.Bindings, config.LauncherBinding{ID: "second", Target: "display", DisplayUUID: "22222222-2222-2222-2222-222222222222", ProfileID: "default"})
	a := newApp(backend, s, "")
	t.Cleanup(a.stopCapture)
	q := make(chan func(), 64)
	a.launcherFactory = func(session uint64, display string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		panel := &badgeDisplayPanel{launcherIntegrationPanel: launcherIntegrationPanel{token: session + 100}, display: display}
		host := &dockFakeWindow{}
		host.native = unsafe.Pointer(new(int))
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return host }, launcherPanelHostAdapter{source: badgePanelHost{panel}, uuid: display, style: style})
		d.onFailure = failed
		return d
	}
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 8)}
	a.launcher.badges.source = source
	a.startLauncher()
	launcherEventually(t, func() bool {
		for {
			select {
			case f := <-q:
				f()
			default:
				goto drained
			}
		}
	drained:
		presentations := a.launcher.core.Snapshot().Presentations
		if len(presentations) != 2 {
			return false
		}
		for _, p := range presentations {
			v := a.GetLauncherBadges(p.Session)
			if len(v.Entries) != 1 || v.Entries[0].Count == nil {
				return false
			}
		}
		return true
	})
	for {
		select {
		case targets := <-source.entered:
			if len(targets) != 1 {
				t.Fatal("duplicate native target", targets)
			}
		default:
			return
		}
	}
}

type badgeReferenceFixture struct {
	launcherReferenceFixture
	entered, release chan struct{}
}

func (f *badgeReferenceFixture) ResolveLauncherBadgeTarget(ctx context.Context, expected platform.LauncherReference) (platform.LauncherBadgeTarget, error) {
	close(f.entered)
	<-f.release
	return platform.LauncherBadgeTarget{BundleID: expected.BundleID, CanonicalAppPath: "/fixture/Selected.app"}, nil
}

func TestLauncherBadgesReferenceReadJoinsRetiredManager(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &badgeReferenceFixture{entered: make(chan struct{}), release: make(chan struct{})}
	m := &appLauncherItems{refs: f, ctx: ctx, cancel: cancel}
	a := &App{launcherItems: m}
	result := make(chan error, 1)
	go func() {
		_, err := a.badgeTarget(context.Background(), launcherBadgePlan{refs: m}, launcher.BadgeItem{Reference: platform.LauncherReference{ID: "id", Kind: "app", State: "ready", Revision: 3, BundleID: "fixture"}})
		result <- err
	}()
	<-f.entered
	a.viewMu.Lock()
	m.closed = true
	cancel()
	a.viewMu.Unlock()
	joined := make(chan struct{})
	go func() { m.wg.Wait(); close(joined) }()
	select {
	case <-joined:
		t.Fatal("manager joined before reference read")
	default:
	}
	close(f.release)
	if err := <-result; err == nil {
		t.Fatal("retired reference published")
	}
	<-joined
}

func TestLauncherBadgesUnavailableAndPhysicalLossClearCount(t *testing.T) {
	a, _, q, panel := launcherIntegrationApp(t)
	enableIntegrationBadges(t, a)
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 4), updates: make(chan platform.LauncherBadgeSnapshot), stopped: make(chan struct{})}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	targets := <-source.entered
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	snapshot := platform.LauncherBadgeSnapshot{Generation: 1, Sequence: 2, Status: platform.BadgeSourceUnavailable, Entries: []platform.LauncherBadgeEntry{{ItemKey: targets[0].ItemKey, TargetRevision: targets[0].TargetRevision, State: platform.BadgeUnavailable}}}
	source.updates <- snapshot
	launcherEventually(t, func() bool { return a.GetLauncherBadges(p.Session).Entries[0].Count == nil })
	n := uint32(9)
	snapshot.Sequence++
	snapshot.Status = platform.BadgeReady
	snapshot.Entries[0].State = platform.BadgeKnown
	snapshot.Entries[0].Kind = platform.BadgeCount
	snapshot.Entries[0].Count = &n
	source.updates <- snapshot
	launcherEventually(t, func() bool { return a.GetLauncherBadges(p.Session).Entries[0].Count != nil })
	panel.unavailable.Store(true)
	snapshot.Sequence++
	source.updates <- snapshot
	launcherEventually(t, func() bool { return a.GetLauncherBadges(p.Session).Entries[0].Count == nil })
	select {
	case <-source.stopped:
	case <-time.After(time.Second):
		t.Fatal("physical loss did not cancel observer")
	}
}

type badgeClockEnvironment struct {
	source platform.LauncherEnvironmentSource
	now    func() time.Time
}

func (e badgeClockEnvironment) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	return e.source.ObserveLauncherEnvironment(ctx, func(v platform.LauncherEnvironment) { v.ObservedAt = e.now(); emit(v) })
}

func TestLauncherBadgesActualClockTickKeepsObserver(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	var tick atomic.Int64
	base := time.Now().Truncate(time.Minute).Add(59500 * time.Millisecond)
	now := func() time.Time { return base.Add(time.Duration(tick.Load()) * time.Second) }
	a.launcher.core = launcher.New(launcher.Deps{Environment: badgeClockEnvironment{backend, now}, Applications: backend, Identities: backend, View: appLauncherView{a}, Now: now})
	a.settingsMu.Lock()
	a.settings.ReplacementDock.Profiles[0].Widgets[0].Enabled = true
	a.settings.ReplacementDock.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	a.settingsMu.Unlock()
	enableIntegrationBadges(t, a)
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 4)}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	before := a.GetLauncherBadges(p.Session)
	tick.Store(1)
	launcherEventually(t, func() bool { return a.GetLauncherBadges(p.Session).PresentationRevision > before.PresentationRevision })
	after := a.GetLauncherBadges(p.Session)
	if after.Owner != before.Owner || source.calls.Load() != 1 || after.Entries[0].Count == nil {
		t.Fatal("clock retired observer", before, after)
	}
}

type badgeRecoveringPlatform struct {
	*launcherIntegrationPlatform
	unavailable atomic.Bool
	resolved    chan struct{}
}

func (p *badgeRecoveringPlatform) ResolveRunningLauncherBadgeTarget(ctx context.Context, process platform.ProcessIdentity, bundle string) (platform.LauncherBadgeTarget, error) {
	select {
	case p.resolved <- struct{}{}:
	default:
	}
	if p.unavailable.Load() {
		return platform.LauncherBadgeTarget{}, launcher.ErrUnavailable
	}
	return p.launcherIntegrationPlatform.ResolveRunningLauncherBadgeTarget(ctx, process, bundle)
}

func TestLauncherBadgesRetryInitialResolutionWithoutSettingsChange(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	resolver := &badgeRecoveringPlatform{launcherIntegrationPlatform: backend, resolved: make(chan struct{}, 8)}
	resolver.unavailable.Store(true)
	a.platform = resolver
	enableIntegrationBadges(t, a)
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 8)}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	select {
	case <-resolver.resolved:
	case <-time.After(time.Second):
		t.Fatal("no preparation")
	}
	before := a.GetLauncherBadges(p.Session)
	if source.calls.Load() != 0 || before.Entries[0].Count != nil {
		t.Fatal("unresolved target observed")
	}
	resolver.unavailable.Store(false)
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	after := a.GetLauncherBadges(p.Session)
	if before.Owner != after.Owner || source.calls.Load() != 1 {
		t.Fatal("recovery replaced generation", before, after)
	}
}

type badgeEndingSource struct {
	calls   atomic.Int32
	active  atomic.Int32
	overlap atomic.Bool
	entered chan time.Time
	release chan struct{}
}

func (f *badgeEndingSource) ObserveLauncherBadges(ctx context.Context, targets []platform.LauncherBadgeTarget, emit func(platform.LauncherBadgeSnapshot)) error {
	if f.active.Add(1) != 1 {
		f.overlap.Store(true)
	}
	defer f.active.Add(-1)
	n := f.calls.Add(1)
	f.entered <- time.Now()
	if n == 1 {
		select {
		case <-f.release:
		case <-ctx.Done():
			return ctx.Err()
		}
		return launcher.ErrUnavailable
	}
	count := uint32(3)
	emit(platform.LauncherBadgeSnapshot{Generation: 1, Sequence: 1, Status: platform.BadgeReady, Entries: []platform.LauncherBadgeEntry{{ItemKey: targets[0].ItemKey, TargetRevision: targets[0].TargetRevision, State: platform.BadgeKnown, Kind: platform.BadgeCount, Count: &count}}})
	<-ctx.Done()
	return ctx.Err()
}

func TestLauncherBadgesRetrySourceEndAfterCleanup(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	enableIntegrationBadges(t, a)
	source := &badgeEndingSource{entered: make(chan time.Time, 4), release: make(chan struct{})}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	<-source.entered
	before := a.GetLauncherBadges(p.Session)
	released := time.Now()
	close(source.release)
	var next time.Time
	select {
	case next = <-source.entered:
	case <-time.After(2500 * time.Millisecond):
		t.Fatal("source did not recover")
	}
	if next.Sub(released) < time.Second || source.overlap.Load() {
		t.Fatal("retry preceded cleanup/delay")
	}
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	if after := a.GetLauncherBadges(p.Session); after.Owner != before.Owner {
		t.Fatal("retry changed owner")
	}
}

type badgePartialPlatform struct{ *badgeRecoveringPlatform }

func (p *badgePartialPlatform) Apps() ([]domain.App, error) {
	return []domain.App{{ID: 4242, Name: "First", BundleID: "test.first"}, {ID: 4243, Name: "Second", BundleID: "test.second"}}, nil
}

func (p *badgePartialPlatform) ResolveRunningLauncherBadgeTarget(ctx context.Context, process platform.ProcessIdentity, bundle string) (platform.LauncherBadgeTarget, error) {
	if process.PID == 4243 && p.unavailable.Load() {
		return platform.LauncherBadgeTarget{}, launcher.ErrUnavailable
	}
	return p.launcherIntegrationPlatform.ResolveRunningLauncherBadgeTarget(ctx, process, bundle)
}

type badgePartialSource struct {
	launcherBadgeFixture
	ended     chan struct{}
	preparing chan struct{}
	resume    chan struct{}
}

func (f *badgePartialSource) ObserveLauncherBadges(ctx context.Context, targets []platform.LauncherBadgeTarget, emit func(platform.LauncherBadgeSnapshot)) error {
	if f.calls.Load() > 0 {
		select {
		case f.preparing <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		select {
		case <-f.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	err := f.launcherBadgeFixture.ObserveLauncherBadges(ctx, targets, emit)
	f.ended <- struct{}{}
	return err
}

func TestLauncherBadgesPartialRetryPreservesHealthyCount(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	p := &badgePartialPlatform{&badgeRecoveringPlatform{launcherIntegrationPlatform: backend}}
	p.unavailable.Store(true)
	a.platform = p
	a.launcher.core = launcher.New(launcher.Deps{Environment: p, Applications: p, Identities: p, View: appLauncherView{a}})
	enableIntegrationBadges(t, a)
	source := &badgePartialSource{launcherBadgeFixture: launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 8)}, ended: make(chan struct{}, 8), preparing: make(chan struct{}, 1), resume: make(chan struct{})}
	a.launcher.badges.source = source
	life, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				backend.send(100, 100)
			}
		}
	}()
	presentation := launcherIntegrationVisible(t, a, q)
	targets := <-source.entered
	if len(targets) != 1 {
		t.Fatal("unresolved target admitted")
	}
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(presentation.Session)
		return len(v.Entries) == 2 && v.Entries[0].Count != nil
	})
	before := a.GetLauncherBadges(presentation.Session)
	select {
	case <-source.ended:
	case <-time.After(2 * time.Second):
		t.Fatal("partial attempt did not end")
	}
	p.unavailable.Store(false)
	select {
	case <-source.preparing:
	case <-time.After(2 * time.Second):
		t.Fatal("next attempt missing")
	}
	// Prior cleanup finished, while the successor has not emitted any replacement.

	if v := a.GetLauncherBadges(presentation.Session); v.Entries[0].Count == nil {
		t.Fatal("scheduled retry cleared healthy count")
	}
	close(source.resume)
	select {
	case targets = <-source.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("partial retry missing")
	}
	if len(targets) != 2 {
		t.Fatal("recovered target missing", targets)
	}
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(presentation.Session)
		return len(v.Entries) == 2 && v.Entries[0].Count != nil && v.Entries[1].Count != nil
	})
	if after := a.GetLauncherBadges(presentation.Session); after.Owner != before.Owner || after.Sequence <= before.Sequence {
		t.Fatal("partial retry authority", before, after)
	}
}

type badgePartialTwoDisplays struct{ *badgePartialPlatform }

func (p *badgePartialTwoDisplays) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	return (&badgeTwoDisplayPlatform{p.launcherIntegrationPlatform}).ObserveLauncherEnvironment(ctx, emit)
}

func TestLauncherBadgesPartialRetryClearsFailedHost(t *testing.T) {
	backend := &badgePartialTwoDisplays{&badgePartialPlatform{&badgeRecoveringPlatform{launcherIntegrationPlatform: &launcherIntegrationPlatform{Fake: fake.New(), entered: make(chan platform.LauncherAppTarget, 4), stopped: make(chan struct{}, 8)}}}}
	backend.unavailable.Store(true)
	s := config.Default()
	s.ReplacementDock.Enabled = true
	s.ReplacementDock.Profiles[0].ShowBadges = true
	s.ReplacementDock.Bindings = append(s.ReplacementDock.Bindings, config.LauncherBinding{ID: "second", Target: "display", DisplayUUID: "22222222-2222-2222-2222-222222222222", ProfileID: "default"})
	a := newApp(backend, s, "")
	t.Cleanup(a.stopCapture)
	q := make(chan func(), 64)
	panels := map[string]*badgeDisplayPanel{}
	a.launcherFactory = func(session uint64, display string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		panel := &badgeDisplayPanel{launcherIntegrationPanel: launcherIntegrationPanel{token: session + 100}, display: display}
		panels[display] = panel
		host := &dockFakeWindow{}
		host.native = unsafe.Pointer(new(int))
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return host }, launcherPanelHostAdapter{source: badgePanelHost{panel}, uuid: display, style: style})
		d.onFailure = failed
		return d
	}
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 16)}
	a.launcher.badges.source = source
	life, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				backend.send(100, 100)
			}
		}
	}()
	a.startLauncher()
	var sessions []uint64
	launcherEventually(t, func() bool {
		for {
			select {
			case f := <-q:
				f()
			default:
				goto drained
			}
		}
	drained:
		ps := a.launcher.core.Snapshot().Presentations
		if len(ps) != 2 {
			return false
		}
		for _, p := range ps {
			v := a.GetLauncherBadges(p.Session)
			if len(v.Entries) != 2 || v.Entries[0].Count == nil {
				return false
			}
		}
		sessions = []uint64{ps[0].Session, ps[1].Session}
		return true
	})
	failed := a.GetLauncherBadges(sessions[0])
	healthy := a.GetLauncherBadges(sessions[1])
	panels[failed.DisplayUUID].unavailable.Store(true)
	launcherEventually(t, func() bool { return a.GetLauncherBadges(failed.Session).Entries[0].Count == nil })
	if v := a.GetLauncherBadges(healthy.Session); v.Entries[0].Count == nil {
		t.Fatal("other host count cleared", v)
	}
}

func TestLauncherBadgesPublicationResolutionFailureRetries(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	resolver := &badgeRecoveringPlatform{launcherIntegrationPlatform: backend, resolved: make(chan struct{}, 16)}
	a.platform = resolver
	enableIntegrationBadges(t, a)
	life, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-ticker.C:
				backend.send(100, 100)
			}
		}
	}()
	source := &launcherBadgeFixture{entered: make(chan []platform.LauncherBadgeTarget, 8), updates: make(chan platform.LauncherBadgeSnapshot)}
	a.launcher.badges.source = source
	p := launcherIntegrationVisible(t, a, q)
	targets := <-source.entered
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	resolver.unavailable.Store(true)
	count := uint32(8)
	source.updates <- platform.LauncherBadgeSnapshot{Generation: 1, Sequence: 2, Status: platform.BadgeReady, Entries: []platform.LauncherBadgeEntry{{ItemKey: targets[0].ItemKey, TargetRevision: targets[0].TargetRevision, State: platform.BadgeKnown, Kind: platform.BadgeCount, Count: &count}}}
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count == nil
	})
	resolver.unavailable.Store(false)
	launcherEventually(t, func() bool {
		v := a.GetLauncherBadges(p.Session)
		return len(v.Entries) == 1 && v.Entries[0].Count != nil
	})
	if source.calls.Load() != 2 {
		t.Fatal("missing joined retry", source.calls.Load())
	}
}
