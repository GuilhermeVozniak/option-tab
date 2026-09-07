package main

import (
	"context"
	"reflect"
	"slices"
	"time"

	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func (a *App) RelaunchLauncherItem(epoch uint64, displayUUID string, session, revision uint64, itemID string) error {
	a.viewMu.Lock()
	r := a.launcher
	allowed := a.launcherAllowedLocked() && r.ready
	a.viewMu.Unlock()
	if !allowed || r == nil {
		return launcher.ErrRetired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return r.core.PerformConfigured(ctx, launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: revision}, itemID, "relaunch")
}

// Source owns final physical panel/process/resource validation. The supplied
// callback rechecks only Go admission and must never dispatch to AppKit.
func (a *App) performConfiguredLauncherItem(ctx context.Context, scope launcher.Scope, target launcher.ConfiguredTarget, action string, controllerGuard func() error) error {
	if ctx == nil || controllerGuard == nil {
		return launcher.ErrRetired
	}
	if action != "open" && action != "relaunch" {
		return launcher.ErrUnavailable
	}
	target.Item.Members = slices.Clone(target.Item.Members)
	if target.Item.Kind != "link" && (target.Reference.ID != target.Item.ReferenceID || target.Reference.Kind != target.Item.Kind || target.Reference.State != "ready" || target.Reference.Revision == 0) {
		return launcher.ErrUnavailable
	}
	if action == "relaunch" && (target.Item.Kind != "app" || target.Reference.Process.PID <= 0 || target.Reference.Process.StartSeconds == 0) {
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m.closed || m.refs == nil {
		a.viewMu.Unlock()
		return launcher.ErrUnavailable
	}
	m.wg.Add(1)
	a.viewMu.Unlock()
	defer m.wg.Done()
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	if m.ctx.Err() != nil {
		cancel()
	}
	var owner *appLauncherHost
	var token uint64
	var profileID string
	guard := func() error {
		if err := child.Err(); err != nil {
			return err
		}
		if err := controllerGuard(); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if a.launcherItems != m || m.closed || !a.launcherAllowedLocked() || !a.launcher.ready {
			return launcher.ErrRetired
		}
		if target.Item.Kind == "app" && !a.launcherReferenceEligible(target.Reference) {
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
		} else if owner != h || h.presentation.Epoch != scope.Epoch || h.presentation.DisplayUUID != scope.DisplayUUID || h.presentation.Session != scope.Session || h.presentation.ProfileID != profileID {
			return launcher.ErrRetired
		}
		// A settings publication can precede controller reconciliation. Do not admit
		// the old selected resource in that gap, even with an otherwise current scope.
		matched := false
		for _, p := range a.settingsSnapshot().ReplacementDock.Profiles {
			if p.ID == h.presentation.ProfileID {
				for _, item := range p.Items {
					if item.ID == target.Item.ID && reflect.DeepEqual(item, target.Item) {
						matched = true
						break
					}
				}
			}
		}
		if !matched {
			return launcher.ErrRetired
		}
		panel, ok := h.window.wheelPanel().(platform.LauncherPanel)
		if !ok || panel.LauncherToken() == 0 || (token != 0 && token != panel.LauncherToken()) {
			return launcher.ErrRetired
		}
		owner, token, profileID = h, panel.LauncherToken(), h.presentation.ProfileID
		return nil
	}
	if err := guard(); err != nil {
		return err
	}
	if target.Item.Kind == "link" {
		if action != "open" {
			return launcher.ErrUnavailable
		}
		return m.refs.OpenLauncherLink(child, target.Item.URL, scope.DisplayUUID, token, guard)
	}
	switch target.Item.Kind {
	case "app", "file", "folder":
	default:
		return launcher.ErrUnavailable
	}
	return m.refs.PerformLauncherItem(child, platform.LauncherItemAction{ReferenceID: target.Reference.ID, ReferenceRevision: target.Reference.Revision, Kind: action, Process: target.Reference.Process, BundleID: target.Reference.BundleID, DisplayUUID: scope.DisplayUUID, PanelToken: token}, guard)
}
