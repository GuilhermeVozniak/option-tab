package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

func windowChildFixture(t *testing.T) (*App, *previewActionPlatform, *childCoreFixture) {
	t.Helper()
	a, _, core := childAppFixture(t)
	p := &previewActionPlatform{appAutomationPlatform: &appAutomationPlatform{fake.New()}}
	p.ActiveAppID = 1
	p.ScreenList = []domain.Screen{{ID: 1, Main: true, Visible: domain.Bounds{W: 1440, H: 900}}}
	p.SetWindows([]domain.Window{{ID: 1, AppID: 1, Title: "Owned", BundleID: "test.fixture", AppName: "Fixture", SpaceID: 1, ScreenID: 1, OnScreen: true}, {ID: 2, AppID: 2, Title: "Other", OnScreen: true}})
	a.platform = p
	core.parent.Configured = config.LauncherItem{}
	core.parent.Reference = platform.LauncherReference{}
	core.parent.App = platform.LauncherAppTarget{Process: platform.ProcessIdentity{PID: 1, StartSeconds: 123}, BundleID: "test.fixture", Name: "Fixture"}
	return a, p, core
}

func waitWindowChild(t *testing.T, a *App, session uint64) LauncherItemPanelState {
	t.Helper()
	var s *LauncherItemPanelState
	launcherEventually(t, func() bool {
		s = a.GetLauncherItemPanelState(session)
		return s != nil && s.Windows != nil && len(s.Windows.Entries) > 0
	})
	return *s
}

func TestLauncherWindowChildExactInventoryCopiedAndScoped(t *testing.T) {
	a, _, _ := windowChildFixture(t)
	loading := showChild(t, a)
	if loading.Kind != "windows" || loading.Folder != nil {
		t.Fatalf("wrong loading kind %+v", loading)
	}
	s := waitWindowChild(t, a, loading.Session)
	if len(s.Windows.Entries) != 1 || s.Windows.Entries[0].WindowID != 1 {
		t.Fatalf("unowned inventory %+v", s.Windows.Entries)
	}
	s.Windows.Entries[0].Title = "changed"
	if got := a.GetLauncherItemPanelState(s.Session); got.Windows.Entries[0].Title == "changed" {
		t.Fatal("snapshot aliased entries")
	}
	if err := a.SelectLauncherWindow(s.Session, s.Revision, 2); err == nil {
		t.Fatal("unrendered selection admitted")
	}
	if err := a.SelectLauncherWindow(s.Session, s.Revision, 1); err != nil {
		t.Fatal(err)
	}
	if err := a.SelectLauncherWindow(s.Session, s.Revision, 1); err == nil {
		t.Fatal("old revision admitted")
	}
}

func TestLauncherWindowChildActionRetiresDuringNativePreparation(t *testing.T) {
	for _, change := range []string{"hide", "parent", "settings", "identity"} {
		t.Run(change, func(t *testing.T) {
			a, p, core := windowChildFixture(t)
			s := waitWindowChild(t, a, showChild(t, a).Session)
			p.prepare = func() {
				switch change {
				case "hide":
					a.viewMu.Lock()
					a.retireLauncherItemPanelLocked(1)
					a.viewMu.Unlock()
				case "parent":
					core.retired.Store(true)
				case "settings":
					v := a.settingsSnapshot()
					v.Appearance.ThumbnailMaxPx++
					a.settingsMu.Lock()
					a.settings = v
					a.settingsMu.Unlock()
				case "identity":
					a.viewMu.Lock()
					a.launcherItemPanels.owners[1].windows.identities[1] = platform.AutomationWindowIdentity{ID: 1}
					a.viewMu.Unlock()
				}
			}
			if err := a.PerformLauncherWindowAction(s.Session, s.Revision, "close", 1, false); err == nil {
				t.Fatal("retired action accepted")
			}
			if p.dispatched != 0 {
				t.Fatal("native mutation occurred after retirement")
			}
		})
	}
}

func TestLauncherWindowChildStoppedAppRefused(t *testing.T) {
	a, _, core := windowChildFixture(t)
	core.parent.App.Process = platform.ProcessIdentity{}
	if _, err := a.ShowLauncherItemPanel(1, integrationDisplay, 1, 1, "folder"); err == nil {
		t.Fatal("stopped app preview implicitly admitted")
	}
}

func TestLauncherWindowFramesIdentityBoundsCopiesAndRetirement(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	identity := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 4, StartSeconds: 10}}
	p := &launcherItemPanel{ctx: ctx, state: LauncherItemPanelState{Session: 7, Revision: 3, Open: true, Windows: &AutomationPreviewViewState{Entries: []switcher.Entry{{WindowID: 1}}, SelectedWindowID: 1}}}
	p.windows = &launcherWindowChild{capture: true, identities: map[domain.WindowID]platform.AutomationWindowIdentity{1: identity}, frames: map[domain.WindowID]string{}}
	var mu sync.Mutex
	var emitted []automationPreviewFrames
	a.eventSink = func(name string, data any) {
		if name == "launcher-item:frames" {
			mu.Lock()
			emitted = append(emitted, data.(automationPreviewFrames))
			mu.Unlock()
		}
	}
	a.publishLauncherWindowsLocked(p)
	wrong := identity
	wrong.Process.StartSeconds++
	a.emitLauncherWindowFrame(p, 1, "wrong", wrong)
	a.emitLauncherWindowFrame(p, 1, strings.Repeat("x", 512*1024+1), identity)
	a.emitLauncherWindowFrame(p, 1, "valid", identity)
	s := a.launcherWindowSnapshotLocked(p)
	if s.Frames[1] != "valid" || s.FrameSequence != 1 {
		t.Fatalf("bad frame state %+v", s)
	}
	s.Frames[1] = "mutated"
	if a.launcherWindowSnapshotLocked(p).Frames[1] != "valid" {
		t.Fatal("frames alias")
	}
	a.retireLauncherWindowCaptureLocked(p)
	a.emitLauncherWindowFrame(p, 1, "late", identity)
	mu.Lock()
	defer mu.Unlock()
	if len(emitted) != 1 || emitted[0].Session != 7 || emitted[0].Revision != 3 {
		t.Fatalf("unexpected events %+v", emitted)
	}
}

type blockedLauncherWindowSource struct {
	*previewActionPlatform
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (s *blockedLauncherWindowSource) Windows() ([]domain.Window, error) {
	if s.calls.Add(1) == 1 {
		close(s.entered)
		<-s.release
	}
	return s.previewActionPlatform.Windows()
}

func TestLauncherWindowReplacementJoinsCanceledInventory(t *testing.T) {
	a, p, _ := windowChildFixture(t)
	source := &blockedLauncherWindowSource{previewActionPlatform: p, entered: make(chan struct{}), release: make(chan struct{})}
	a.platform = source
	old := showChild(t, a)
	<-source.entered
	next := showChild(t, a)
	a.viewMu.Lock()
	successor := a.launcherItemPanels.owners[1]
	a.viewMu.Unlock()
	if successor.predecessor == nil {
		t.Fatal("replacement lost old source receipt")
	}
	select {
	case <-successor.predecessor:
		t.Fatal("receipt closed before blocked inventory exited")
	default:
	}
	if source.calls.Load() != 1 {
		t.Fatal("replacement read before predecessor joined")
	}
	if a.GetLauncherItemPanelState(old.Session) != nil {
		t.Fatal("old child remained admitted")
	}
	close(source.release)
	current := waitWindowChild(t, a, next.Session)
	if current.Session == old.Session || source.calls.Load() != 2 {
		t.Fatal("replacement did not resume after join")
	}
}

func TestLauncherWindowActionDispatchesOnlyExactRenderedIdentity(t *testing.T) {
	a, p, _ := windowChildFixture(t)
	s := waitWindowChild(t, a, showChild(t, a).Session)
	if err := a.PerformLauncherWindowAction(s.Session, s.Revision, "quit", 1, false); err == nil {
		t.Fatal("unadvertised app action accepted")
	}
	if err := a.PerformLauncherWindowAction(s.Session, s.Revision, "close", 2, false); err == nil {
		t.Fatal("other window accepted")
	}
	if p.dispatched != 0 {
		t.Fatal("refused command dispatched")
	}
	if err := a.PerformLauncherWindowAction(s.Session, s.Revision, "minimize", 1, false); err != nil {
		t.Fatal(err)
	}
	if p.dispatched != 1 {
		t.Fatal("exact action not dispatched")
	}
}

func TestLauncherWindowFramesSnapshotBudgetAndPriority(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &launcherItemPanel{ctx: ctx, state: LauncherItemPanelState{Session: 7, Revision: 3, Open: true, Windows: &AutomationPreviewViewState{SelectedWindowID: 35}}}
	p.windows = &launcherWindowChild{capture: true, identities: map[domain.WindowID]platform.AutomationWindowIdentity{}, frames: map[domain.WindowID]string{}}
	for i := domain.WindowID(1); i <= 35; i++ {
		p.state.Windows.Entries = append(p.state.Windows.Entries, switcher.Entry{WindowID: i})
		p.windows.identities[i] = platform.AutomationWindowIdentity{ID: i, Process: platform.ProcessIdentity{PID: 4, StartSeconds: 10}}
	}
	ids := launcherWindowCaptureIDs(p)
	if len(ids) != 30 || ids[0] != 35 {
		t.Fatalf("capture priorities %v", ids)
	}
	a.publishLauncherWindowsLocked(p)
	for _, id := range ids {
		a.emitLauncherWindowFrame(p, id, strings.Repeat("x", 512*1024), p.windows.identities[id])
	}
	s := a.launcherWindowSnapshotLocked(p)
	if len(s.Frames) != 8 {
		t.Fatalf("4MiB snapshot admitted %d full frames", len(s.Frames))
	}
	a.emitLauncherWindowFrame(p, 35, "", p.windows.identities[35])
	if _, ok := a.launcherWindowSnapshotLocked(p).Frames[35]; ok {
		t.Fatal("unavailable frame retained")
	}
}

func TestLauncherWindowRevisionUpdateIncludesCurrentFrameSnapshot(t *testing.T) {
	a := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	identity := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 4, StartSeconds: 10}}
	p := &launcherItemPanel{ctx: ctx, state: LauncherItemPanelState{Session: 7, Revision: 3, Open: true, Windows: &AutomationPreviewViewState{Entries: []switcher.Entry{{WindowID: 1}}, SelectedWindowID: 1}}}
	p.windows = &launcherWindowChild{capture: true, identities: map[domain.WindowID]platform.AutomationWindowIdentity{1: identity}, frames: map[domain.WindowID]string{}}
	a.publishLauncherWindowsLocked(p)
	a.emitLauncherWindowFrame(p, 1, "static-frame", identity)
	var update LauncherItemPanelState
	a.eventSink = func(name string, data any) {
		if name == "launcher-item:update" {
			update = data.(LauncherItemPanelState)
		}
	}
	a.publishLauncherChildLocked(p)
	if update.Windows.Frames[1] != "static-frame" || update.Windows.FrameSequence != 1 || update.Windows.Revision != 4 {
		t.Fatalf("revision update lost static capture %+v", update.Windows)
	}
}

func TestLauncherWindowFailedRefreshRetiresOldResult(t *testing.T) {
	a, p, _ := windowChildFixture(t)
	s := waitWindowChild(t, a, showChild(t, a).Session)
	p.WindowsErr = errors.New("incomplete inventory")
	if err := a.PerformLauncherWindowAction(s.Session, s.Revision, "minimize", 1, false); err != nil {
		t.Fatal(err)
	}
	var failed *LauncherItemPanelState
	launcherEventually(t, func() bool {
		failed = a.GetLauncherItemPanelState(s.Session)
		return failed != nil && failed.Error != ""
	})
	if len(failed.Windows.Entries) != 0 {
		t.Fatal("failed inventory retained old valid entries")
	}
	a.viewMu.Lock()
	owner := a.launcherItemPanels.owners[1]
	active := len(owner.windows.identities) > 0 || owner.windows.lease.Load() != nil
	a.viewMu.Unlock()
	if active {
		t.Fatal("failed inventory retained capture/identity admission")
	}
	if err := a.SelectLauncherWindow(failed.Session, failed.Revision, 1); err == nil {
		t.Fatal("old failed inventory admitted selection")
	}
}

type retiringWindowIdentitySource struct {
	*previewActionPlatform
	after func()
}

func (p *retiringWindowIdentitySource) WindowIdentityCurrent(id platform.AutomationWindowIdentity) bool {
	valid := p.previewActionPlatform.WindowIdentityCurrent(id)
	p.after()
	return valid
}

func TestLauncherWindowPhysicalRetireDuringIdentityRead(t *testing.T) {
	a, source, _ := windowChildFixture(t)
	state := waitWindowChild(t, a, showChild(t, a).Session)
	a.viewMu.Lock()
	owner := a.launcherItemPanels.owners[1]
	panel := owner.host.wheelPanel().(*launcherIntegrationPanel)
	identity := owner.windows.identities[1]
	a.viewMu.Unlock()
	a.platform = &retiringWindowIdentitySource{previewActionPlatform: source, after: func() { panel.unavailable.Store(true) }}
	if err := a.launcherWindowActionGuard(owner, state.Revision, identity); err == nil {
		t.Fatal("physical child retired during final identity read but guard accepted")
	}
}
