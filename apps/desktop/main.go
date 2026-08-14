package main

import (
	"embed"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"option-tab/internal/platform"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	wailsApp := application.New(application.Options{
		Name: "Option Tab",
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			// A switcher is a background utility: no Dock icon, no menu bar.
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.optiontab.app",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				dlog("second instance launch: opening preferences")
				app.OpenPreferences()
			},
		},
	})

	// --- Switcher overlay window ---
	// Frameless, transparent, always-on-top, and created hidden; shown on the
	// global hotkey. Show() never activates the app (Wails v3's windowShow is a
	// bare makeKeyAndOrderFront), so the previously active app keeps keyboard
	// focus — activating here is what broke option+tab switching under Wails v2.
	// The transparent backdrop (opaque=NO + clear color) is applied natively by
	// Wails from these options. AlwaysOnTop is NOT: for hidden windows Wails
	// only applies it on WindowDidBecomeKey, which never fires for a
	// never-activated window, so App.Show re-asserts it via SetAlwaysOnTop.
	overlay := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:          "overlay",
		Title:         "Option Tab",
		Width:         1280,
		Height:        820,
		Frameless:     true,
		AlwaysOnTop:   true,
		Hidden:        true,
		DisableResize: true,
		Mac: application.MacWindow{
			Backdrop:      application.MacBackdropTransparent,
			DisableShadow: true, // the panel draws its own CSS shadow
			Appearance:    application.NSAppearanceNameDarkAqua,
		},
	})

	// --- Preferences window ---
	// A regular titled window, created hidden and shown on demand. Wails v3
	// alpha has no hide-on-close option, so closing is intercepted: hide the
	// window (the app keeps running as a menu-bar accessory) and drop the Dock
	// icon that opening preferences added. The factory lets OpenPreferences
	// recreate the window if it was ever destroyed anyway.
	makePrefs := func() *application.WebviewWindow {
		prefs := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
			Title:  "Option Tab",
			Width:  900,
			Height: 640,
			Hidden: true,
			URL:    "/#/settings",
			Mac: application.MacWindow{
				Appearance: application.NSAppearanceNameDarkAqua,
			},
		})
		prefs.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
			e.Cancel()
			app.closePreferencesWindow()
		})
		return prefs
	}
	app.setRuntime(wailsApp, overlay, makePrefs(), makePrefs)

	// --- Menubar tray ---
	// Menu accelerators are display-only on macOS (a status-item menu is outside
	// the key-equivalent responder chain); the CGEventTap in internal/platform
	// remains the only global hotkey handler, so no double-firing.
	tray := wailsApp.SystemTray.New()
	menu := wailsApp.NewMenu()
	menu.Add("Show Option Tab").OnClick(func(*application.Context) {
		app.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	})
	pauseItem := menu.Add("Pause").OnClick(func(*application.Context) { app.TogglePause() })
	menu.AddSeparator()
	menu.Add("Settings…").SetAccelerator("CmdOrCtrl+,").OnClick(func(*application.Context) { app.OpenPreferences() })
	menu.Add("Check for updates…").OnClick(func(*application.Context) { app.CheckForUpdates() })
	menu.Add("Check permissions…").OnClick(func(*application.Context) { app.openPreferencesTab("General") })
	menu.AddSeparator()
	menu.Add("About Option Tab").OnClick(func(*application.Context) { app.openPreferencesTab("About") })
	menu.Add("Debug tools").OnClick(func(*application.Context) {
		// Reveal the folder holding settings and crash logs.
		if dir := app.crashDir(); dir != "" {
			app.OpenURL("file://" + dir)
		}
	})
	menu.Add("Send feedback…").OnClick(func(*application.Context) { app.OpenURL(projectURL + "/issues/new") })
	menu.Add("Support this project").OnClick(func(*application.Context) { app.OpenURL(projectURL) })
	menu.AddSeparator()
	menu.Add("Quit Option Tab").SetAccelerator("CmdOrCtrl+Q").OnClick(func(*application.Context) { wailsApp.Quit() })
	tray.SetMenu(menu)
	app.setTray(tray, menu, pauseItem)

	// --- Lifecycle ---
	// Application-lifecycle events are subscribed on the Event manager with
	// typed constants from pkg/events (Wails v3 has no OnStartup option).
	wailsApp.Event.OnApplicationEvent(events.Mac.ApplicationDidFinishLaunching, func(*application.ApplicationEvent) {
		app.startup()
	})

	if err := wailsApp.Run(); err != nil {
		slog.Error("app exited", "err", err)
	}
}
