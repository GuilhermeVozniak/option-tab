package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform/fake"

	"option-tab/internal/platform"
)

func TestLauncherChildSnapshotOwnsEntriesAndOpaqueIdentity(t *testing.T) {
	p := &launcherItemPanel{state: LauncherItemPanelState{Session: 8, Folder: &LauncherItemFolderState{FolderIdentity: "launcher-child:8", Entries: []platform.FolderEntry{{ID: "opaque", Name: "owned"}}}}}
	got := cloneLauncherItemPanelState(p.state)
	got.Folder.Entries[0].Name = "mutated"
	if p.state.Folder.Entries[0].Name != "owned" {
		t.Fatal("snapshot aliases live entries")
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "file:") || !strings.Contains(string(b), `"folderIdentity":"launcher-child:8"`) {
		t.Fatal(string(b))
	}
}

type childCoreFixture struct {
	parent  launcher.ChildParent
	retired atomic.Bool
}

func (c *childCoreFixture) CaptureChildParent(s launcher.Scope, id string) (launcher.ChildParent, error) {
	if c.retired.Load() || s != c.parent.Scope || id != c.parent.ItemID {
		return launcher.ChildParent{}, launcher.ErrRetired
	}
	return c.parent, nil
}

func (c *childCoreFixture) ValidateChildParent(p launcher.ChildParent) (launcher.Scope, error) {
	if c.retired.Load() || p.Admission != c.parent.Admission {
		return launcher.Scope{}, launcher.ErrRetired
	}
	return c.parent.Scope, nil
}

func (c *childCoreFixture) PlaceChild(_ launcher.ChildParent, w, h float64) (domain.Bounds, error) {
	return domain.Bounds{X: 100, Y: 100, W: w, H: h}, nil
}

func (c *childCoreFixture) SetChildBounds(launcher.ChildParent, uint64, domain.Bounds) error {
	return nil
}
func (c *childCoreFixture) ClearChildBounds(launcher.ChildParent, uint64) {}

type childReferenceFixture struct{ launcherReferenceFixture }

func (s *childReferenceFixture) WithLauncherFolder(ctx context.Context, id string, rev uint64, use func(platform.FolderRef) error) error {
	r, err := s.ResolveLauncherReference(ctx, id)
	if err != nil {
		return err
	}
	if r.Revision != rev {
		return launcher.ErrRetired
	}
	return use(platform.FolderRef{Identity: "file:///owned/private-folder", Path: "/owned/private-folder"})
}

type childFolderFixture struct {
	entered     chan struct{}
	release     chan struct{}
	calls       atomic.Int32
	beforeGuard func()
	opened      atomic.Int32
}

func (s *childFolderFixture) ListFolder(context.Context, platform.FolderRef, platform.FolderSort) (platform.FolderListing, error) {
	s.calls.Add(1)
	if s.entered != nil {
		s.entered <- struct{}{}
	}
	if s.release != nil {
		<-s.release
	}
	return platform.FolderListing{Status: "ready", FolderIdentity: "file:///owned/private-folder", Entries: []platform.FolderEntry{{ID: "entry", Name: "Owned.txt", Kind: "file"}}}, nil
}

func (s *childFolderFixture) RequestFolderAccess(context.Context, platform.FolderRef) (platform.FolderGrant, error) {
	panic("unexpected chooser")
}

func (s *childFolderFixture) OpenFolderEntryGuarded(_ context.Context, _ platform.FolderRef, _ string, guard func() error) error {
	if s.beforeGuard != nil {
		s.beforeGuard()
	}
	if err := guard(); err != nil {
		return err
	}
	s.opened.Add(1)
	return nil
}

func childAppFixture(t *testing.T, sources ...*childFolderFixture) (*App, *childReferenceFixture, *childCoreFixture) {
	t.Helper()
	settings := config.Default()
	settings.ReplacementDock.Enabled = true
	a := newApp(fake.New(), settings, "")
	ref := platform.LauncherReference{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "folder", Label: "Owned", State: "ready", Revision: 1}
	source := &childReferenceFixture{launcherReferenceFixture: launcherReferenceFixture{records: map[string]platform.LauncherReference{ref.ID: ref}}}
	a.wireLauncherItems(source, nil)
	core := &childCoreFixture{parent: launcher.ChildParent{Scope: launcher.Scope{Epoch: 1, DisplayUUID: integrationDisplay, Session: 1, Revision: 1}, ProfileID: "default", ItemID: "folder", Configured: config.LauncherItem{ID: "folder", Kind: "folder", ReferenceID: ref.ID}, Reference: ref, Admission: 1}}
	q := make(chan func(), 128)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case f := <-q:
				f()
			case <-stop:
				return
			}
		}
	}()
	factory := func(_ uint64, uuid string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		w := &dockFakeWindow{}
		w.native = unsafe.Pointer(new(int))
		panel := &launcherIntegrationPanel{token: 99}
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, launcherPanelHostAdapter{source: launcherIntegrationHost{panel}, uuid: uuid, style: style})
		d.onFailure = failed
		return d
	}
	parent := factory(1, integrationDisplay, platform.LauncherPanelStyle{Material: "solid", Theme: "system"}, func() {})
	parent.show(domain.Bounds{W: 300, H: 60})
	launcherEventually(t, func() bool { return parent.wheelPanel() != nil })
	a.viewMu.Lock()
	a.launcherFactory = factory
	a.launcherItemPanelFactory = factory
	a.launcher.started = true
	a.launcher.available = true
	a.launcher.ready = true
	a.launcher.configuration = settings.ReplacementDock
	a.launcher.hosts[1] = &appLauncherHost{window: parent, presentation: launcher.Presentation{Scope: core.parent.Scope, Visible: true, Appearance: settings.ReplacementDock.Profiles[0].Appearance}}
	a.launcherItemPanels.core = core
	count := 0
	a.launcherItemPanels.folderFactory = func() platform.FolderSource {
		if count >= len(sources) {
			panic("extra source")
		}
		s := sources[count]
		count++
		return s
	}
	a.viewMu.Unlock()
	t.Cleanup(func() {
		a.stopCapture()
		a.viewMu.Lock()
		m := a.launcherItems
		a.viewMu.Unlock()
		if m != nil {
			<-m.done
		}
		<-parent.closeAndDrain()
		close(stop)
	})
	return a, source, core
}

func showChild(t *testing.T, a *App) LauncherItemPanelState {
	t.Helper()
	s, err := a.ShowLauncherItemPanel(1, integrationDisplay, 1, 1, "folder")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func waitChildReady(t *testing.T, a *App, session uint64) LauncherItemPanelState {
	t.Helper()
	var state *LauncherItemPanelState
	launcherEventually(t, func() bool {
		state = a.GetLauncherItemPanelState(session)
		return state != nil && state.Folder.Status == "ready"
	})
	return *state
}

type childReadinessPanel struct {
	*launcherIntegrationPanel
	first            sync.Once
	entered, release chan struct{}
	physical         atomic.Bool
}

func (p *childReadinessPanel) ValidateLauncherPanel(ctx context.Context, display string) error {
	visible := p.physical.Load()
	p.first.Do(func() {
		close(p.entered)
		select {
		case <-p.release:
		case <-ctx.Done():
		}
	})
	if !visible {
		return platform.ErrDockPanelHostClosed
	}
	return p.launcherIntegrationPanel.ValidateLauncherPanel(ctx, display)
}

type childReadinessHost struct{ panel platform.DockPanel }

func (h childReadinessHost) CreateDockPanel(unsafe.Pointer) (platform.DockPanel, error) {
	return h.panel, nil
}

func childWaitingForVisibility(t *testing.T, a *App, visible bool) (*childReadinessPanel, *launcherItemPanel) {
	t.Helper()
	panel := &childReadinessPanel{launcherIntegrationPanel: &launcherIntegrationPanel{token: 100}, entered: make(chan struct{}), release: make(chan struct{})}
	panel.physical.Store(visible)
	a.viewMu.Lock()
	factory := a.launcherItemPanelFactory
	a.launcherItemPanelFactory = func(session uint64, uuid string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		d := factory(session, uuid, style, failed)
		d.host = childReadinessHost{panel: panel}
		return d
	}
	a.viewMu.Unlock()
	showChild(t, a)
	select {
	case <-panel.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("child never reached physical visibility validation")
	}
	a.viewMu.Lock()
	p := a.launcherItemPanels.owners[1]
	a.viewMu.Unlock()
	return panel, p
}

func TestLauncherChildWaitsForPhysicalVisibilityBeforeFolderAccess(t *testing.T) {
	fs := &childFolderFixture{}
	a, _, _ := childAppFixture(t, fs)
	panel, p := childWaitingForVisibility(t, a, false)
	state := a.GetLauncherItemPanelState(p.state.Session)
	if state == nil || state.Folder.Status != "loading" {
		t.Fatal("child was not loading while native visibility was pending")
	}
	if err := a.OpenLauncherFolderEntry(state.Session, state.Revision, "entry"); err == nil || fs.calls.Load() != 0 || fs.opened.Load() != 0 {
		t.Fatal("folder access admitted before native visibility", err)
	}
	panel.physical.Store(true)
	close(panel.release) // The first physical check still returns its earlier refusal.
	ready := waitChildReady(t, a, state.Session)
	if len(ready.Folder.Entries) != 1 || ready.Folder.Entries[0].Name != "Owned.txt" {
		t.Fatal("ready child did not publish its folder contents", ready)
	}
}

func TestLauncherChildPhysicalVisibilityDeadlineRetiresAndDrains(t *testing.T) {
	fs := &childFolderFixture{}
	a, _, _ := childAppFixture(t, fs)
	panel, p := childWaitingForVisibility(t, a, false)
	close(panel.release)
	select {
	case <-p.done:
		t.Fatal("transient physical refusal retired the child before its readiness deadline")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-p.done:
	case <-time.After(3 * time.Second):
		t.Fatal("unavailable child did not retire and drain within its readiness deadline")
	}
	if a.GetLauncherItemPanelState(p.state.Session) != nil || panel.closes.Load() != 1 || fs.calls.Load() != 0 {
		t.Fatal("unavailable child retained its host or accessed its folder")
	}
}

func TestLauncherChildPendingVisibilityCannotSurviveRetirement(t *testing.T) {
	for _, reason := range []string{"cancel", "parent", "child token"} {
		t.Run(reason, func(t *testing.T) {
			fs := &childFolderFixture{}
			a, _, core := childAppFixture(t, fs)
			panel, p := childWaitingForVisibility(t, a, false)
			switch reason {
			case "cancel":
				a.viewMu.Lock()
				a.stopLauncherItemsLocked()
				a.viewMu.Unlock()
			case "parent":
				core.retired.Store(true)
			case "child token":
				p.host.mu.Lock()
				p.host.current.panel = &launcherIntegrationPanel{token: 101}
				p.host.mu.Unlock()
			}
			panel.physical.Store(true)
			close(panel.release)
			select {
			case <-p.done:
			case <-time.After(3 * time.Second):
				t.Fatal("retired readiness owner did not drain")
			}
			if a.GetLauncherItemPanelState(p.state.Session) != nil || fs.calls.Load() != 0 || fs.opened.Load() != 0 {
				t.Fatal("retired readiness owner accessed its folder or returned")
			}
		})
	}
}

func TestLauncherChildLatePhysicalValidationCannotAdmitFolderAccess(t *testing.T) {
	fs := &childFolderFixture{entered: make(chan struct{}, 1)}
	a, _, _ := childAppFixture(t, fs)
	panel, p := childWaitingForVisibility(t, a, true)
	// The readiness timer started before this native validation entered.
	<-time.After(2100 * time.Millisecond)
	close(panel.release)
	select {
	case <-fs.entered:
		t.Fatal("native validation admitted folder access after the readiness deadline")
	case <-p.done:
	case <-time.After(3 * time.Second):
		t.Fatal("late validation did not retire and drain its child")
	}
	if a.GetLauncherItemPanelState(p.state.Session) != nil || panel.closes.Load() != 1 || fs.calls.Load() != 0 {
		t.Fatal("late validation retained its child or accessed its folder")
	}
}

func TestLauncherChildReplacementWaitsForReadAndRejectsOldResult(t *testing.T) {
	first := &childFolderFixture{entered: make(chan struct{}, 1), release: make(chan struct{})}
	second := &childFolderFixture{}
	a, _, _ := childAppFixture(t, first, second)
	old := showChild(t, a)
	<-first.entered
	next := showChild(t, a)
	if a.GetLauncherItemPanelState(old.Session) != nil {
		t.Fatal("retired child still readable")
	}
	time.Sleep(20 * time.Millisecond)
	if second.calls.Load() != 0 {
		t.Fatal("replacement queried before predecessor joined")
	}
	close(first.release)
	state := waitChildReady(t, a, next.Session)
	if state.Folder.FolderIdentity != fmt.Sprintf("launcher-child:%d", next.Session) {
		t.Fatal("native folder identity leaked")
	}
	if err := a.CloseLauncherItemPanel(old.Session, old.Revision); err == nil {
		t.Fatal("stale close accepted")
	}
}

func TestLauncherChildManagerCloseWaitsForCanceledNativeRead(t *testing.T) {
	fs := &childFolderFixture{entered: make(chan struct{}, 1), release: make(chan struct{})}
	a, source, _ := childAppFixture(t, fs)
	s := showChild(t, a)
	<-fs.entered
	a.viewMu.Lock()
	a.stopLauncherItemsLocked()
	a.viewMu.Unlock()
	source.mu.Lock()
	closed := source.closed
	source.mu.Unlock()
	if closed {
		t.Fatal("source closed during retained callback")
	}
	if a.GetLauncherItemPanelState(s.Session) != nil {
		t.Fatal("source replacement retained child")
	}
	close(fs.release)
	launcherEventually(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.closed })
}

func TestLauncherChildRelinkDuringEntryPreparationRefuses(t *testing.T) {
	fs := &childFolderFixture{}
	a, source, _ := childAppFixture(t, fs)
	s := waitChildReady(t, a, showChild(t, a).Session)
	fs.beforeGuard = func() {
		source.mu.Lock()
		r := source.records["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
		r.Revision = 3
		source.records[r.ID] = r
		source.mu.Unlock()
	}
	if err := a.OpenLauncherFolderEntry(s.Session, s.Revision, "entry"); err == nil || fs.opened.Load() != 0 {
		t.Fatal("relinked reference dispatched", err)
	}
}

func TestLauncherChildRapidReplacementCannotBypassAncestorDrain(t *testing.T) {
	first := &childFolderFixture{entered: make(chan struct{}, 1), release: make(chan struct{})}
	last := &childFolderFixture{}
	a, _, _ := childAppFixture(t, first, last)
	showChild(t, a)
	<-first.entered
	current := showChild(t, a)
	extra, err := a.ShowLauncherItemPanel(1, integrationDisplay, 1, 1, "folder")
	if err == nil {
		t.Error("accepted extra pending child")
		current = extra
	}
	time.Sleep(20 * time.Millisecond)
	early := last.calls.Load() != 0
	close(first.release)
	waitChildReady(t, a, current.Session)
	if early {
		t.Fatal("bypassed live ancestor drain")
	}
}

func TestLauncherChildRelinkWhileListBlockedDropsOldEntries(t *testing.T) {
	fs := &childFolderFixture{entered: make(chan struct{}, 1), release: make(chan struct{})}
	a, source, _ := childAppFixture(t, fs)
	s := showChild(t, a)
	<-fs.entered
	source.mu.Lock()
	r := source.records["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
	r.Revision = 3
	source.records[r.ID] = r
	source.mu.Unlock()
	close(fs.release)
	launcherEventually(t, func() bool {
		state := a.GetLauncherItemPanelState(s.Session)
		return state != nil && state.Folder.Status == "unavailable" && len(state.Folder.Entries) == 0
	})
}

func TestLauncherChildNativeParentIncarnationChangeRefuses(t *testing.T) {
	fs := &childFolderFixture{}
	a, _, _ := childAppFixture(t, fs)
	s := waitChildReady(t, a, showChild(t, a).Session)
	a.viewMu.Lock()
	p := a.launcherItemPanels.owners[1]
	d := p.parentHost.window
	a.viewMu.Unlock()
	d.mu.Lock()
	d.current.panel = &launcherIntegrationPanel{token: 100}
	d.mu.Unlock()
	if err := a.OpenLauncherFolderEntry(s.Session, s.Revision, "entry"); err == nil || fs.opened.Load() != 0 {
		t.Fatal("recreated parent admitted old child", err)
	}
}

func TestLauncherChildNativeChildIncarnationChangeRefuses(t *testing.T) {
	fs := &childFolderFixture{}
	a, _, _ := childAppFixture(t, fs)
	s := waitChildReady(t, a, showChild(t, a).Session)
	a.viewMu.Lock()
	d := a.launcherItemPanels.owners[1].host
	a.viewMu.Unlock()
	d.mu.Lock()
	d.current.panel = &launcherIntegrationPanel{token: 100}
	d.mu.Unlock()
	if err := a.OpenLauncherFolderEntry(s.Session, s.Revision, "entry"); err == nil || fs.opened.Load() != 0 {
		t.Fatal("recreated child admitted old lease", err)
	}
}

type childValidatingPanel struct {
	*launcherIntegrationPanel
	validate func()
}

func (p *childValidatingPanel) ValidateLauncherPanel(ctx context.Context, display string) error {
	if p.validate != nil {
		p.validate()
	}
	return p.launcherIntegrationPanel.ValidateLauncherPanel(ctx, display)
}

func TestLauncherChildRechecksNativeTokensAfterExternalValidation(t *testing.T) {
	fs := &childFolderFixture{}
	a, _, _ := childAppFixture(t, fs)
	s := waitChildReady(t, a, showChild(t, a).Session)
	a.viewMu.Lock()
	p := a.launcherItemPanels.owners[1]
	d := p.parentHost.window
	a.viewMu.Unlock()
	d.mu.Lock()
	d.current.panel = &childValidatingPanel{launcherIntegrationPanel: &launcherIntegrationPanel{token: 99}, validate: func() { d.mu.Lock(); d.current.panel = &launcherIntegrationPanel{token: 100}; d.mu.Unlock() }}
	d.mu.Unlock()
	if err := a.launcherChildGuard(p, true); err == nil {
		t.Fatal("token replacement during external validation accepted", s.Session)
	}
}

func TestLauncherChildPendingFloodRefusesWithoutAllocating(t *testing.T) {
	first := &childFolderFixture{entered: make(chan struct{}, 1), release: make(chan struct{})}
	a, _, _ := childAppFixture(t, first, &childFolderFixture{})
	showChild(t, a)
	<-first.entered
	pending := showChild(t, a)
	for range 100 {
		extra, err := a.ShowLauncherItemPanel(1, integrationDisplay, 1, 1, "folder")
		if !errors.Is(err, launcher.ErrBusy) {
			close(first.release)
			waitChildReady(t, a, extra.Session)
			t.Fatal("flood allocated extra pending child")
		}
	}
	a.viewMu.Lock()
	next := a.launcherItemPanels.next
	a.viewMu.Unlock()
	close(first.release)
	waitChildReady(t, a, pending.Session)
	if next != pending.Session {
		t.Fatal("refused commands consumed owner identities")
	}
}

func TestLauncherChildGlobalRetirementCapacityRefusesBeforeAllocation(t *testing.T) {
	a, _, _ := childAppFixture(t)
	// Retiring owners may span old parent sessions after Space/profile turnover.
	// Admission counts their retained native/source receipts, not the active map.
	a.viewMu.Lock()
	a.launcherItemPanels.retained = 16
	a.viewMu.Unlock()
	_, err := a.ShowLauncherItemPanel(1, integrationDisplay, 1, 1, "folder")
	a.viewMu.Lock()
	next := a.launcherItemPanels.next
	a.launcherItemPanels.retained = 0
	a.viewMu.Unlock()
	if !errors.Is(err, launcher.ErrBusy) || next != 0 {
		t.Fatal("global retiring-owner budget bypassed", err, next)
	}
}
