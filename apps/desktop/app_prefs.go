package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// ---- Pause ----

// SetPaused suspends or resumes activation, persists the choice, and reflects it
// in the menubar. While paused the global hotkey does not open the switcher.
func (a *App) SetPaused(paused bool) {
	a.settings.Behavior.Paused = paused
	a.controller.SetPaused(paused)
	if a.settingsPath != "" {
		_ = config.SaveFile(a.settingsPath, a.settings)
	}
	if t, ok := a.platform.(platform.Tray); ok && a.trayInstalled {
		t.SetTrayPaused(paused)
	}
}

// TogglePause flips the paused state and returns the new value.
func (a *App) TogglePause() bool {
	a.SetPaused(!a.controller.Paused())
	return a.controller.Paused()
}

// IsPaused reports whether activation is currently suspended.
func (a *App) IsPaused() bool { return a.controller.Paused() }

// ---- Preferences window ----

// OpenPreferences turns the shared window into a regular titled window and
// asks the frontend to render the settings panel. Invoked by the menubar
// "Preferences…" item and on first launch (onboarding).
func (a *App) OpenPreferences() {
	a.prefsOpen = true
	if wm, ok := a.platform.(platform.WindowModer); ok {
		wm.SetPrefsWindowMode(true)
	}
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
	}
	a.emit("prefs:open", nil)
}

// ClosePreferences hides the window, restores the overlay chrome, and tells
// the frontend to dismiss the settings panel.
func (a *App) ClosePreferences() {
	a.prefsOpen = false
	a.emit("prefs:close", nil)
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
	if wm, ok := a.platform.(platform.WindowModer); ok {
		wm.SetPrefsWindowMode(false)
	}
}

// beforeClose intercepts the titled preferences window's close button: it
// dismisses preferences instead of quitting. Any other close quits normally.
func (a *App) beforeClose(_ context.Context) bool {
	if a.prefsOpen {
		a.ClosePreferences()
		return true
	}
	return false
}

// ---- Menubar tray ----

// syncTray reconciles the menubar status item with the ShowMenubarIcon setting,
// installing or removing it as needed and refreshing the Pause label. It is a
// no-op when the platform provides no tray (stub/fake).
func (a *App) syncTray() {
	t, ok := a.platform.(platform.Tray)
	if !ok {
		return
	}
	switch {
	case a.settings.Behavior.ShowMenubarIcon && !a.trayInstalled:
		cmds := t.InstallTray()
		a.trayInstalled = true
		go a.trayLoop(cmds)
	case !a.settings.Behavior.ShowMenubarIcon && a.trayInstalled:
		t.RemoveTray()
		a.trayInstalled = false
	}
	if a.trayInstalled {
		t.SetTrayPaused(a.controller.Paused())
		t.SetTrayStyle(string(a.settings.Behavior.MenubarIconStyle))
	}
}

// trayLoop routes menubar commands to the matching action.
func (a *App) trayLoop(cmds <-chan platform.TrayCommand) {
	for cmd := range cmds {
		switch cmd {
		case platform.TrayPreferences:
			a.OpenPreferences()
		case platform.TrayTogglePause:
			a.TogglePause()
		case platform.TrayQuit:
			if a.ctx != nil {
				runtime.Quit(a.ctx)
			}
		}
	}
}
