package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// SettingsState pairs a canonical snapshot with its process-local write revision.
// Renderers do not survive a process restart, so this revision is not persisted.
type SettingsState struct {
	Revision uint64 `json:"revision"`
	JSON     string `json:"json"`
}

func (a *App) GetSettingsState() SettingsState {
	// A menu callback may run on AppKit's main thread. It must not wait for
	// saveMu while a writer is applying native settings on that same thread.
	a.settingsMu.RLock()
	s, revision := a.settingsSnapshotLocked(), max(a.settingsRevision, 1)
	a.settingsMu.RUnlock()
	b, err := json.Marshal(s)
	if err != nil {
		return SettingsState{}
	}
	return SettingsState{Revision: revision, JSON: string(b)}
}

func (a *App) SaveSettingsAtRevision(document string, expectedRevision uint64) (SettingsState, error) {
	s, err := config.Load(strings.NewReader(document))
	if err != nil {
		return SettingsState{}, err
	}
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	current := a.GetSettingsState()
	if expectedRevision == 0 || expectedRevision != current.Revision {
		return current, errors.New("settings: stale revision")
	}
	if err := a.saveSettingsLocked(s); err != nil {
		return current, err
	}
	return a.GetSettingsState(), nil
}

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
	return a.settingsSnapshotLocked()
}

func (a *App) settingsSnapshotLocked() config.Settings {
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
	if a.settingsRevision == 0 {
		a.settingsRevision = 1
	}
	a.settingsRevision++
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
