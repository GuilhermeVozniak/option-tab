package main

import "option-tab/internal/launcher"

// SetLauncherAutoHideHold extends only an already-visible, exact host owner.
// It deliberately does not depend on optional gesture/haptic capabilities.
func (a *App) SetLauncherAutoHideHold(epoch uint64, displayUUID string, session, revision, sequence uint64, phase string) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcher == nil || !a.launcherAllowedLocked() {
		return launcher.ErrRetired
	}
	scope := launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: revision}
	host := a.launcher.hosts[session]
	if host == nil || host.window == nil || !host.presentation.Visible || host.presentation.Scope != scope {
		return launcher.ErrRetired
	}
	return a.launcher.core.SetAutoHideHold(scope, sequence, phase)
}
