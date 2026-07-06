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
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/mru"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

// selfBundleID is the switcher's own bundle id, excluded from its window list.
const selfBundleID = "com.optiontab.app"

var debugOn = os.Getenv("OPTIONTAB_DEBUG") != ""

func dlog(format string, args ...any) {
	if debugOn {
		fmt.Fprintf(os.Stderr, "[ot] "+format+"\n", args...)
	}
}

// App is the Wails-bound adapter. It owns the platform backend and the switcher
// controller, implements switcher.View by emitting Wails events, and exposes
// the controller's actions plus settings access to the frontend. It holds no
// business logic of its own.
type App struct {
	ctx          context.Context
	platform     platform.Platform
	controller   *switcher.Controller
	settings     config.Settings
	settingsPath string

	iconMu    sync.Mutex
	iconCache map[int]string // pid -> base64 PNG data URL

	// thumbGen invalidates in-flight thumbnail captures: each Show/Hide bumps
	// it, and a capture goroutine stops emitting once its generation is stale.
	thumbGen int64

	// trayInstalled tracks whether the menubar status item is currently shown,
	// so syncTray installs/removes it at most once per state change.
	trayInstalled bool

	// prefsOpen tracks whether the shared window is currently showing the
	// preferences UI (titled window mode) rather than the overlay.
	prefsOpen bool

	// lastSelected is the previously shown selection index, used to fire the
	// haptic tick only when the selection actually moves.
	lastSelected int

	// thumbCache holds background-captured window thumbnails (data URLs) so
	// the switcher can paint instantly when CaptureInBackground is enabled.
	thumbCacheMu sync.Mutex
	thumbCache   map[domain.WindowID]string
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

// startup captures the Wails context, hides the overlay window, and starts the
// global hotkey listener.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	dlog("startup: platform=%s accessibility=%v", a.platform.Name(), a.platform.Accessibility())
	runtime.WindowHide(ctx)
	// The overlay window must be fully transparent (Wails leaves it opaque, so
	// the clear background would otherwise render as a square dark backdrop).
	if w, ok := a.platform.(platform.OverlayWindowPreparer); ok {
		w.PrepareOverlayWindow()
	}
	// A switcher is a background utility: keep it out of the Dock (bundles also
	// set LSUIElement; this covers `wails dev` runs).
	if h, ok := a.platform.(platform.DockHider); ok {
		h.HideDockIcon()
	}
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
	}
	a.syncTray()
	a.setupCrashCapture()
	a.registerHotkeys()
	go a.hotkeyLoop()
	go a.updateLoop()
	go a.backgroundCaptureLoop()
}

func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}
