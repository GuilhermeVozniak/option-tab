package main

import (
	"encoding/json"
	"strings"

	"option-tab/internal/config"
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
