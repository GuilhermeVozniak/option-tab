package main

import (
	"context"
	"errors"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
)

func (a *App) wireDockMonitorLock() {
	source, ok := a.platform.(platform.DockMonitorLockSource)
	if !ok {
		return
	}
	a.dockMonitorLock = dock.NewMonitorLockController(dock.MonitorLockControllerDeps{Source: source, Changed: a.acceptDockMonitorLockState})
	a.viewMu.Lock()
	a.syncDockMonitorLockLocked()
	a.viewMu.Unlock()
}

func monitorLockPolicy(s config.Settings) platform.DockMonitorLockPolicy {
	modifier := hotkey.ModOption
	switch s.Dock.MonitorLock.BypassModifier {
	case "command":
		modifier = hotkey.ModCommand
	case "control":
		modifier = hotkey.ModControl
	case "shift":
		modifier = hotkey.ModShift
	}
	return platform.DockMonitorLockPolicy{Target: s.Dock.MonitorLock.Target, DisplayUUID: s.Dock.MonitorLock.DisplayUUID, Bypass: hotkey.ModSet(0).With(modifier)}
}

func (a *App) dockMonitorLockAllowedLocked() bool {
	select {
	case <-a.captureStop:
		return false
	default:
	}
	s := a.settingsSnapshot()
	return s.Dock.MonitorLock.Enabled && !s.Behavior.Paused && !a.sessionInactive
}

// Monitor protection remains independent of hover panels, preferences and the
// switcher. Configure invalidates native and placement admission synchronously.
func (a *App) syncDockMonitorLockLocked() {
	if a.dockMonitorLock != nil {
		a.dockMonitorLock.Configure(a.dockMonitorLockAllowedLocked(), monitorLockPolicy(a.settingsSnapshot()))
	}
}

func (a *App) acceptDockMonitorLockState(state platform.DockMonitorLockState) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.dockMonitorLock == nil {
		return
	}
	current := a.dockMonitorLock.Snapshot()
	if state.Session != current.Session || state.Revision != current.Revision || state.Generation != current.Generation || state.Sequence != current.Sequence {
		return
	}
	select {
	case <-a.captureStop:
		return
	default:
	}
	current.PlacementAvailable = a.dockPlacementAvailable()
	a.emit("dock:monitor-lock", current)
}

func (a *App) dockPlacementAvailable() bool {
	source, ok := a.platform.(platform.DockPlacementAvailability)
	return ok && source.DockPlacementAvailable()
}

func (a *App) GetDockMonitorLockState() platform.DockMonitorLockState {
	if a.dockMonitorLock != nil {
		state := a.dockMonitorLock.Snapshot()
		state.PlacementAvailable = a.dockPlacementAvailable()
		return state
	}
	state := platform.DockMonitorLockState{Status: "disabled", Displays: []platform.DockLockDisplay{}}
	if a.settingsSnapshot().Dock.MonitorLock.Enabled {
		state.Status, state.Reason = "unavailable", "Native Dock monitor locking is unavailable on this platform"
	}
	return state
}

// Display discovery is read-only and does not enable native input filtering.
func (a *App) GetDockMonitorLockDisplays() ([]platform.DockLockDisplay, error) {
	source, ok := a.platform.(platform.DockMonitorLockSource)
	if !ok {
		return nil, errors.New("native display inventory is unavailable on this platform")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	displays, err := source.DockMonitorLockDisplays(ctx)
	return append([]platform.DockLockDisplay{}, displays...), err
}

// This explicit UI action may briefly move the pointer. Native code owns the
// bounded attempt and must relinquish cursor restoration on physical takeover.
func (a *App) PlaceDockOnSelectedMonitor(session, revision, generation uint64) (platform.DockPlacementResult, error) {
	a.viewMu.Lock()
	// Settings are published before the shared configure hook runs. Synchronize
	// this owner now so a click cannot slip through that publication interval.
	a.syncDockMonitorLockLocked()
	allowed := a.dockMonitorLock != nil && a.dockMonitorLockAllowedLocked() && a.dockPlacementAvailable()
	a.viewMu.Unlock()
	if !allowed {
		return platform.DockPlacementResult{}, errors.New("dock monitor locking is disabled or unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return a.dockMonitorLock.PlaceScoped(ctx, session, revision, generation)
}

func (a *App) CancelDockPlacement() {
	if a.dockMonitorLock != nil {
		a.dockMonitorLock.CancelPlacement()
	}
}
