package main

import (
	"context"
	"errors"
	"math"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/platform"
)

func (a *App) BeginDockPreviewDrag(session, gesture, windowID uint64, appID int, pointerX, pointerY, grabX, grabY float64) error {
	if session == 0 || gesture == 0 || windowID == 0 || appID <= 0 || nonfinite(pointerX, pointerY, grabX, grabY) {
		return errors.New("invalid dock preview drag")
	}
	s := a.settingsSnapshot()
	if !s.Dock.Input.PreviewDrag {
		return errors.New("dock preview drag is disabled")
	}
	a.viewMu.Lock()
	if current := a.settingsSnapshot(); !current.Dock.Enabled || !current.Dock.Input.PreviewDrag || current.Behavior.Paused {
		a.viewMu.Unlock()
		return errors.New("dock preview drag is disabled")
	}
	if err := a.validateDockDragTargetLocked(session, domain.WindowID(windowID), domain.AppID(appID)); err != nil {
		a.viewMu.Unlock()
		return err
	}
	a.dockDragMu.Lock()
	if a.dockDragCancel != nil || session < a.dockDragHighSession || (session == a.dockDragHighSession && gesture <= a.dockDragHighGesture) || (session == a.dockDragCancelledSession && gesture <= a.dockDragCancelled) {
		a.dockDragMu.Unlock()
		a.viewMu.Unlock()
		return errors.New("dock preview drag gesture is stale or busy")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.dockDragHighSession, a.dockDragHighGesture = session, gesture
	a.dockDragGesture, a.dockDragSession, a.dockDragCancel = gesture, session, cancel
	a.dockDragMu.Unlock()
	a.viewMu.Unlock()
	defer func() {
		cancel()
		a.dockDragMu.Lock()
		if a.dockDragSession == session && a.dockDragGesture == gesture {
			a.dockDragCancel = nil
			a.dockDragSession = 0
			a.dockDragGesture = 0
		}
		a.dockDragMu.Unlock()
	}()
	windows, err := a.platform.Windows()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s = a.settingsSnapshot()
	fc := filter.Context{ActiveAppID: a.platform.ActiveApp(), ActiveSpaceID: a.platform.ActiveSpace(), ActiveScreenID: a.platform.ActiveScreen(), CursorScreenID: a.platform.CursorScreen(), SelfBundleID: selfBundleID}
	found := false
	for _, w := range filter.Apply(windows, s.Filters, s.Dock.Scope, fc) {
		if w.ID == domain.WindowID(windowID) && w.AppID == domain.AppID(appID) {
			found = true
			break
		}
	}
	if !found {
		return errors.New("dock preview window is no longer eligible")
	}
	if err := a.validatePendingDockDrag(session, gesture); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dragger, ok := a.platform.(platform.PreviewDragger)
	if !ok {
		return errors.New("dock preview drag is unavailable")
	}
	_, err = dragger.DragPreview(ctx, platform.PreviewDragRequest{Session: session, GestureID: gesture, WindowID: domain.WindowID(windowID), AppID: domain.AppID(appID), PointerX: pointerX, PointerY: pointerY, GrabX: math.Max(0, math.Min(1, grabX)), GrabY: math.Max(0, math.Min(1, grabY))})
	if err != nil && !errors.Is(err, context.Canceled) {
		a.setDockDragError(err)
	} else if err == nil {
		a.setDockDragError(nil)
	}
	return err
}

func (a *App) validatePendingDockDrag(session, gesture uint64) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	select {
	case <-a.captureStop:
		return errStaleDockSession
	default:
	}
	s := a.settingsSnapshot()
	if !s.Dock.Enabled || !s.Dock.Input.PreviewDrag || s.Behavior.Paused || a.sessionInactive {
		return errStaleDockSession
	}
	a.dockDragMu.Lock()
	defer a.dockDragMu.Unlock()
	if a.dockDragSession != session || a.dockDragGesture != gesture || a.dockDragCancel == nil {
		return errStaleDockSession
	}
	return nil
}

func (a *App) validateDockDragTargetLocked(session uint64, windowID domain.WindowID, appID domain.AppID) error {
	if session == 0 || session != a.dockState.Session || !a.dockAllowedLocked() {
		return errStaleDockSession
	}
	if epoch := a.dockState.AdmissionEpoch; epoch != 0 && a.dockController != nil && epoch != a.dockController.AdmissionEpoch() {
		return errStaleDockSession
	}
	if appID <= 0 || appID != a.dockState.Item.AppID || a.dockState.EmptyReason == "notRunning" {
		return errors.New("application is not available in this Dock preview")
	}
	for _, window := range a.dockState.Windows {
		if window.ID == windowID && window.AppID == appID {
			return nil
		}
	}
	return errors.New("window is not available in this Dock preview")
}

func (a *App) CancelDockPreviewDrag(session, gesture uint64) {
	a.dockDragMu.Lock()
	defer a.dockDragMu.Unlock()
	if session > a.dockDragCancelledSession || (session == a.dockDragCancelledSession && gesture > a.dockDragCancelled) {
		a.dockDragCancelledSession = session
		a.dockDragCancelled = gesture
	}
	if a.dockDragSession == session && a.dockDragGesture == gesture && a.dockDragCancel != nil {
		a.dockDragCancel()
	}
}

func (a *App) cancelDockPreviewDrag() {
	a.dockDragMu.Lock()
	defer a.dockDragMu.Unlock()
	if a.dockDragCancel != nil {
		a.dockDragCancel()
	}
}

func (a *App) syncDockPreviewDrag(s config.Settings) {
	if !s.Dock.Enabled || !s.Dock.Input.PreviewDrag || s.Behavior.Paused {
		a.cancelDockPreviewDrag()
		a.setDockDragError(nil)
	}
}

func (a *App) dockDragFloor(session uint64) uint64 {
	a.dockDragMu.Lock()
	defer a.dockDragMu.Unlock()
	var floor uint64
	if session == a.dockDragHighSession {
		floor = a.dockDragHighGesture
	}
	if session == a.dockDragCancelledSession && a.dockDragCancelled > floor {
		floor = a.dockDragCancelled
	}
	return floor
}
