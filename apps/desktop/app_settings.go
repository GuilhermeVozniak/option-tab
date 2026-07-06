package main

import (
	"encoding/json"
	"strings"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// GetSettings returns the current settings as JSON for the preferences UI.
func (a *App) GetSettings() string {
	b, err := json.Marshal(a.settings)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// SaveSettings validates, applies, and persists settings from the preferences
// UI, then re-registers hotkeys to reflect any chord changes.
func (a *App) SaveSettings(jsonStr string) error {
	s, err := config.Load(strings.NewReader(jsonStr))
	if err != nil {
		return err
	}
	if s.Behavior.StartAtLogin != a.settings.Behavior.StartAtLogin {
		_ = a.platform.SetEnabled(s.Behavior.StartAtLogin)
	}
	a.settings = s
	a.controller.SetSettings(s)
	if a.settingsPath != "" {
		_ = config.SaveFile(a.settingsPath, s)
	}
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
