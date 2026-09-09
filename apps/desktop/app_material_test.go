package main

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type materialFakeSurface struct {
	mu      sync.Mutex
	applies []platform.MaterialStyle
	retired []platform.MaterialScope
	closed  int
	started chan struct{}
	release chan struct{}
	err     error
}

func (s *materialFakeSurface) Apply(ctx context.Context, style platform.MaterialStyle) (platform.MaterialStatus, error) {
	s.mu.Lock()
	s.applies = append(s.applies, style)
	started, release := s.started, s.release
	s.started = nil
	s.mu.Unlock()
	if started != nil {
		close(started)
		<-release
	}
	if err := ctx.Err(); err != nil {
		return platform.MaterialStatus{}, err
	}
	return platform.MaterialStatus{Session: style.Scope.Session, Revision: style.Scope.Revision, State: "system"}, s.err
}

func (s *materialFakeSurface) Retire(scope platform.MaterialScope, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retired = append(s.retired, scope)
	return nil
}

func (s *materialFakeSurface) Close() error { s.mu.Lock(); defer s.mu.Unlock(); s.closed++; return nil }

func materialStyle(session, revision uint64) platform.MaterialStyle {
	return platform.MaterialStyle{Scope: platform.MaterialScope{Session: session, Revision: revision}, Enabled: true, Theme: "system", CornerRadiusPx: 12, Rect: domain.Bounds{X: 10, Y: 20, W: 300, H: 150}}
}

func TestMaterialOwnerCancelsBlockedApplyAndPublishesOnlyLatest(t *testing.T) {
	surface := &materialFakeSurface{started: make(chan struct{}), release: make(chan struct{})}
	started := surface.started
	statuses := make(chan platform.MaterialStatus, 8)
	owner := newAppMaterialOwner(func() (platform.MaterialSurface, error) { return surface, nil }, func(s platform.MaterialStatus) {
		if s.Reason != "preparing" {
			statuses <- s
		}
	})
	owner.apply(materialStyle(1, 1), "")
	<-started
	owner.apply(materialStyle(1, 2), "")
	owner.retire(platform.MaterialScope{Session: 1, Revision: 3}, 180)
	owner.apply(materialStyle(2, 4), "")
	close(surface.release)
	select {
	case s := <-statuses:
		if s.Session != 2 || s.Revision != 4 {
			t.Fatalf("stale %+v", s)
		}
	case <-time.After(time.Second):
		t.Fatal("no latest apply")
	}
	owner.close()
	<-owner.done
	surface.mu.Lock()
	defer surface.mu.Unlock()
	if surface.closed != 1 || len(surface.retired) != 1 {
		t.Fatal("lifetime", surface.closed, surface.retired)
	}
}

func TestMaterialOwnerFailureAndMissingReporterRemainFallback(t *testing.T) {
	statuses := make(chan platform.MaterialStatus, 4)
	surface := &materialFakeSurface{err: errors.New("fixture")}
	owner := newAppMaterialOwner(func() (platform.MaterialSurface, error) { return surface, nil }, func(s platform.MaterialStatus) {
		if s.Reason != "preparing" {
			statuses <- s
		}
	})
	owner.apply(materialStyle(1, 1), "")
	s := <-statuses
	if s.State != "unavailable" {
		t.Fatal(s)
	}
	style := materialStyle(1, 2)
	style.Enabled = false
	style.Rect = domain.Bounds{}
	owner.apply(style, "missingReporter")
	s = <-statuses
	if s.State != "unavailable" || s.Reason != "missingReporter" {
		t.Fatal(s)
	}
	owner.close()
	<-owner.done
}

type materialDockHost struct {
	dockFakeHost
	surface *materialFakeSurface
	created int
}

func (h *materialDockHost) CreatePreviewMaterial(panel platform.DockPanel) (platform.MaterialSurface, error) {
	h.created++
	if panel.(*dockFakePanel).shows == 0 {
		return nil, errors.New("unmeasured panel")
	}
	return h.surface, nil
}

func TestDockMaterialFailurePreservesPanelAndClosesBeforeHost(t *testing.T) {
	q := make(dockQueue, 8)
	surface := &materialFakeSurface{err: errors.New("unsupported")}
	host := &materialDockHost{surface: surface}
	w := &dockFakeWindow{}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, host)
	statuses := make(chan platform.MaterialStatus, 8)
	style := materialStyle(1, 1)
	style.Rect = domain.Bounds{W: 300, H: 150}
	d.setMaterial(style, func(s platform.MaterialStatus, _ func() bool) {
		if s.Reason != "preparing" {
			statuses <- s
		}
	})
	d.show(domain.Bounds{W: 300, H: 150})
	q.next(t)
	if host.panels[0].shows != 1 || host.panels[0].closes != 0 || (<-statuses).State != "unavailable" {
		t.Fatal("material failure destroyed preview")
	}
	w.onClose = func() {
		surface.mu.Lock()
		defer surface.mu.Unlock()
		if surface.closed != 1 {
			t.Error("host closed before material")
		}
	}
	d.close()
	q.next(t)
}

func TestDockMaterialBlockedApplyHideReopenDropsOldStatus(t *testing.T) {
	q := make(dockQueue, 8)
	surface := &materialFakeSurface{started: make(chan struct{}), release: make(chan struct{})}
	started := surface.started
	host := &materialDockHost{surface: surface}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return &dockFakeWindow{} }, host)
	statuses := make(chan platform.MaterialStatus, 8)
	d.setMaterial(materialStyle(1, 1), func(s platform.MaterialStatus, _ func() bool) {
		if s.Reason != "preparing" {
			statuses <- s
		}
	})
	d.show(domain.Bounds{W: 400, H: 300})
	finished := make(chan struct{})
	go func() { q.next(t); close(finished) }()
	<-started
	d.hide()
	d.setMaterial(materialStyle(2, 2), func(s platform.MaterialStatus, _ func() bool) {
		if s.Reason != "preparing" {
			statuses <- s
		}
	})
	d.show(domain.Bounds{W: 400, H: 300})
	close(surface.release)
	<-finished
	q.next(t)
	select {
	case s := <-statuses:
		if s.Session != 2 {
			t.Fatal("old status", s)
		}
	case <-time.After(time.Second):
		t.Fatal("new status missing")
	}
	d.close()
	q.next(t)
}

type materialAppPlatform struct {
	*fake.Fake
	surface *materialFakeSurface
	create  func()
}

func (p *materialAppPlatform) CreateOverlayMaterial(unsafe.Pointer) (platform.MaterialSurface, error) {
	if p.create != nil {
		p.create()
	}
	return p.surface, nil
}

func materialAppFixture(t *testing.T, surface *materialFakeSurface) (*App, *materialAppPlatform, uint64) {
	t.Helper()
	p := &materialAppPlatform{Fake: fake.New(), surface: surface}
	p.SetWindows(appTestWindows())
	settings := config.Default()
	settings.Shortcuts[0].Mode = config.ModeWindows
	settings.Appearance.FadeOutAnimation = false
	a := newApp(p, settings, "")
	w := &fakeWindow{native: unsafe.Pointer(new(int))}
	a.setRuntime(nil, w, &fakeWindow{}, nil)
	t.Cleanup(a.stopCapture)
	st, err := a.controller.Open(config.ModeWindows)
	if err != nil {
		t.Fatal(err)
	}
	return a, p, st.Session
}

func materialStateRevision(a *App) uint64 {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.switcherRevision
}

func TestSwitcherMaterialRectangleAdmissionAndContentRetention(t *testing.T) {
	surface := &materialFakeSurface{}
	a, _, session := materialAppFixture(t, surface)
	status := a.GetSwitcherMaterialStatus(session)
	if status.State != "unavailable" || status.Reason != "missingReporter" {
		t.Fatal(status)
	}
	revision := materialStateRevision(a)
	if err := a.SetSwitcherMaterialRect(session, revision-1, 1, 10, 20, 300, 150); err == nil {
		t.Fatal("stale state accepted")
	}
	if err := a.SetSwitcherMaterialRect(session, revision, 1, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	pollUntil(t, time.Second, "native system material", func() bool { return a.GetSwitcherMaterialStatus(session).State == "system" })
	applied := a.GetSwitcherMaterialStatus(session)
	if applied.Revision <= status.Revision {
		t.Fatal("material completion was not newer")
	}
	if err := a.SetSwitcherMaterialRect(session, revision, 1, 10, 20, 310, 150); err == nil {
		t.Fatal("replayed sequence accepted")
	}
	a.controller.Advance()
	updated := materialStateRevision(a)
	if updated == revision {
		t.Fatal("no selection update")
	}
	if got := a.GetSwitcherMaterialStatus(session); got.Revision != applied.Revision || got.State != "system" {
		t.Fatal("content retired material", got)
	}
	if err := a.SetSwitcherMaterialRect(session, updated, 2, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	if got := a.GetSwitcherMaterialStatus(session); got.Revision != applied.Revision {
		t.Fatal("unchanged rect re-applied")
	}
	if err := a.SetSwitcherMaterialRect(session, updated, 3, math.NaN(), 20, 300, 150); err == nil {
		t.Fatal("invalid rect accepted")
	}
	if got := a.GetSwitcherMaterialStatus(session); got.State != "unavailable" {
		t.Fatal("invalid rect retained native truth", got)
	}
}

func TestSwitcherMaterialResizeRequiresFreshStateAndRectangle(t *testing.T) {
	a, _, session := materialAppFixture(t, &materialFakeSurface{})
	revision := materialStateRevision(a)
	if err := a.SetSwitcherMaterialRect(session, revision, 1, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	epoch := a.overlay.materialResized()
	if err := a.SetSwitcherMaterialRect(session, revision, 2, 10, 20, 300, 150); err == nil {
		t.Fatal("old-size report accepted")
	}
	a.materialHostResized(a.overlay, epoch)
	next := materialStateRevision(a)
	if next <= revision {
		t.Fatal("resize failed to publish new state")
	}
	if err := a.SetSwitcherMaterialRect(session, revision, 3, 10, 20, 320, 150); err == nil {
		t.Fatal("prior state report accepted")
	}
	if err := a.SetSwitcherMaterialRect(session, next, 3, 10, 20, 320, 150); err != nil {
		t.Fatal(err)
	}
}

func TestSwitcherMaterialNativeReentryAndHideCancel(t *testing.T) {
	surface := &materialFakeSurface{started: make(chan struct{}), release: make(chan struct{})}
	started := surface.started
	a, p, session := materialAppFixture(t, surface)
	p.create = func() { _ = a.GetSwitcherMaterialStatus(session) }
	if err := a.SetSwitcherMaterialRect(session, materialStateRevision(a), 1, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("create reentry deadlocked")
	}
	a.HideSession(session)
	close(surface.release)
	if got := a.GetSwitcherMaterialStatus(session); got.State == "system" {
		t.Fatal("hidden native status admitted")
	}
	pollUntil(t, time.Second, "native hide retirement", func() bool { surface.mu.Lock(); defer surface.mu.Unlock(); return len(surface.retired) > 0 })
	a.viewMu.Lock()
	owner := a.switcherMaterial.owner
	a.viewMu.Unlock()
	owner.close()
	<-owner.done
	surface.mu.Lock()
	defer surface.mu.Unlock()
	if len(surface.retired) == 0 {
		t.Fatal("hide did not retire native session")
	}
}

func TestDockMaterialSameSessionContentReturnCreatesFreshSurface(t *testing.T) {
	q := make(dockQueue, 8)
	host := &materialDockHost{surface: &materialFakeSurface{}}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return &dockFakeWindow{} }, host)
	d.setMaterial(materialStyle(1, 1), nil)
	d.show(domain.Bounds{W: 400, H: 300})
	q.next(t)
	d.clearMaterial()
	q.next(t)
	d.setMaterial(materialStyle(1, 2), nil)
	q.next(t)
	if host.created != 2 {
		t.Fatal("terminal same-session material reused", host.created)
	}
	d.close()
	q.next(t)
}

func TestSwitcherMaterialBlurABAReplacesPreparingAuthority(t *testing.T) {
	surface := &materialFakeSurface{started: make(chan struct{}), release: make(chan struct{})}
	started := surface.started
	a, _, session := materialAppFixture(t, surface)
	if err := a.SetSwitcherMaterialRect(session, materialStateRevision(a), 1, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	<-started
	st := a.controller.State()
	st.Appearance.Blur = false
	a.Update(st)
	if s := a.GetSwitcherMaterialStatus(session); s.State != "solid" {
		t.Fatal("false blur fallback", s)
	}
	st.Appearance.Blur = true
	a.Update(st)
	if s := a.GetSwitcherMaterialStatus(session); s.Reason != "missingReporter" {
		t.Fatal("style change retained old rect", s)
	}
	if err := a.SetSwitcherMaterialRect(session, materialStateRevision(a), 2, 20, 30, 320, 150); err != nil {
		t.Fatal(err)
	}
	close(surface.release)
	pollUntil(t, time.Second, "latest blur material", func() bool { return a.GetSwitcherMaterialStatus(session).State == "system" })
	surface.mu.Lock()
	last := surface.applies[len(surface.applies)-1]
	surface.mu.Unlock()
	if !last.Enabled || last.Rect.X != 20 {
		t.Fatal("old blur authority applied", last)
	}
}

func TestLiveMaterialHostCloseRetiresOwner(t *testing.T) {
	owner := newAppMaterialOwner(nil, nil)
	w := newLiveWindow(&fakeWindow{})
	w.bindMaterial(owner)
	w.markClosed()
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("host retirement blocked")
	}
	if w.alive() {
		t.Fatal("host remained alive")
	}
}

func TestSwitcherMaterialHostClosureDuringNativeCreateCannotPublish(t *testing.T) {
	surface := &materialFakeSurface{}
	a, p, session := materialAppFixture(t, surface)
	a.viewMu.Lock()
	owner := a.switcherMaterial.owner
	host := a.overlay
	a.viewMu.Unlock()
	p.create = func() { host.markClosed(); a.materialHostClosed(host) }
	if err := a.SetSwitcherMaterialRect(session, materialStateRevision(a), 1, 10, 20, 300, 150); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("native close callback deadlocked")
	}
	if status := a.GetSwitcherMaterialStatus(session); status.State == "system" {
		t.Fatal("closed host published material")
	}
	surface.mu.Lock()
	defer surface.mu.Unlock()
	if surface.closed != 1 {
		t.Fatal("created surface leaked", surface.closed)
	}
}

func TestDockMaterialCompletionDoesNotBlockNativeQueueOnAppLock(t *testing.T) {
	q := make(dockQueue, 8)
	host := &materialDockHost{surface: &materialFakeSurface{}}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return &dockFakeWindow{} }, host)
	a := &App{}
	a.viewMu.Lock()
	d.setMaterial(materialStyle(1, 1), func(platform.MaterialStatus, func() bool) { a.viewMu.Lock(); a.materialRevision++; a.viewMu.Unlock() })
	d.show(domain.Bounds{W: 400, H: 300})
	finished := make(chan struct{})
	go func() { q.next(t); close(finished) }()
	fast := false
	select {
	case <-finished:
		fast = true
	case <-time.After(100 * time.Millisecond):
	}
	a.viewMu.Unlock()
	<-finished
	d.close()
	q.next(t)
	if !fast {
		t.Fatal("native UI queue waited for App.viewMu")
	}
}

type materialBoundsPanel struct {
	bounds     domain.Bounds
	duringShow func()
}

func (p *materialBoundsPanel) Show(b domain.Bounds) error {
	p.bounds = b
	if p.duringShow != nil {
		f := p.duringShow
		p.duringShow = nil
		f()
	}
	return nil
}
func (p *materialBoundsPanel) Hide() error  { return nil }
func (p *materialBoundsPanel) Close() error { return nil }

type materialBoundsSurface struct {
	materialFakeSurface
	panel   *materialBoundsPanel
	invalid int
}

func (s *materialBoundsSurface) Apply(ctx context.Context, style platform.MaterialStyle) (platform.MaterialStatus, error) {
	if style.Rect.W != s.panel.bounds.W || style.Rect.H != s.panel.bounds.H {
		s.invalid++
		return platform.MaterialStatus{}, errors.New("bounds mismatch")
	}
	return s.materialFakeSurface.Apply(ctx, style)
}

type materialBoundsHost struct {
	panel   *materialBoundsPanel
	surface *materialBoundsSurface
}

func (h materialBoundsHost) CreateDockPanel(unsafe.Pointer) (platform.DockPanel, error) {
	return h.panel, nil
}

func (h materialBoundsHost) CreatePreviewMaterial(platform.DockPanel) (platform.MaterialSurface, error) {
	return h.surface, nil
}

func TestDockMaterialBoundsAndRequestAreAdmittedTogether(t *testing.T) {
	q := make(dockQueue, 8)
	panel := &materialBoundsPanel{}
	surface := &materialBoundsSurface{panel: panel}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return &dockFakeWindow{} }, materialBoundsHost{panel: panel, surface: surface})
	request := func(revision uint64, width float64) {
		style := materialStyle(1, revision)
		style.Rect = domain.Bounds{W: width, H: 150}
		d.showMaterial(domain.Bounds{W: width, H: 150}, style, nil)
	}
	request(1, 300)
	q.next(t)
	panel.duringShow = func() { request(3, 600) }
	request(2, 400)
	q.next(t)
	q.next(t)
	if surface.invalid != 0 {
		t.Fatal("new material applied against old panel bounds")
	}
	surface.mu.Lock()
	last := surface.applies[len(surface.applies)-1]
	surface.mu.Unlock()
	if last.Rect.W != 600 || last.Scope.Revision != 3 {
		t.Fatal("successor not applied", last)
	}
	d.close()
	q.next(t)
}
