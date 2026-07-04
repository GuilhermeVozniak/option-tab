package main

import (
	"encoding/json"

	"option-tab/internal/platform"
)

func permStateString(s platform.PermState) string {
	switch s {
	case platform.PermGranted:
		return "granted"
	case platform.PermDenied:
		return "denied"
	default:
		return "unknown"
	}
}

// GetPermissions returns the grant state of the OS permissions the switcher
// needs as JSON: {"accessibility":"granted|denied|unknown","screenRecording":...}.
// The preferences UI polls this so a grant made in System Settings reflects live.
func (a *App) GetPermissions() string {
	b, _ := json.Marshal(map[string]string{
		"accessibility":   permStateString(a.platform.Accessibility()),
		"screenRecording": permStateString(a.platform.ScreenRecording()),
	})
	return string(b)
}

// RequestAccessibility triggers the OS Accessibility permission prompt (needed
// for the global hotkey and window actions).
func (a *App) RequestAccessibility() { a.platform.Request(platform.PermAccessibility) }

// RequestScreenRecording triggers the OS Screen Recording permission prompt
// (needed for live window thumbnails).
func (a *App) RequestScreenRecording() { a.platform.Request(platform.PermScreenRecording) }

// OpenPermissionSettings opens the System Settings privacy pane for a permission,
// guiding the user when a prior denial means the prompt no longer appears.
func (a *App) OpenPermissionSettings(kind string) {
	opener, ok := a.platform.(platform.SettingsOpener)
	if !ok {
		return
	}
	switch kind {
	case "accessibility":
		opener.OpenPrivacySettings(platform.PermAccessibility)
	case "screenRecording":
		opener.OpenPrivacySettings(platform.PermScreenRecording)
	}
}
