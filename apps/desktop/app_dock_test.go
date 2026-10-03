package main

import (
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
)

func dockFixtureState(session uint64, id domain.WindowID, app domain.AppID) dock.State {
	return dock.State{Session: session, Item: dock.Item{Kind: "app", AppID: app, Title: "Fixture", BundleID: "fixture.app"}, Windows: []domain.Window{{ID: id, AppID: app, Title: "Fixture window", BundleID: "fixture.app"}}, SelectedWindowID: id, Bounds: domain.Bounds{X: 100, Y: 100, W: 300, H: 200}, Appearance: config.Default().Dock.Appearance}
}

func dockEnabledSettings() config.Settings { s := config.Default(); s.Dock.Enabled = true; return s }

func TestDockRuntimeRejectsPendingShowFromBeforeRapidSuspendResume(t *testing.T) {
	a := newApp(fake.New(), dockEnabledSettings(), "")
	defer a.stopCapture()
	a.dockController = dock.NewController(dock.Deps{}, dockEnabledSettings())
	st := dockFixtureState(1, 101, 10)
	st.AdmissionEpoch = a.dockController.AdmissionEpoch()
	a.dockController.Suspend(true)
	a.dockController.Suspend(false)
	a.showDock(st, true)
	if a.dockState.Session != 0 || a.captureDockSession.Load() != 0 {
		t.Fatal("retired pending Dock presentation reopened after resume")
	}
	st.Session = 2
	st.AdmissionEpoch = a.dockController.AdmissionEpoch()
	a.showDock(st, true)
	if a.dockState.Session != 2 {
		t.Fatal("new epoch could not open Dock presentation")
	}
}

func TestDockPointerPacketsAreScopedAndRetainedForLateViews(t *testing.T) {
	a := newApp(fake.New(), dockEnabledSettings(), "")
	defer a.stopCapture()
	a.dockController = dock.NewController(dock.Deps{}, dockEnabledSettings())
	st := dockFixtureState(1, 101, 10)
	st.AdmissionEpoch = a.dockController.AdmissionEpoch()
	a.showDock(st, true)
	packets := 0
	a.eventSink = func(name string, data any) {
		if name == "dock:pointer" {
			packets++
		}
	}
	point := dock.PointerState{Session: 1, AdmissionEpoch: st.AdmissionEpoch, Sequence: 1, X: 20, Y: 30, Inside: true}
	a.moveDockPointer(point)
	a.moveDockPointer(point)
	a.showDock(st, false)
	snapshot := a.GetDockState()
	if packets != 1 || snapshot.Pointer == nil || snapshot.Pointer.Sequence != 1 || snapshot.Pointer.X != 20 {
		t.Fatalf("pointer was lost or replayed: packets=%d snapshot=%+v", packets, snapshot)
	}
	snapshot.Pointer.X = 999
	if a.GetDockState().Pointer.X != 20 {
		t.Fatal("snapshot aliases live pointer state")
	}
	a.hideDock(1)
	st.Session = 2
	a.showDock(st, true)
	point.Sequence++
	a.moveDockPointer(point)
	if a.GetDockState().Pointer != nil || packets != 1 {
		t.Fatal("retired pointer reached new presentation")
	}
	point.Session = 2
	a.dockController.Suspend(true)
	a.dockController.Suspend(false)
	a.moveDockPointer(point)
	if a.GetDockState().Pointer != nil || packets != 1 {
		t.Fatal("invalidated epoch delivered pointer")
	}
}

func TestDockRuntimeSwitcherWinsAndCaptureRoutingChangesAfterCancellation(t *testing.T) {
	p := &lifecycleStreamPlatform{Fake: fake.New(), started: make(chan domain.WindowID, 8), stopped: make(chan domain.WindowID, 8)}
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	a.showDock(dockFixtureState(7, 10, 1), true)
	if id := <-p.started; id != 10 {
		t.Fatalf("Dock capture=%d", id)
	}
	a.Show(switcher.State{Open: true, Style: config.StyleThumbnails, Appearance: config.Default().Appearance, Entries: []switcher.Entry{{WindowID: 20, AppID: 2}}})
	if id := <-p.stopped; id != 10 {
		t.Fatalf("prior Dock capture not cancelled: %d", id)
	}
	if id := <-p.started; id != 20 {
		t.Fatalf("switcher capture=%d", id)
	}
	if a.dockState.Session != 0 || a.captureDockSession.Load() != 0 {
		t.Fatal("Dock retained capture ownership")
	}
	a.showDock(dockFixtureState(8, 30, 3), true)
	select {
	case id := <-p.started:
		t.Fatalf("Dock reopened over switcher, window %d", id)
	case <-time.After(30 * time.Millisecond):
	}
}

func TestDockRuntimePreferencesPauseAndShutdownRejectShow(t *testing.T) {
	for _, reason := range []string{"preferences", "pause", "disabled", "locked", "shutdown"} {
		t.Run(reason, func(t *testing.T) {
			a := newApp(fake.New(), dockEnabledSettings(), "")
			defer a.stopCapture()
			a.showDock(dockFixtureState(1, 101, 10), true)
			switch reason {
			case "preferences":
				a.OpenPreferences()
			case "pause":
				a.SetPaused(true)
			case "disabled":
				s := dockEnabledSettings()
				s.Dock.Enabled = false
				a.saveMu.Lock()
				err := a.saveSettingsLocked(s)
				a.saveMu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			case "locked":
				a.setSessionInactive(true)
			case "shutdown":
				a.stopCapture()
			}
			a.showDock(dockFixtureState(2, 102, 10), true)
			if a.dockState.Session != 0 {
				t.Fatalf("Dock shown during %s", reason)
			}
		})
	}
}

func TestDockActionsValidateSessionAndDisplayedTargets(t *testing.T) {
	p := fake.New()
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "First"}, {ID: 102, AppID: 10, Title: "Second"}, {ID: 201, AppID: 20, Title: "Other"}})
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	st := dockFixtureState(7, 101, 10)
	st.Windows = append(st.Windows, domain.Window{ID: 102, AppID: 10, Title: "Second"})
	st.SelectedWindowID = 102
	a.showDock(st, true)
	for _, target := range []struct {
		session, id uint64
		app         int
	}{{6, 101, 10}, {7, 201, 20}, {7, 201, 10}, {7, 101, 20}} {
		if _, err := a.PerformDockAction(target.session, "minimize", target.id, target.app); err == nil {
			t.Fatalf("invalid target accepted: %+v", target)
		}
	}
	if len(p.MinimizeCalls) != 0 {
		t.Fatal("rejected Dock action reached platform")
	}
	result, err := a.PerformDockAction(7, "minimize", 101, 10)
	if err != nil || result.Succeeded != 1 || len(p.MinimizeCalls) != 1 || p.MinimizeCalls[0] != 101 {
		t.Fatalf("explicit action result=%+v error=%v calls=%v", result, err, p.MinimizeCalls)
	}
	if _, err := a.FocusDockWindow(7, 101, 10); err != nil {
		t.Fatal(err)
	}
	if a.dockState.Session != 0 || len(p.FocusCalls) != 1 || p.FocusCalls[0] != 101 {
		t.Fatal("focus did not dismiss exact session")
	}
}

type delayedDockFocus struct {
	*fake.Fake
	entered, release chan struct{}
}

type delayedDockInventory struct {
	*fake.Fake
	entered, release chan struct{}
}

func (p *delayedDockInventory) Windows() ([]domain.Window, error) {
	close(p.entered)
	<-p.release
	return p.Fake.Windows()
}

func TestDockActionCannotDispatchAfterSessionRetiresDuringLookup(t *testing.T) {
	p := &delayedDockInventory{Fake: fake.New(), entered: make(chan struct{}), release: make(chan struct{})}
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "First"}})
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	a.showDock(dockFixtureState(7, 101, 10), true)
	done := make(chan error, 1)
	go func() { _, err := a.PerformDockAction(7, "minimize", 101, 10); done <- err }()
	<-p.entered
	a.hideDock(7)
	a.showDock(dockFixtureState(8, 101, 10), true)
	close(p.release)
	if err := <-done; err == nil {
		t.Fatal("stale action reported success")
	}
	if len(p.MinimizeCalls) != 0 {
		t.Fatal("stale action reached the platform")
	}
}

func (p *delayedDockFocus) Focus(id domain.WindowID) error {
	close(p.entered)
	<-p.release
	return p.Fake.Focus(id)
}

func TestDelayedDockFocusCannotHideReplacementPanel(t *testing.T) {
	p := &delayedDockFocus{Fake: fake.New(), entered: make(chan struct{}), release: make(chan struct{})}
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "First"}, {ID: 201, AppID: 20, Title: "Other"}})
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	a.showDock(dockFixtureState(7, 101, 10), true)
	done := make(chan error, 1)
	go func() { _, err := a.FocusDockWindow(7, 101, 10); done <- err }()
	<-p.entered
	a.showDock(dockFixtureState(8, 201, 20), true)
	close(p.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if a.dockState.Session != 8 {
		t.Fatal("old focus completion hid replacement panel")
	}
}
