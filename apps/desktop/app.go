package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/hotkey"
	"option-tab/internal/mru"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

// selfBundleID is the switcher's own bundle id, excluded from its window list.
const selfBundleID = "com.optiontab.app"

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
	a := &App{platform: p, settings: settings, settingsPath: settingsPath}
	a.controller = switcher.New(switcher.Deps{
		Windows:      p,
		Focuser:      p,
		Env:          p,
		View:         a,
		MRU:          mru.New(),
		SelfBundleID: selfBundleID,
	}, settings)
	return a
}

// startup captures the Wails context, hides the overlay window, and starts the
// global hotkey listener.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowHide(ctx)
	a.registerHotkeys()
	go a.hotkeyLoop()
}

func (a *App) registerHotkeys() {
	eng := a.platform.Hotkeys()
	for _, sc := range a.settings.Shortcuts {
		if !sc.Enabled {
			continue
		}
		chord, err := hotkey.Parse(sc.Chord)
		if err != nil {
			continue
		}
		_ = eng.Register(sc.ID, chord)
	}
}

func (a *App) hotkeyLoop() {
	for ev := range a.platform.Hotkeys().Events() {
		a.controller.HandleHotkey(ev)
	}
}

// ---- switcher.View ----

// Show reveals the overlay window and pushes the initial state.
func (a *App) Show(st switcher.State) {
	a.emit("switcher:show", st)
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowCenter(a.ctx)
	}
}

// Update pushes a new state to the visible overlay.
func (a *App) Update(st switcher.State) { a.emit("switcher:update", st) }

// Hide pushes the hide event and hides the overlay window.
func (a *App) Hide() {
	a.emit("switcher:hide", nil)
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

// ---- Bound controller actions (called from the frontend) ----

func (a *App) Advance()           { a.controller.Advance() }
func (a *App) Reverse()           { a.controller.Reverse() }
func (a *App) Confirm()           { a.controller.Confirm() }
func (a *App) Cancel()            { a.controller.Cancel() }
func (a *App) Select(index int)   { a.controller.Select(index) }
func (a *App) SetSearch(q string) { a.controller.SetSearch(q) }
func (a *App) CloseSelected()     { a.controller.CloseSelected() }
func (a *App) MinimizeSelected()  { a.controller.MinimizeSelected() }
func (a *App) QuitSelectedApp()   { a.controller.QuitSelectedApp() }
func (a *App) HideSelectedApp()   { a.controller.HideSelectedApp() }

// ---- Settings ----

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
	a.settings = s
	a.controller.SetSettings(s)
	if a.settingsPath != "" {
		_ = config.SaveFile(a.settingsPath, s)
	}
	a.reRegisterHotkeys()
	return nil
}

func (a *App) reRegisterHotkeys() {
	eng := a.platform.Hotkeys()
	for i := 1; i <= config.MaxShortcuts; i++ {
		_ = eng.Unregister(i)
	}
	a.registerHotkeys()
}
