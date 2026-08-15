// Package main wires the Wails desktop app. App is split by concern across
// files, all thin adapters over internal/ packages:
//
//	app.go             — struct, constructors, startup wiring
//	app_switcher.go    — switcher.View impl, bound switcher actions, hotkeys
//	app_prefs.go       — preferences window, pause, menubar tray
//	app_settings.go    — settings load/save bindings
//	app_update.go      — version/links and the background update check
//	app_crash.go       — crash-report capture and reporting
//	app_permissions.go — OS-permission state and requests
package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/mru"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
	"option-tab/internal/update"
)

// selfBundleID is the switcher's own bundle id, excluded from its window list.
const selfBundleID = "com.optiontab.app"

var debugOn = os.Getenv("OPTIONTAB_DEBUG") != ""

func dlog(format string, args ...any) {
	if debugOn {
		fmt.Fprintf(os.Stderr, "[ot] "+format+"\n", args...)
	}
}

// App is the Wails-bound service. It owns the platform backend and the switcher
// controller, implements switcher.View by emitting Wails events, and exposes
// the controller's actions plus settings access to the frontend. It holds no
// business logic of its own.
type App struct {
	// wailsApp, overlay, prefs and the tray wiring are injected by main via
	// setRuntime/setTray after the Wails app and windows are created. The
	// setters are unexported so they don't become frontend bindings.
	wailsApp *application.App
	overlay  *application.WebviewWindow
	prefs    *application.WebviewWindow
	// prefsFactory recreates the preferences window if macOS ever destroys it
	// under us (the Wails v3 alpha has no destroyed-window probe).
	prefsFactory func() *application.WebviewWindow

	// tray, trayMenu and pauseItem are the Wails v3 menubar pieces; nil until
	// setTray runs (and on stub/fake test paths).
	tray      *application.SystemTray
	trayMenu  *application.Menu
	pauseItem *application.MenuItem

	platform     platform.Platform
	controller   *switcher.Controller
	settings     config.Settings
	settingsPath string

	iconMu    sync.Mutex
	iconCache map[int]string // pid -> base64 PNG data URL

	// thumbGen invalidates in-flight thumbnail captures: each Show/Hide bumps
	// it, and a capture goroutine stops emitting once its generation is stale.
	thumbGen int64

	// prefsOpen tracks whether the preferences window is currently shown.
	prefsOpen bool

	// lastSelected is the previously shown selection index, used to fire the
	// haptic tick only when the selection actually moves.
	lastSelected int

	// thumbCache holds background-captured window thumbnails (data URLs) so
	// the switcher can paint instantly when CaptureInBackground is enabled.
	thumbCacheMu sync.Mutex
	thumbCache   map[domain.WindowID]string

	// updateMu guards the self-update state shared by the background checker
	// goroutine and the frontend-triggered install: the newest release found
	// (what the banner offers to install) and whether an install is running.
	updateMu         sync.Mutex
	pendingUpdate    *update.Release
	updateInstalling bool
}

// NewApp wires production dependencies: the native platform backend and the
// persisted settings.
func NewApp() *App {
	p, _ := platform.New()
	path, _ := config.DefaultPath()
	settings, _ := config.LoadFile(path)
	return newApp(p, settings, path)
}

// newApp builds an App from explicit dependencies (used by tests).
func newApp(p platform.Platform, settings config.Settings, settingsPath string) *App {
	a := &App{
		platform:     p,
		settings:     settings,
		settingsPath: settingsPath,
		thumbCache:   map[domain.WindowID]string{},
	}
	deps := switcher.Deps{
		Windows:      p,
		Focuser:      p,
		Env:          p,
		View:         a,
		MRU:          mru.New(),
		SelfBundleID: selfBundleID,
	}
	if cw, ok := p.(platform.CursorWarper); ok {
		deps.Cursor = cw
	}
	a.controller = switcher.New(deps, settings)
	return a
}

// setRuntime injects the Wails app and the two windows. Called by main before
// Run; unexported so it is not exposed as a frontend binding.
func (a *App) setRuntime(wailsApp *application.App, overlay, prefs *application.WebviewWindow, prefsFactory func() *application.WebviewWindow) {
	a.wailsApp = wailsApp
	a.overlay = overlay
	a.prefs = prefs
	a.prefsFactory = prefsFactory
}

// setTray injects the menubar tray, its menu, and the Pause item (whose label
// flips between Pause/Resume). Called by main before Run.
func (a *App) setTray(tray *application.SystemTray, menu *application.Menu, pauseItem *application.MenuItem) {
	a.tray = tray
	a.trayMenu = menu
	a.pauseItem = pauseItem
}

// startup runs on ApplicationDidFinishLaunching: requests permissions, syncs
// the tray, and starts the global hotkey listener and background loops. The
// overlay window starts hidden (created with Hidden: true).
func (a *App) startup() {
	dlog("startup: platform=%s accessibility=%v", a.platform.Name(), a.platform.Accessibility())
	if !a.settings.Behavior.Onboarded {
		// First launch: open preferences, where the onboarding wizard walks the
		// user through granting permissions instead of firing bare OS prompts.
		go func() {
			time.Sleep(400 * time.Millisecond) // let the webview finish loading
			a.OpenPreferences()
		}()
	} else {
		// The global hotkey needs Accessibility. Prompt on launch if missing; the
		// native tap retries until the grant takes effect, so no restart needed.
		if a.platform.Accessibility() != platform.PermGranted {
			dlog("startup: accessibility not granted, requesting")
			a.platform.Request(platform.PermAccessibility)
		}
		// Screen Recording is needed for live window thumbnails (default style).
		if a.platform.ScreenRecording() != platform.PermGranted {
			dlog("startup: screen recording not granted, requesting")
			a.platform.Request(platform.PermScreenRecording)
		}
		// Wails v3 activates the app at launch (its internal DidFinishLaunching
		// hook calls activateIgnoringOtherApps unconditionally). For a menu-bar
		// utility that is rude: drop the activation again once startup settles,
		// unless preferences opened (first launch) and the activation is wanted.
		if act, ok := a.platform.(platform.AppActivator); ok {
			go func() {
				time.Sleep(300 * time.Millisecond)
				if !a.prefsOpen {
					act.HideAppIfActive()
				}
			}()
		}
	}
	a.syncTray()
	a.setupCrashCapture()
	a.registerHotkeys()
	go a.hotkeyLoop()
	go a.keyLoop()
	go a.updateLoop()
	go a.backgroundCaptureLoop()
}

func (a *App) emit(name string, data any) {
	if a.wailsApp != nil {
		a.wailsApp.Event.Emit(name, data)
	}
}
