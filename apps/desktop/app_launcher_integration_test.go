package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

const integrationDisplay = "11111111-1111-1111-1111-111111111111"

type launcherIntegrationPlatform struct {
	*fake.Fake
	mu        sync.Mutex
	emit      func(platform.LauncherEnvironment)
	sequence  uint64
	entered   chan platform.LauncherAppTarget
	release   chan struct{}
	mutations atomic.Int32
	stopped   chan struct{}
}

func (p *launcherIntegrationPlatform) Apps() ([]domain.App, error) {
	return []domain.App{{ID: 4242, Name: "Owned fixture", BundleID: "test.launcher.fixture"}}, nil
}

func (p *launcherIntegrationPlatform) ProcessIdentity(id domain.AppID) (platform.ProcessIdentity, error) {
	return platform.ProcessIdentity{PID: id, StartSeconds: 123}, nil
}

func (p *launcherIntegrationPlatform) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	p.mu.Lock()
	p.emit = emit
	p.sequence = 0
	p.mu.Unlock()
	p.send(100, 100)
	<-ctx.Done()
	p.stopped <- struct{}{}
	return ctx.Err()
}

func (p *launcherIntegrationPlatform) send(x, y float64) {
	p.mu.Lock()
	p.sequence++
	seq := p.sequence
	emit := p.emit
	p.mu.Unlock()
	if emit == nil {
		return
	}
	emit(platform.LauncherEnvironment{Generation: 1, Sequence: seq, ObservedAt: time.Now(), Complete: true, Status: "ready", PointerKnown: true, PointerX: x, PointerY: y, NativeDock: platform.LauncherNativeDock{Process: platform.ProcessIdentity{PID: 1, StartSeconds: 1}, Edge: "bottom", Visibility: "hidden", Confidence: "known", Bounds: domain.Bounds{X: 400, Y: 780, W: 200, H: 20}}, Displays: []platform.LauncherDisplay{{UUID: integrationDisplay, Main: true, Frame: domain.Bounds{W: 1000, H: 800}, UsableFrame: domain.Bounds{Y: 25, W: 1000, H: 775}, Scale: 2, SpaceID: 1, SpaceKind: "ordinary", SpaceStatus: "known"}}})
}

func (p *launcherIntegrationPlatform) ActivateLauncherApp(_ context.Context, target platform.LauncherAppTarget, guard func() error) error {
	p.entered <- target
	if p.release != nil {
		<-p.release
	}
	if err := guard(); err != nil {
		return err
	}
	p.mutations.Add(1)
	return nil
}

type launcherIntegrationPanel struct {
	token  uint64
	shows  atomic.Int32
	closes atomic.Int32
}

func (p *launcherIntegrationPanel) LauncherToken() uint64    { return p.token }
func (p *launcherIntegrationPanel) Show(domain.Bounds) error { p.shows.Add(1); return nil }
func (p *launcherIntegrationPanel) Hide() error              { return nil }
func (p *launcherIntegrationPanel) Close() error             { p.closes.Add(1); return nil }

type launcherIntegrationHost struct{ panel *launcherIntegrationPanel }

func (h launcherIntegrationHost) CreateDockPanel(unsafe.Pointer) (platform.DockPanel, error) {
	return h.panel, nil
}

func launcherEventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("launcher condition timed out")
}

func launcherIntegrationApp(t *testing.T) (*App, *launcherIntegrationPlatform, chan func(), *launcherIntegrationPanel) {
	t.Helper()
	p := &launcherIntegrationPlatform{Fake: fake.New(), entered: make(chan platform.LauncherAppTarget, 4), stopped: make(chan struct{}, 8)}
	s := config.Default()
	s.ReplacementDock.Enabled = true
	s.Dock.Enabled = true
	a := newApp(p, s, "")
	q := make(chan func(), 64)
	panel := &launcherIntegrationPanel{token: 99}
	a.launcherFactory = func(_ uint64, _ string, failed func()) *dockWindow {
		w := &dockFakeWindow{}
		w.native = unsafe.Pointer(new(int))
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, launcherIntegrationHost{panel})
		d.onFailure = failed
		return d
	}
	t.Cleanup(a.stopCapture)
	return a, p, q, panel
}

func launcherIntegrationVisible(t *testing.T, a *App, q chan func()) launcher.Presentation {
	t.Helper()
	a.startLauncher()
	launcherEventually(t, func() bool {
		select {
		case f := <-q:
			f()
		default:
		}
		s := a.launcher.core.Snapshot()
		return len(s.Presentations) > 0 && s.Presentations[0].Visible && len(s.Presentations[0].Items) > 0 && a.GetLauncherState(s.Presentations[0].Session).Visible && a.launcherPanelReady(s.Presentations[0].Session)
	})
	return a.launcher.core.Snapshot().Presentations[0]
}

func (a *App) launcherPanelReady(session uint64) bool {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	h := a.launcher.hosts[session]
	return h != nil && h.window.wheelPanel() != nil
}

func activateIntegration(a *App, p launcher.Presentation) error {
	return a.ActivateLauncherItem(p.Epoch, p.DisplayUUID, p.Session, p.Revision, p.Items[0].ID)
}

func TestLauncherIntegrationExactTargetAndPublicationGap(t *testing.T) {
	a, backend, q, panel := launcherIntegrationApp(t)
	p := launcherIntegrationVisible(t, a, q)
	if err := activateIntegration(a, p); err != nil {
		t.Fatal(err)
	}
	target := <-backend.entered
	if target.Process.PID != 4242 || target.Process.StartSeconds != 123 || target.BundleID != "test.launcher.fixture" || target.Name != "Owned fixture" || target.PanelToken != panel.token || target.DisplayUUID != integrationDisplay {
		t.Fatalf("wrong native target %+v", target)
	}
	backend.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- activateIntegration(a, p) }()
	<-backend.entered
	a.settingsMu.Lock()
	a.settings.Behavior.Paused = true
	a.settingsMu.Unlock()
	close(backend.release)
	if err := <-done; !errors.Is(err, launcher.ErrRetired) {
		t.Fatalf("publication gap action: %v", err)
	}
	if backend.mutations.Load() != 1 {
		t.Fatal("paused action mutated native fixture")
	}
}

func TestLauncherIntegrationHostFailureRetiresScope(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	p := launcherIntegrationVisible(t, a, q)
	a.viewMu.Lock()
	w := a.launcher.hosts[p.Session].window
	a.viewMu.Unlock()
	a.launcherHostFailed(p.Session, w)
	if a.GetLauncherState(p.Session).Visible {
		t.Fatal("failed host remained visible")
	}
	if err := activateIntegration(a, p); !errors.Is(err, launcher.ErrRetired) {
		t.Fatalf("failed host admitted: %v", err)
	}
	a.stopCapture()
	if a.GetLauncherState(p.Session).Visible {
		t.Fatal("shutdown revived host")
	}
}

func TestLauncherIntegrationPointerYieldsNativeHover(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	a.settingsMu.Lock()
	a.settings.Dock.Media = config.DockMediaSettings{Enabled: true, MusicEnabled: true}
	a.settingsMu.Unlock()
	a.wireMedia(&appMediaSource{started: make(chan func(platform.MediaSample), 4), stopped: make(chan struct{}, 4)}, nil)
	a.viewMu.Lock()
	pin := a.newMediaPresentationLocked(platform.MediaMusic, true)
	a.viewMu.Unlock()

	p := launcherIntegrationVisible(t, a, q)
	a.viewMu.Lock()
	initial := a.dockAllowedLocked()
	a.viewMu.Unlock()
	if !initial {
		t.Fatal("launcher outside pointer suppressed native hover")
	}
	backend.send(p.Bounds.X+p.Bounds.W/2, p.Bounds.Y+p.Bounds.H/2)
	launcherEventually(t, func() bool {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		return a.launcher.pointerOwned && !a.dockAllowedLocked()
	})
	a.viewMu.Lock()
	pinSurvived := a.media.panels[pin.state.Session] == pin && a.mediaPanelAllowedLocked(pin)
	a.viewMu.Unlock()
	if !pinSurvived {
		t.Fatal("launcher pointer retired independent media pin")
	}
	backend.send(100, 100)
	launcherEventually(t, func() bool {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		return !a.launcher.pointerOwned && a.dockAllowedLocked()
	})
}

type launcherDrainSource struct{ started, cancelled, release chan struct{} }

func (s *launcherDrainSource) DockMonitorLockDisplays(context.Context) ([]platform.DockLockDisplay, error) {
	return nil, nil
}

func (s *launcherDrainSource) PlaceDock(context.Context, platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
	return platform.DockPlacementResult{}, errors.New("not used")
}

func (s *launcherDrainSource) ObserveDockMonitorLock(ctx context.Context, _ platform.DockMonitorLockPolicy, _ func(platform.DockMonitorLockState)) error {
	close(s.started)
	<-ctx.Done()
	close(s.cancelled)
	<-s.release
	return ctx.Err()
}

func TestLauncherIntegrationSourceJoinsBeforeHostShow(t *testing.T) {
	a, _, q, panel := launcherIntegrationApp(t)
	source := &launcherDrainSource{started: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
	c := dock.NewMonitorLockController(dock.MonitorLockControllerDeps{Source: source})
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	mediaReceive(t, source.started)
	a.viewMu.Lock()
	a.dockMonitorLock = c
	a.viewMu.Unlock()
	a.startLauncher()
	mediaReceive(t, source.cancelled)
	if len(q) != 0 || panel.shows.Load() != 0 {
		t.Fatal("host queued before source joined")
	}
	a.viewMu.Lock()
	ready := a.launcher.ready
	a.viewMu.Unlock()
	if ready {
		t.Fatal("launcher admitted before native drain")
	}
	close(source.release)
	p := launcherIntegrationVisible(t, a, q)
	if !p.Visible || panel.shows.Load() != 1 {
		t.Fatal("joined source did not enable host")
	}
	a.stopCapture()
	cancel()
	mediaReceive(t, done)
}

func TestLauncherIntegrationPauseResumesFreshSessionAndRecoveryCloses(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	p := launcherIntegrationVisible(t, a, q)
	a.settingsMu.Lock()
	a.settings.Behavior.Paused = true
	a.settingsMu.Unlock()
	a.viewMu.Lock()
	a.syncLauncherLocked()
	a.viewMu.Unlock()
	if a.GetLauncherState(p.Session).Visible {
		t.Fatal("pause retained host")
	}
	a.settingsMu.Lock()
	a.settings.Behavior.Paused = false
	a.settingsMu.Unlock()
	a.viewMu.Lock()
	a.syncLauncherLocked()
	a.viewMu.Unlock()
	fresh := launcherIntegrationVisible(t, a, q)
	if fresh.Session == p.Session {
		t.Fatal("resume reused retired host session")
	}
	if err := a.UseNativeDock(); err != nil {
		t.Fatal(err)
	}
	if a.GetLauncherState(fresh.Session).Visible || a.settingsSnapshot().ReplacementDock.Enabled {
		t.Fatal("recovery did not retire and persist disabled")
	}
}

func TestLauncherIntegrationNameExclusionPublicationGap(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	p := launcherIntegrationVisible(t, a, q)
	backend.release = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- activateIntegration(a, p) }()
	mediaReceive(t, backend.entered)
	a.settingsMu.Lock()
	a.settings.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "Owned fixture", Hide: config.HideAlways}}
	a.settingsMu.Unlock()
	close(backend.release)
	if err := mediaReceive(t, done); !errors.Is(err, launcher.ErrRetired) {
		t.Fatalf("name exclusion publication gap: %v", err)
	}
	if backend.mutations.Load() != 0 {
		t.Fatal("newly excluded app was activated")
	}
}
