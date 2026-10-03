package main

import (
	"context"
	"errors"
	"reflect"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func launcherMutationError(code string) error { return errors.New("launcher mutation: " + code) }

// MutateLauncherItems changes only existing profile structure after exact native
// presentation admission. Once the final admitted save starts, disk owns completion.
func (a *App) MutateLauncherItems(epoch uint64, displayUUID string, session, revision uint64, expectedItemsRevision string, mutation config.LauncherItemMutation) error {
	scope := launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: revision}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	a.viewMu.Lock()
	runtime := a.launcher
	sessionGeneration := a.sessionGeneration
	if runtime == nil || !a.launcherAllowedLocked() || !runtime.ready {
		a.viewMu.Unlock()
		return launcherMutationError("stale")
	}
	host := runtime.hosts[session]
	if host == nil || host.window == nil || host.presentation.Scope != scope || !host.presentation.Visible {
		a.viewMu.Unlock()
		return launcherMutationError("stale")
	}
	window := host.window
	panel, ok := window.wheelPanel().(platform.LauncherPanel)
	if !ok || panel.LauncherToken() == 0 {
		a.viewMu.Unlock()
		return launcherMutationError("itemsUnavailable")
	}
	token := panel.LauncherToken()
	validator, ok := panel.(platform.LauncherPanelValidator)
	if !ok {
		a.viewMu.Unlock()
		return launcherMutationError("itemsUnavailable")
	}
	authority, err := runtime.core.CaptureItemMutation(scope, expectedItemsRevision, mutation.ItemID, mutation.TargetID)
	a.viewMu.Unlock()
	if err != nil {
		return launcherMutationError("stale")
	}
	logical := func() error {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if ctx.Err() != nil || a.sessionGeneration != sessionGeneration || a.launcher != runtime || !a.launcherAllowedLocked() || !runtime.ready || runtime.hosts[session] != host || host.window != window || host.presentation.Scope != scope || !host.presentation.Visible {
			return launcherMutationError("stale")
		}
		current, ok := window.wheelPanel().(platform.LauncherPanel)
		if !ok || current.LauncherToken() != token {
			return launcherMutationError("stale")
		}
		if runtime.core.ValidateItemMutation(authority) != nil {
			return launcherMutationError("stale")
		}
		return nil
	}
	guard := func() error {
		if err := logical(); err != nil {
			return err
		}
		if err := validator.ValidateLauncherPanel(ctx, displayUUID); err != nil {
			return launcherMutationError("stale")
		}
		return logical()
	}
	if err = guard(); err != nil {
		return err
	}
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if err = logical(); err != nil {
		return err
	}
	settings := a.settingsSnapshot()
	index := -1
	for i, p := range settings.ReplacementDock.Profiles {
		if p.ID == authority.ProfileID {
			index = i
			break
		}
	}
	if index < 0 || !settings.ReplacementDock.Enabled || !settings.ReplacementDock.Profiles[index].RuntimeReorder || config.LauncherItemsRevision(settings.ReplacementDock.Profiles[index].Items) != authority.ItemsRevision {
		return launcherMutationError("stale")
	}
	mutation.ItemID, mutation.TargetID = authority.ItemID, authority.TargetID
	items, err := config.MutateLauncherItems(settings.ReplacementDock.Profiles[index].Items, mutation)
	if err != nil {
		return launcherMutationError("invalidMutation")
	}
	if err = guard(); err != nil {
		return err
	}
	if reflect.DeepEqual(items, settings.ReplacementDock.Profiles[index].Items) {
		return nil
	}
	settings.ReplacementDock.Profiles[index].Items = items
	if err = a.saveSettingsLocked(settings); err != nil {
		return launcherMutationError("saveFailed")
	}
	return nil
}
