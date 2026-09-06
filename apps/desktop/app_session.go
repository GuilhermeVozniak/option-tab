package main

import (
	"context"

	"option-tab/internal/platform"
)

// Session observation belongs to the App lifetime, including when Dock previews
// are disabled or their observer has been suspended.
func (a *App) startSessionObservation() {
	a.sessionObserveOnce.Do(func() {
		source, ok := a.platform.(platform.SessionObservationSource)
		if !ok {
			return
		}
		select {
		case <-a.captureStop:
			return
		default:
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-a.captureStop:
				cancel()
			case <-ctx.Done():
			}
		}()
		go func() {
			defer cancel()
			if err := source.ObserveSession(ctx, a.acceptSessionState); err != nil && ctx.Err() == nil {
				dlog("session observation ended: %v", err)
			}
		}()
	})
}

func (a *App) acceptSessionState(state platform.SessionState) {
	if state.Generation == 0 {
		return
	}
	a.transitionSession(state.Generation, state.Inactive)
}

func (a *App) transitionSession(generation uint64, inactive bool) {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	a.viewMu.Lock()
	select {
	case <-a.captureStop:
		a.viewMu.Unlock()
		return
	default:
	}
	previous := a.sessionGeneration
	if generation != 0 {
		if generation <= previous {
			a.viewMu.Unlock()
			return
		}
		a.sessionGeneration = generation
	}
	// Even an active latest state can summarize a lock/unlock between deliveries.
	// Close admission before cancelling controller state and prior capture epochs.
	invalidate := inactive || (generation != 0 && previous != 0)
	if invalidate {
		a.sessionInactive = true
		a.cancelDismissalLocked()
		a.dismissDockLocked()
		a.captureActive = false
		a.captureDockSession.Store(0)
		a.switcherVisible = false
		retiredSession := a.visibleSwitcherSession
		a.visibleSwitcherSession = 0
		if a.captures != nil {
			a.captures.Hide()
		}
		a.captureSwitcherSession.Store(0)
		a.platform.Hotkeys().SetOpen(false)
		a.setKeySession(0)
		a.emitSwitcherHideLocked(retiredSession)
		a.overlay.hide()
		a.syncDockSuspensionLocked()
	}
	a.viewMu.Unlock()
	// Hide reenters viewMu. Never call the switcher controller while holding it.
	if invalidate && a.controller != nil {
		a.controller.Suspend(true)
	}
	a.viewMu.Lock()
	select {
	case <-a.captureStop:
		a.viewMu.Unlock()
		return
	default:
	}
	a.sessionInactive = inactive
	a.syncDockSuspensionLocked()
	a.viewMu.Unlock()
	if !inactive && a.controller != nil {
		a.controller.Suspend(false)
	}
}
