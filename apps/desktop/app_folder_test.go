package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

func folderEnabledSettings() config.Settings {
	s := config.Default()
	s.Dock.FolderPop.Enabled = true
	return s
}

type folderAppSource struct {
	list  func(context.Context, platform.FolderRef, platform.FolderSort) (platform.FolderListing, error)
	grant func(context.Context, platform.FolderRef) (platform.FolderGrant, error)
	open  func(context.Context, platform.FolderRef, string, func() error) error
}

func (s *folderAppSource) ListFolder(ctx context.Context, ref platform.FolderRef, sort platform.FolderSort) (platform.FolderListing, error) {
	if s.list != nil {
		return s.list(ctx, ref, sort)
	}
	return platform.FolderListing{}, errors.New("unexpected listing")
}

type folderAppObservation struct{ *fake.Fake }

func (p *folderAppObservation) ObserveDock(ctx context.Context, receive func(platform.DockObservation)) error {
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	var sequence uint64
	for {
		select {
		case at := <-tick.C:
			sequence++
			receive(platform.DockObservation{Generation: 1, Sequence: sequence, DockPID: 30, ObservedAt: at, Status: "ready", PointerX: 110, PointerY: 780, Item: &platform.DockItem{Kind: "folder", Path: "/tmp/folder-fixture", Title: "Folder", Bounds: domain.Bounds{X: 100, Y: 770, W: 40, H: 30}, ScreenID: 1, Edge: "bottom"}})
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func waitFolderView(t *testing.T, a *App, predicate func(*DockViewState) bool) *DockViewState {
	t.Helper()
	deadline := time.After(2 * time.Second)
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		view := a.GetDockState()
		if predicate(view) {
			return view
		}
		select {
		case <-tick.C:
		case <-deadline:
			t.Fatalf("folder state did not arrive: %+v", view)
			return nil
		}
	}
}

func TestDockFolderGrantRefreshesAfterSamePresentationResizes(t *testing.T) {
	var granted atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	source := &folderAppSource{
		list: func(_ context.Context, ref platform.FolderRef, _ platform.FolderSort) (platform.FolderListing, error) {
			if !granted.Load() {
				return platform.FolderListing{FolderIdentity: ref.Identity, Status: "permissionRequired"}, nil
			}
			return platform.FolderListing{FolderIdentity: ref.Identity, Status: "ready", Entries: []platform.FolderEntry{{ID: "after-grant", Name: "Granted.txt", Kind: "file"}}}, nil
		},
		grant: func(ctx context.Context, ref platform.FolderRef) (platform.FolderGrant, error) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return platform.FolderGrant{}, ctx.Err()
			}
			granted.Store(true)
			return platform.FolderGrant{FolderIdentity: ref.Identity, Status: "ready"}, nil
		},
	}
	p := &folderAppObservation{fake.New()}
	p.ScreenList = []domain.Screen{{ID: 1, Main: true, Bounds: domain.Bounds{W: 1200, H: 800}}}
	s := folderEnabledSettings()
	s.Dock.HoverDelayMs = 0
	a := newApp(p, s, "", source)
	defer a.stopCapture()
	a.startDock()
	before := waitFolderView(t, a, func(v *DockViewState) bool { return v.Folder != nil && v.Folder.Status == "permissionRequired" })
	done := make(chan error, 1)
	go func() { done <- a.RequestDockFolderAccess(before.Session, before.Revision) }()
	<-entered
	a.SetDockPanelSize(before.Session, 410, 210)
	after := waitFolderView(t, a, func(v *DockViewState) bool { return v.Revision > before.Revision })
	if after.Folder.Revision != before.Folder.Revision {
		t.Fatal("geometry changed logical folder contents")
	}
	close(release)
	if err := awaitFolderResult(t, done); err != nil {
		t.Fatal(err)
	}
	ready := waitFolderView(t, a, func(v *DockViewState) bool { return v.Folder != nil && v.Folder.Status == "ready" })
	if ready.Session != before.Session || len(ready.Folder.Entries) != 1 || ready.Folder.Entries[0].ID != "after-grant" {
		t.Fatal("accepted grant did not refresh original folder presentation")
	}
}

func (s *folderAppSource) RequestFolderAccess(ctx context.Context, ref platform.FolderRef) (platform.FolderGrant, error) {
	return s.grant(ctx, ref)
}

func (s *folderAppSource) OpenFolderEntryGuarded(ctx context.Context, ref platform.FolderRef, id string, guard func() error) error {
	return s.open(ctx, ref, id, guard)
}

func appFolderState(session uint64) dock.State {
	return dock.State{Session: session, ContentKind: "folder", Item: dock.Item{Kind: "folder", Path: "/tmp/folder-fixture", Title: "Folder"}, Folder: &dock.FolderState{
		Revision: 1, FolderIdentity: "file:///tmp/folder-fixture", Status: "ready",
		Entries: []platform.FolderEntry{{ID: "opaque-item", Name: "Example.txt", Kind: "file"}},
		Sort:    platform.FolderSort{Field: "name", Direction: "asc", FoldersFirst: true},
	}, Appearance: config.Default().Dock.Appearance}
}

func awaitFolderResult(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("folder operation did not finish")
		return nil
	}
}

func TestDockFolderOpenUsesOpaqueItemAndRevalidatesBeforeDispatch(t *testing.T) {
	for _, retire := range []string{"hide", "revision", "identity", "item", "disable", "pause", "inactive", "shutdown"} {
		t.Run(retire, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var dispatched atomic.Int32
			source := &folderAppSource{open: func(ctx context.Context, ref platform.FolderRef, id string, guard func() error) error {
				if ref.Identity != "file:///tmp/folder-fixture" || ref.Path != "/tmp/folder-fixture" || id != "opaque-item" {
					return errors.New("incorrect captured target")
				}
				close(entered)
				<-release
				if err := guard(); err != nil {
					return err
				}
				dispatched.Add(1)
				return ctx.Err()
			}}
			a := newApp(fake.New(), folderEnabledSettings(), "", source)
			defer a.stopCapture()
			st := appFolderState(1)
			a.showDock(st, true)
			done := make(chan error, 1)
			go func() { done <- a.OpenDockFolderEntry(1, a.GetDockState().Revision, "opaque-item") }()
			<-entered
			switch retire {
			case "hide":
				a.hideDock(1)
			case "revision":
				a.showDock(st, false)
			case "identity":
				st.Item.Path = "/tmp/replacement"
				st.Folder.FolderIdentity = "file:///tmp/replacement"
				a.showDock(st, false)
			case "item":
				st.Folder.Entries = nil
				a.showDock(st, false)
			case "disable":
				s := folderEnabledSettings()
				s.Dock.FolderPop.Enabled = false
				if err := saveFolderSettings(a, s); err != nil {
					t.Fatal(err)
				}
			case "pause":
				a.SetPaused(true)
			case "inactive":
				a.setSessionInactive(true)
			case "shutdown":
				a.stopCapture()
			}
			close(release)
			if err := awaitFolderResult(t, done); err == nil || dispatched.Load() != 0 {
				t.Fatalf("stale open dispatched: error=%v count=%d", err, dispatched.Load())
			}
		})
	}
}

func TestDockFolderRejectsInvalidRPCScopesBeforeIO(t *testing.T) {
	var calls atomic.Int32
	source := &folderAppSource{
		grant: func(context.Context, platform.FolderRef) (platform.FolderGrant, error) {
			calls.Add(1)
			return platform.FolderGrant{}, nil
		},
		open: func(context.Context, platform.FolderRef, string, func() error) error { calls.Add(1); return nil },
	}
	a := newApp(fake.New(), folderEnabledSettings(), "", source)
	defer a.stopCapture()
	a.showDock(appFolderState(1), true)
	revision := a.GetDockState().Revision
	for _, scope := range [][2]uint64{{0, revision}, {2, revision}, {1, 0}, {1, revision + 1}} {
		if err := a.RequestDockFolderAccess(scope[0], scope[1]); err == nil {
			t.Fatal("stale access admitted")
		}
		if err := a.OpenDockFolderEntry(scope[0], scope[1], "opaque-item"); err == nil {
			t.Fatal("stale open admitted")
		}
		if err := a.SetDockFolderSort(scope[0], scope[1], "name", "asc", true); err == nil {
			t.Fatal("stale sort admitted")
		}
	}
	if err := a.OpenDockFolderEntry(1, revision, "/tmp/arbitrary-file"); err == nil {
		t.Fatal("frontend path admitted as opaque item")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid scope reached folder service")
	}
}

func TestDockFolderGrantSurvivesHoverHideButCannotUpdateReplacement(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var granted atomic.Bool
	source := &folderAppSource{grant: func(ctx context.Context, ref platform.FolderRef) (platform.FolderGrant, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return platform.FolderGrant{}, ctx.Err()
		}
		granted.Store(true)
		return platform.FolderGrant{FolderIdentity: ref.Identity, Status: "ready"}, nil
	}}
	a := newApp(fake.New(), folderEnabledSettings(), "", source)
	defer a.stopCapture()
	a.showDock(appFolderState(1), true)
	revision := a.GetDockState().Revision
	done := make(chan error, 1)
	go func() { done <- a.RequestDockFolderAccess(1, revision) }()
	<-entered
	a.hideDock(1)
	a.showDock(appFolderState(2), true)
	current := a.GetDockState().Revision
	if err := a.RequestDockFolderAccess(2, current); err == nil {
		t.Fatal("second chooser admitted while accepted grant active")
	}
	a.CancelDockFolderAccess(2, current)
	close(release)
	if err := awaitFolderResult(t, done); err != nil || !granted.Load() {
		t.Fatalf("ordinary hover retirement cancelled accepted grant: %v", err)
	}
	if state := a.GetDockState(); state.Session != 2 || state.Revision != current {
		t.Fatal("late grant changed replacement presentation")
	}
}

func TestDockFolderGrantCancelsOnExplicitLifecycleRetirement(t *testing.T) {
	for _, retire := range []string{"cancel", "disable", "pause", "inactive", "shutdown"} {
		t.Run(retire, func(t *testing.T) {
			entered := make(chan struct{})
			source := &folderAppSource{grant: func(ctx context.Context, _ platform.FolderRef) (platform.FolderGrant, error) {
				close(entered)
				<-ctx.Done()
				return platform.FolderGrant{}, ctx.Err()
			}}
			a := newApp(fake.New(), folderEnabledSettings(), "", source)
			defer a.stopCapture()
			a.showDock(appFolderState(1), true)
			revision := a.GetDockState().Revision
			done := make(chan error, 1)
			go func() { done <- a.RequestDockFolderAccess(1, revision) }()
			<-entered
			switch retire {
			case "cancel":
				a.hideDock(1)
				a.CancelDockFolderAccess(1, revision)
			case "disable":
				s := folderEnabledSettings()
				s.Dock.FolderPop.Enabled = false
				if err := saveFolderSettings(a, s); err != nil {
					t.Fatal(err)
				}
			case "pause":
				a.SetPaused(true)
			case "inactive":
				a.setSessionInactive(true)
			case "shutdown":
				a.stopCapture()
			}
			if err := awaitFolderResult(t, done); !errors.Is(err, context.Canceled) {
				t.Fatalf("grant not cancelled: %v", err)
			}
		})
	}
}

func TestDockFolderViewCopiesEntriesAndExposesNoOpenPath(t *testing.T) {
	a := newApp(fake.New(), folderEnabledSettings(), "")
	defer a.stopCapture()
	st := appFolderState(1)
	a.showDock(st, true)
	st.Folder.Entries[0].Name = "mutated input"
	view := a.GetDockState()
	if view.ContentKind != "folder" || view.Folder == nil || view.Folder.Entries[0].Name != "Example.txt" || len(view.Entries) != 0 || view.Item.Path != "" {
		t.Fatalf("incorrect folder DTO: %+v", view)
	}
	view.Folder.Entries[0].Name = "mutated output"
	if a.GetDockState().Folder.Entries[0].Name != "Example.txt" {
		t.Fatal("folder DTO aliases app state")
	}
}

func TestDockFolderAdmissionIsIndependentAndNeverEnablesAppActions(t *testing.T) {
	a := newApp(fake.New(), folderEnabledSettings(), "")
	defer a.stopCapture()
	st := dockFixtureState(1, 101, 10)
	a.showDock(st, true)
	if a.GetDockState().Open {
		t.Fatal("Folder Pop enabled an app preview while window previews are disabled")
	}
	st.Session = 2
	st.Item = dock.Item{Kind: "folder", Path: "/tmp/folder-fixture", Title: "Folder"}
	st.Windows = nil
	a.showDock(st, true)
	if !a.GetDockState().Open {
		t.Fatal("Folder Pop could not show independently of window previews")
	}
	if a.captureDockSession.Load() != 0 || a.GetDockState().PreviewDragEnabled {
		t.Fatal("folder presentation acquired capture or preview drag ownership")
	}
	if err := a.SetDockPreviewRegions(2, a.GetDockState().Revision, nil); err == nil {
		t.Fatal("folder presentation admitted app gesture regions")
	}
}

func saveFolderSettings(a *App, s config.Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return a.SaveSettings(string(data))
}

func TestDockFolderErrorsSurviveGeometryAndUnrelatedInputStatus(t *testing.T) {
	fail := true
	source := &folderAppSource{open: func(_ context.Context, _ platform.FolderRef, _ string, guard func() error) error {
		if err := guard(); err != nil {
			return err
		}
		if fail {
			return errors.New("fixture open failed")
		}
		return nil
	}}
	a := newApp(fake.New(), folderEnabledSettings(), "", source)
	defer a.stopCapture()
	a.setDockInputError(errors.New("unrelated icon failure"))
	st := appFolderState(1)
	a.showDock(st, true)
	if a.GetDockState().Error != "" {
		t.Fatal("folder inherited unrelated app gesture failure")
	}
	if err := a.OpenDockFolderEntry(1, a.GetDockState().Revision, "opaque-item"); err == nil {
		t.Fatal("fixture open unexpectedly succeeded")
	}
	revision := a.GetDockState().Revision
	a.setDockInputError(nil)
	if view := a.GetDockState(); view.Revision != revision || view.Error != "fixture open failed" {
		t.Fatal("unrelated input recovery cleared folder failure")
	}
	st.Bounds.W = 400
	a.showDock(st, false)
	if a.GetDockState().Error != "fixture open failed" {
		t.Fatal("geometry update cleared folder failure")
	}
	fail = false
	if err := a.OpenDockFolderEntry(1, a.GetDockState().Revision, "opaque-item"); err != nil {
		t.Fatal(err)
	}
	if a.GetDockState().Error != "" {
		t.Fatal("successful retry retained stale folder error")
	}
}
