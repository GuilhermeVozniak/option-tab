package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"

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
	a.syncTray()
}

// TogglePause flips the paused state and returns the new value.
func (a *App) TogglePause() bool {
	a.SetPaused(!a.controller.Paused())
	return a.controller.Paused()
}

// IsPaused reports whether activation is currently suspended.
func (a *App) IsPaused() bool { return a.controller.Paused() }

// ---- Preferences window ----

// OpenPreferences shows and focuses the preferences window (created hidden at
// startup; recreated defensively if macOS destroyed it). Invoked by the menubar
// "Settings…" item and on first launch (onboarding).
func (a *App) OpenPreferences() {
	dlog("OpenPreferences: prefsOpen=%v", a.prefsOpen)
	a.prefsOpen = true
	// Preferences need keyboard focus: flip the accessory app to a regular,
	// activated app (the switcher overlay itself never activates).
	if act, ok := a.platform.(platform.AppActivator); ok {
		act.ActivateForPrefs()
	}
	a.showPrefsWindow()
}

// showPrefsWindow shows and focuses the preferences window, recreating it once
// via the factory if the native window was destroyed under us — the Wails v3
// alpha has no destroyed-window probe, so a recovered panic is the only signal.
func (a *App) showPrefsWindow() {
	w := a.prefs
	if w == nil && a.prefsFactory != nil {
		w = a.prefsFactory()
		a.prefs = w
	}
	if w == nil {
		return
	}
	if !tryShowWindow(w) && a.prefsFactory != nil {
		w = a.prefsFactory()
		a.prefs = w
		tryShowWindow(w)
	}
}

func tryShowWindow(w *application.WebviewWindow) (ok bool) {
	defer func() {
		if r := recover(); r != nil {
			dlog("prefs window Show/Focus panicked (window destroyed?): %v", r)
			ok = false
		}
	}()
	w.Show()
	w.Focus()
	return true
}

// ClosePreferences hides the preferences window and returns the app to the
// accessory policy (no Dock icon).
func (a *App) ClosePreferences() {
	a.closePreferencesWindow()
}

// closePreferencesWindow is the internal hide path (also used when the switcher
// opens over an open preferences window and on WindowClosing).
func (a *App) closePreferencesWindow() {
	a.prefsOpen = false
	if a.prefs != nil {
		a.prefs.Hide()
	}
	if h, ok := a.platform.(platform.DockHider); ok {
		h.HideDockIcon()
	}
}

// openPreferencesTab opens the preferences window on a specific tab.
func (a *App) openPreferencesTab(tab string) {
	a.OpenPreferences()
	a.emit("prefs:tab", tab)
}

// ---- Menubar tray ----

// trayGlyph maps the MenubarIconStyle setting to the status-item text glyph.
func trayGlyph(style config.MenubarIconStyle) string {
	switch style {
	case config.MenubarIconOutline:
		return "⧉"
	case config.MenubarIconDot:
		return "●"
	default:
		return "⌥⇥"
	}
}

// syncTray reconciles the menubar status item with the current settings and
// pause state: visibility (ShowMenubarIcon), the Pause/Resume label, and the
// icon glyph. Menu mutations touch AppKit, so they are marshaled onto the main
// thread (and the menu is rebuilt to pick up label changes).
func (a *App) syncTray() {
	if a.tray == nil {
		return
	}
	show := a.settings.Behavior.ShowMenubarIcon
	paused := a.controller.Paused()
	glyph := trayGlyph(a.settings.Behavior.MenubarIconStyle)
	application.InvokeAsync(func() {
		if a.pauseItem != nil {
			label := "Pause"
			if paused {
				label = "Resume"
			}
			a.pauseItem.SetLabel(label)
		}
		if a.trayMenu != nil {
			a.trayMenu.Update()
		}
		a.tray.SetLabel(glyph)
		if show {
			a.tray.Show()
		} else {
			a.tray.Hide()
		}
	})
}
