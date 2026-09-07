package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// GetSettings returns the current settings as JSON for the preferences UI.
func (a *App) GetSettings() string {
	b, err := json.Marshal(a.settingsSnapshot())
	if err != nil {
		return "{}"
	}
	return string(b)
}

// settingsSnapshot copies mutable fields so callers cannot mutate
// shared settings after the read lock has been released.
func (a *App) settingsSnapshot() config.Settings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	s := a.settings
	s.Shortcuts = slices.Clone(s.Shortcuts)
	s.Behavior.ActionBindings = maps.Clone(s.Behavior.ActionBindings)
	s.AppSwitcher.Behavior.ActionBindings = maps.Clone(s.AppSwitcher.Behavior.ActionBindings)
	s.Filters.AppBlacklist = slices.Clone(s.Filters.AppBlacklist)
	s.ReplacementDock = config.CloneReplacementDock(s.ReplacementDock)
	return s
}

// SaveSettings serializes saves and publishes only successfully persisted settings.
func (a *App) SaveSettings(jsonStr string) error {
	s, err := config.Load(strings.NewReader(jsonStr))
	if err != nil {
		return err
	}
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	return a.saveSettingsLocked(s)
}

// saveSettingsLocked requires saveMu. Platform and disk failures leave the
// currently published settings unchanged.
func (a *App) saveSettingsLocked(s config.Settings) error {
	previous := a.settingsSnapshot()
	loginChanged := s.Behavior.StartAtLogin != previous.Behavior.StartAtLogin
	if loginChanged {
		if err := a.platform.SetEnabled(s.Behavior.StartAtLogin); err != nil {
			return fmt.Errorf("settings: start at login: %w", err)
		}
	}
	if a.settingsPath != "" {
		if err := config.SaveFile(a.settingsPath, s); err != nil {
			if loginChanged {
				if rollbackErr := a.platform.SetEnabled(previous.Behavior.StartAtLogin); rollbackErr != nil {
					return fmt.Errorf("%w; restoring start at login failed: %v", err, rollbackErr)
				}
			}
			return err
		}
	}
	a.settingsMu.Lock()
	a.settings = s
	a.settingsMu.Unlock()
	a.controller.SetSettings(s)
	a.configureDock(s)
	a.configureLauncher(previous, s)
	a.reRegisterHotkeys()
	a.syncTray()
	return nil
}

// CaptureShortcut arms native chord recording for the preferences UI and
// blocks until the next chord pressed anywhere — including Command+Tab and
// the switcher's own chord, which never reach the webview. Returns "" when
// cancelled (Escape), timed out, or unsupported (stub platforms).
func (a *App) CaptureShortcut() string {
	c, ok := a.platform.(platform.ShortcutCapturer)
	if !ok {
		return ""
	}
	return c.CaptureShortcut(10 * time.Second)
}

// CancelShortcutCapture disarms a pending shortcut capture (the recorder
// input lost focus).
func (a *App) CancelShortcutCapture() {
	if c, ok := a.platform.(platform.ShortcutCapturer); ok {
		c.CancelShortcutCapture()
	}
}
