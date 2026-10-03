package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// Exercise the real launcher core→App publication path. The native host and
// filesystem are disposable fakes; no actual display, folder or input changes.
func TestLauncherChildEnvironmentRetirementClosesHostThroughCorePublication(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	ref := platform.LauncherReference{ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Kind: "folder", Label: "Folder", State: "ready", Revision: 1}
	refs := &childReferenceFixture{launcherReferenceFixture: launcherReferenceFixture{records: map[string]platform.LauncherReference{ref.ID: ref}}}
	a.wireLauncherItems(refs, nil)
	previous := a.settingsSnapshot()
	a.settingsMu.Lock()
	a.settings.ReplacementDock.Profiles[0].Items = []config.LauncherItem{{ID: "folder", Kind: "folder", Label: "Folder", ReferenceID: ref.ID, FolderView: "list"}}
	a.settingsMu.Unlock()
	a.configureLauncher(previous, a.settingsSnapshot())
	a.launcherItemPanels.folderFactory = func() platform.FolderSource { return &childFolderFixture{} }
	childPanel := &launcherIntegrationPanel{token: 101}
	a.launcherItemPanelFactory = func(_ uint64, uuid string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		w := &dockFakeWindow{}
		w.native = unsafe.Pointer(new(int))
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, launcherPanelHostAdapter{source: launcherIntegrationHost{childPanel}, uuid: uuid, style: style})
		d.onFailure = failed
		return d
	}
	var hidden atomic.Bool
	a.eventSink = func(name string, _ any) {
		if name == "launcher-item:hide" {
			hidden.Store(true)
		}
	}
	parent := launcherIntegrationVisible(t, a, q)
	ctx, cancel := context.WithCancel(context.Background())
	uiDone := make(chan struct{})
	go func() {
		defer close(uiDone)
		for {
			select {
			case f := <-q:
				f()
			case <-ctx.Done():
				return
			}
		}
	}()
	t.Cleanup(func() {
		a.stopCapture()
		<-a.launcherItems.done
		cancel()
		<-uiDone
	})
	state, err := a.ShowLauncherItemPanel(parent.Epoch, parent.DisplayUUID, parent.Session, parent.Revision, "pin:folder")
	if err != nil {
		t.Fatal(err)
	}
	waitChildReady(t, a, state.Session)
	backend.mu.Lock()
	backend.sequence += 2
	sequence, emit := backend.sequence, backend.emit
	backend.mu.Unlock()
	e := platform.LauncherEnvironment{Generation: 1, Sequence: sequence - 1, ObservedAt: time.Now(), Complete: true, Status: "ready", PointerKnown: true, PointerX: 100, PointerY: 100, NativeDock: platform.LauncherNativeDock{Process: platform.ProcessIdentity{PID: 1, StartSeconds: 1}, Edge: "bottom", Visibility: "hidden", Confidence: "known", Bounds: domain.Bounds{X: 400, Y: 780, W: 200, H: 20}}, Displays: []platform.LauncherDisplay{{UUID: integrationDisplay, Main: true, Frame: domain.Bounds{W: 1000, H: 800}, UsableFrame: domain.Bounds{Y: 25, W: 1000, H: 775}, Scale: 2, SpaceID: 2, SpaceKind: "ordinary", SpaceStatus: "known"}}}
	emit(e)
	e.Sequence = sequence
	e.Displays[0].SpaceID = 1
	emit(e)
	launcherEventually(t, func() bool { return hidden.Load() && childPanel.closes.Load() == 1 })
	if a.GetLauncherItemPanelState(state.Session) != nil {
		t.Fatal("retired child still exposed")
	}
}
