package main

import (
	"context"

	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func (a *App) relaunchLauncherTarget(ctx context.Context, scope launcher.Scope, target platform.LauncherAppTarget, controllerGuard func() error) error {
	if ctx == nil || controllerGuard == nil {
		return launcher.ErrRetired
	}
	source, ok := a.platform.(platform.LauncherAppRelauncher)
	if !ok {
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m.closed {
		a.viewMu.Unlock()
		return launcher.ErrRetired
	}
	m.wg.Add(1)
	a.viewMu.Unlock()
	defer m.wg.Done()
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	var owner *appLauncherHost
	var token uint64
	var profile string
	guard := func() error {
		if err := child.Err(); err != nil {
			return err
		}
		if err := controllerGuard(); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if a.launcherItems != m || m.closed || m.ctx.Err() != nil || !a.launcherAllowedLocked() || !a.launcher.ready || !a.launcherAppEligible(domain.App{ID: target.Process.PID, Name: target.Name, BundleID: target.BundleID}) {
			return launcher.ErrRetired
		}
		h := a.launcher.hosts[scope.Session]
		if h == nil || !h.presentation.Visible {
			return launcher.ErrRetired
		}
		if owner == nil {
			if h.presentation.Scope != scope {
				return launcher.ErrRetired
			}
		} else if h != owner || h.presentation.Epoch != scope.Epoch || h.presentation.Session != scope.Session || h.presentation.DisplayUUID != scope.DisplayUUID || h.presentation.ProfileID != profile {
			return launcher.ErrRetired
		}
		panel, ok := h.window.wheelPanel().(platform.LauncherPanel)
		if !ok || panel.LauncherToken() == 0 || (token != 0 && token != panel.LauncherToken()) {
			return launcher.ErrRetired
		}
		owner, token, profile = h, panel.LauncherToken(), h.presentation.ProfileID
		return nil
	}
	if err := guard(); err != nil {
		return err
	}
	target.PanelToken = token
	return source.RelaunchLauncherApp(child, target, guard)
}
