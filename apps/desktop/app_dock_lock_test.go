package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type appMonitorLockPlatform struct {
	*fake.Fake
	policies chan platform.DockMonitorLockPolicy
	stopped  chan struct{}
	places   chan platform.DockPlacementRequest
}

func (p *appMonitorLockPlatform) DockPlacementAvailable() bool { return true }

type monitorWithoutPlacement struct {
	*fake.Fake
	source *appMonitorLockPlatform
}

func (p *monitorWithoutPlacement) DockMonitorLockDisplays(ctx context.Context) ([]platform.DockLockDisplay, error) {
	return p.source.DockMonitorLockDisplays(ctx)
}

func (p *monitorWithoutPlacement) ObserveDockMonitorLock(ctx context.Context, policy platform.DockMonitorLockPolicy, emit func(platform.DockMonitorLockState)) error {
	return p.source.ObserveDockMonitorLock(ctx, policy, emit)
}

func (p *monitorWithoutPlacement) PlaceDock(ctx context.Context, request platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
	return p.source.PlaceDock(ctx, request)
}

func (p *appMonitorLockPlatform) DockMonitorLockDisplays(context.Context) ([]platform.DockLockDisplay, error) {
	return []platform.DockLockDisplay{{UUID: "fixture-screen", Name: "Fixture", Main: true}}, nil
}

func (p *appMonitorLockPlatform) ObserveDockMonitorLock(ctx context.Context, policy platform.DockMonitorLockPolicy, changed func(platform.DockMonitorLockState)) error {
	p.policies <- policy
	changed(platform.DockMonitorLockState{Session: policy.Session, Revision: policy.Revision, Generation: 10, Sequence: 1, Status: "awaitingPlacement", TargetUUID: "fixture-screen"})
	<-ctx.Done()
	p.stopped <- struct{}{}
	return ctx.Err()
}

func (p *appMonitorLockPlatform) PlaceDock(ctx context.Context, request platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
	p.places <- request
	<-ctx.Done()
	return platform.DockPlacementResult{}, ctx.Err()
}

func monitorLockApp(t *testing.T, enabled bool) (*App, *appMonitorLockPlatform) {
	t.Helper()
	p := &appMonitorLockPlatform{Fake: fake.New(), policies: make(chan platform.DockMonitorLockPolicy, 4), stopped: make(chan struct{}, 4), places: make(chan platform.DockPlacementRequest, 4)}
	s := config.Default()
	s.Dock.MonitorLock.Enabled = enabled
	a := newApp(p, s, "")
	t.Cleanup(a.stopCapture)
	a.startDock()
	return a, p
}

func waitMonitorLockState(t *testing.T, a *App, status string) platform.DockMonitorLockState {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		state := a.GetDockMonitorLockState()
		if state.Status == status {
			return state
		}
		select {
		case <-deadline:
			t.Fatalf("monitor lock status=%q, want %q", state.Status, status)
		case <-ticker.C:
		}
	}
}

func TestMonitorLockDefaultOffStillListsDisplays(t *testing.T) {
	a, p := monitorLockApp(t, false)
	if state := a.GetDockMonitorLockState(); state.Status != "disabled" {
		t.Fatalf("default=%+v", state)
	}
	displays, err := a.GetDockMonitorLockDisplays()
	if err != nil || len(displays) != 1 || displays[0].Name != "Fixture" {
		t.Fatalf("inventory=%v error=%v", displays, err)
	}
	if _, err := a.PlaceDockOnSelectedMonitor(0, 0, 0); err == nil {
		t.Fatal("disabled placement was admitted")
	}
	select {
	case <-p.policies:
		t.Fatal("display inventory installed a native filter")
	default:
	}
}

func TestMonitorLockIndependentOfPreviewsAndPreferences(t *testing.T) {
	a, p := monitorLockApp(t, true)
	<-p.policies
	waitMonitorLockState(t, a, "awaitingPlacement")
	a.viewMu.Lock()
	a.prefsOpen, a.switcherVisible = true, true
	a.syncDockSuspensionLocked()
	a.viewMu.Unlock()
	if state := a.GetDockMonitorLockState(); state.Status != "awaitingPlacement" {
		t.Fatalf("preferences retired independent lock: %+v", state)
	}
	select {
	case <-p.stopped:
		t.Fatal("preview suspension stopped monitor protection")
	default:
	}
	a.setSessionInactive(true)
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("session inactivity left the native owner running")
	}
	if a.GetDockMonitorLockState().Status != "disabled" {
		t.Fatal("inactive session retained lock admission")
	}
}

func TestMonitorLockPlacementCancelledByPauseAndExplicitCancel(t *testing.T) {
	for _, pause := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "pause"}[pause], func(t *testing.T) {
			a, p := monitorLockApp(t, true)
			policy := <-p.policies
			state := waitMonitorLockState(t, a, "awaitingPlacement")
			done := make(chan error, 1)
			go func() {
				_, err := a.PlaceDockOnSelectedMonitor(state.Session, state.Revision, state.Generation)
				done <- err
			}()
			select {
			case request := <-p.places:
				if request.Session != policy.Session || request.Revision != policy.Revision || request.Generation != 10 || request.RequestID == 0 {
					t.Fatalf("placement lost exact native scope: %+v", request)
				}
			case <-time.After(time.Second):
				t.Fatal("placement was not admitted")
			}
			if pause {
				a.SetPaused(true)
			} else {
				a.CancelDockPlacement()
			}
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) && !errors.Is(err, dock.ErrMonitorLockRetired) {
					t.Fatalf("cancel error=%v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel did not reach native placement")
			}
		})
	}
}

func TestMonitorLockShutdownClosesAdmissionImmediately(t *testing.T) {
	a, p := monitorLockApp(t, true)
	<-p.policies
	state := waitMonitorLockState(t, a, "awaitingPlacement")
	a.stopCapture()
	if _, err := a.PlaceDockOnSelectedMonitor(state.Session, state.Revision, state.Generation); err == nil {
		t.Fatal("shutdown admitted placement")
	}
	select {
	case <-p.stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not stop the native owner")
	}
}

func TestMonitorLockPublishedTargetChangeRefusesEarlierClick(t *testing.T) {
	a, p := monitorLockApp(t, true)
	<-p.policies
	state := waitMonitorLockState(t, a, "awaitingPlacement")
	// Reproduce the interval after SaveSettings publishes but before its Dock
	// configure hook. The earlier UI snapshot still names the original target.
	a.settingsMu.Lock()
	a.settings.Dock.MonitorLock.Target = "display"
	a.settings.Dock.MonitorLock.DisplayUUID = "aabbccdd-0000-0000-0000-000000000001"
	a.settingsMu.Unlock()
	if _, err := a.PlaceDockOnSelectedMonitor(state.Session, state.Revision, state.Generation); err == nil {
		t.Fatal("old click adopted a newly published monitor target")
	}
	select {
	case request := <-p.places:
		t.Fatalf("stale click reached native placement: %+v", request)
	default:
	}
}

func TestMonitorLockUnprovenPlacementCapabilityNeverReachesNative(t *testing.T) {
	p := &appMonitorLockPlatform{Fake: fake.New(), policies: make(chan platform.DockMonitorLockPolicy, 4), stopped: make(chan struct{}, 4), places: make(chan platform.DockPlacementRequest, 4)}
	s := config.Default()
	s.Dock.MonitorLock.Enabled = true
	a := newApp(&monitorWithoutPlacement{Fake: p.Fake, source: p}, s, "")
	defer a.stopCapture()
	a.startDock()
	<-p.policies
	state := waitMonitorLockState(t, a, "awaitingPlacement")
	if state.PlacementAvailable {
		t.Fatal("missing capability advertised automatic placement")
	}
	if _, err := a.PlaceDockOnSelectedMonitor(state.Session, state.Revision, state.Generation); err == nil {
		t.Fatal("unproven placement was admitted")
	}
	select {
	case request := <-p.places:
		t.Fatalf("unproven placement reached native: %+v", request)
	default:
	}
}
