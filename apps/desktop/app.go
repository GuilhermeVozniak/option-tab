package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/crash"
	"option-tab/internal/hotkey"
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
		dlog("registerHotkeys: registering shortcut %d chord=%q", sc.ID, sc.Chord)
		if rerr := eng.Register(sc.ID, chord); rerr != nil {
			dlog("registerHotkeys: register error: %v", rerr)
		}
	}
}

func (a *App) hotkeyLoop() {
	for ev := range a.platform.Hotkeys().Events() {
		dlog("hotkeyLoop: received event kind=%d shortcut=%d", ev.Kind, ev.ShortcutID)
		a.controller.HandleHotkey(ev)
	}
}

// ---- switcher.View ----

// Show reveals the overlay window and pushes the initial state. If the
// preferences window is open it is dismissed first, since both share the
// single Wails window.
func (a *App) Show(st switcher.State) {
	dlog("Show: %d entries, selected=%d", len(st.Entries), st.Selected)
	if a.prefsOpen {
		a.prefsOpen = false
		a.emit("prefs:close", nil)
		if wm, ok := a.platform.(platform.WindowModer); ok {
			wm.SetPrefsWindowMode(false)
		}
	}
	a.enrichIcons(&st)
	a.emit("switcher:show", st)
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowCenter(a.ctx)
	}
	a.captureThumbnails(st)
	a.capturePreview(st)
}

// captureThumbnails snapshots each window off the hotkey path and streams the
// results to the overlay via "switcher:thumbnails" events, so the switcher
// appears instantly (with icons) and previews fill in as they are captured.
// Each Show/Hide bumps thumbGen; a stale goroutine stops emitting.
func (a *App) captureThumbnails(st switcher.State) {
	if st.Style != config.StyleThumbnails {
		return
	}
	src, ok := a.platform.(platform.ThumbnailSource)
	if !ok {
		return
	}
	px := st.Appearance.ThumbnailMaxPx
	if px <= 0 {
		px = 256
	}
	gen := atomic.AddInt64(&a.thumbGen, 1)
	entries := st.Entries
	go func() {
		for _, e := range entries {
			if atomic.LoadInt64(&a.thumbGen) != gen {
				return
			}
			url := src.ThumbnailDataURL(e.WindowID, px)
			if url == "" || atomic.LoadInt64(&a.thumbGen) != gen {
				continue
			}
			a.emit("switcher:thumbnails", map[string]string{strconv.Itoa(int(e.WindowID)): url})
		}
	}()
}

// Update pushes a new state to the visible overlay.
func (a *App) Update(st switcher.State) {
	a.enrichIcons(&st)
	a.emit("switcher:update", st)
	a.capturePreview(st)
}

// capturePreview captures a high-resolution snapshot of the selected window
// when "preview selected window" is enabled, streamed via "switcher:preview".
// Stale captures are dropped via the same generation counter as thumbnails.
func (a *App) capturePreview(st switcher.State) {
	if !st.Appearance.PreviewSelected || st.Selected < 0 || st.Selected >= len(st.Entries) {
		return
	}
	src, ok := a.platform.(platform.ThumbnailSource)
	if !ok {
		return
	}
	id := st.Entries[st.Selected].WindowID
	gen := atomic.LoadInt64(&a.thumbGen)
	go func() {
		dataURL := src.ThumbnailDataURL(id, 1024)
		if dataURL == "" || atomic.LoadInt64(&a.thumbGen) != gen {
			return
		}
		a.emit("switcher:preview", map[string]string{strconv.Itoa(int(id)): dataURL})
	}()
}

// enrichIcons fills each entry's Icon with the owning app's icon (a base64 PNG
// data URL), cached by pid. It is a no-op when the platform provides no icons
// (stub/fake), so the overlay falls back to letter glyphs.
func (a *App) enrichIcons(st *switcher.State) {
	src, ok := a.platform.(platform.IconSource)
	if !ok {
		return
	}
	px := st.Appearance.IconSizePx * 2 // render at 2x for retina crispness
	if px <= 0 {
		px = 64
	}
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	if a.iconCache == nil {
		a.iconCache = make(map[int]string)
	}
	for i := range st.Entries {
		pid := int(st.Entries[i].AppID)
		img, cached := a.iconCache[pid]
		if !cached {
			img = src.AppIcon(pid, px)
			a.iconCache[pid] = img
		}
		st.Entries[i].Icon = img
	}
}

// Hide pushes the hide event and hides the overlay window.
func (a *App) Hide() {
	atomic.AddInt64(&a.thumbGen, 1) // invalidate any in-flight thumbnail capture
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

func (a *App) Advance()            { a.controller.Advance() }
func (a *App) Reverse()            { a.controller.Reverse() }
func (a *App) Confirm()            { a.controller.Confirm() }
func (a *App) Cancel()             { a.controller.Cancel() }
func (a *App) Select(index int)    { a.controller.Select(index) }
func (a *App) SetSearch(q string)  { a.controller.SetSearch(q) }
func (a *App) CloseSelected()      { a.controller.CloseSelected() }
func (a *App) MinimizeSelected()   { a.controller.MinimizeSelected() }
func (a *App) FullscreenSelected() { a.controller.FullscreenSelected() }
func (a *App) QuitSelectedApp()    { a.controller.QuitSelectedApp() }
func (a *App) HideSelectedApp()    { a.controller.HideSelectedApp() }

// ---- Pause & menubar ----

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

func (a *App) reRegisterHotkeys() {
	eng := a.platform.Hotkeys()
	for i := 1; i <= config.MaxShortcuts; i++ {
		_ = eng.Unregister(i)
	}
	a.registerHotkeys()
}

// ---- About / links ----

// appVersion is shown in the About tab.
const appVersion = "0.1.0"

// projectURL and releasesURL are the About-tab links; the free clone has no
// auto-updater, so "check for updates" opens the releases page.
const (
	projectURL  = "https://github.com/GuilhermeVozniak/option-tab"
	releasesURL = projectURL + "/releases"
)

// GetVersion returns the app version for the About tab.
func (a *App) GetVersion() string { return appVersion }

// OpenURL opens a link in the user's default browser (About-tab links).
func (a *App) OpenURL(url string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// CheckForUpdates opens the releases page in the browser (manual check).
func (a *App) CheckForUpdates() { a.OpenURL(releasesURL) }

// updateCheckURL is GitHub's "latest release" endpoint for this repo.
const updateCheckURL = "https://api.github.com/repos/GuilhermeVozniak/option-tab/releases/latest"

// updateLoop checks for a newer release shortly after launch and then daily,
// honoring Behavior.UpdatePolicy. A hit emits "update:available" so the
// preferences UI can show a download banner. There is no silent auto-install:
// the "auto" policy behaves like "check" until a signed updater exists.
func (a *App) updateLoop() {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for range timer.C {
		if a.settings.Behavior.UpdatePolicy != config.UpdatesOff {
			a.checkForUpdateOnce()
		}
		timer.Reset(24 * time.Hour)
	}
}

func (a *App) checkForUpdateOnce() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateCheckURL)
	if err != nil {
		dlog("update: check failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return // e.g. no releases yet (404) or rate-limited
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}
	rel, err := update.ParseLatest(body)
	if err != nil || !update.Newer(appVersion, rel.Version) {
		return
	}
	dlog("update: %s available at %s", rel.Version, rel.URL)
	a.emit("update:available", map[string]string{"version": rel.Version, "url": rel.URL})
}

// ---- Crash reports ----

// crashDir is where crash logs live (next to the settings file).
func (a *App) crashDir() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Dir(a.settingsPath)
}

// setupCrashCapture rotates any crash log left by the previous run to pending
// and arms capture for this run — unless the policy is "never", which also
// clears any leftovers. Nothing is ever transmitted automatically.
func (a *App) setupCrashCapture() {
	dir := a.crashDir()
	if dir == "" {
		return
	}
	if a.settings.Behavior.CrashReports == config.CrashNever {
		_ = crash.Dismiss(dir)
		return
	}
	if err := crash.Rotate(dir); err != nil {
		dlog("crash: rotate: %v", err)
	}
	if err := crash.Arm(dir); err != nil {
		dlog("crash: arm: %v", err)
	}
}

// GetCrashReport returns the previous run's crash log, or "" when there is
// none (or the policy is "never").
func (a *App) GetCrashReport() string {
	dir := a.crashDir()
	if dir == "" || a.settings.Behavior.CrashReports == config.CrashNever {
		return ""
	}
	return crash.Pending(dir)
}

// DismissCrashReport discards the pending crash log.
func (a *App) DismissCrashReport() {
	if dir := a.crashDir(); dir != "" {
		_ = crash.Dismiss(dir)
	}
}

// ReportCrash opens a prefilled GitHub issue containing the pending crash log
// (truncated), so the user sees exactly what is shared before submitting.
func (a *App) ReportCrash() {
	log := a.GetCrashReport()
	if log == "" {
		return
	}
	if len(log) > 3000 {
		log = log[:3000] + "\n… (truncated)"
	}
	issueURL := projectURL + "/issues/new?title=" +
		url.QueryEscape("Crash report ("+appVersion+")") +
		"&body=" + url.QueryEscape("```\n"+log+"\n```")
	a.OpenURL(issueURL)
}

// ---- Permissions ----

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
