package main

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/switcher"
	"option-tab/internal/update"
)

// ---- Test doubles: wrappers extending fake.Fake with optional capabilities ----

// pollUntil polls cond every few milliseconds until it holds or the deadline
// elapses (async capture goroutines and the hotkey loop need this).
func pollUntil(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func appTestWindows() []domain.Window {
	return []domain.Window{
		{ID: 1, AppID: 1, AppName: "A", Title: "a", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 2, AppID: 2, AppName: "B", Title: "b", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 3, AppID: 3, AppName: "C", Title: "c", OnScreen: true, SpaceID: 1, ScreenID: 1},
	}
}

// recordingHotkeyEngine records Register/Unregister calls (the fake engine's
// registered map is unexported, so app tests need their own).
type recordingHotkeyEngine struct {
	mu         sync.Mutex
	registered map[int]hotkey.Chord
	ch         chan platform.HotkeyEvent
}

func newRecordingHotkeyEngine() *recordingHotkeyEngine {
	return &recordingHotkeyEngine{
		registered: map[int]hotkey.Chord{},
		ch:         make(chan platform.HotkeyEvent, 8),
	}
}

func (e *recordingHotkeyEngine) Register(id int, c hotkey.Chord) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registered[id] = c
	return nil
}

func (e *recordingHotkeyEngine) Unregister(id int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.registered, id)
	return nil
}

func (e *recordingHotkeyEngine) Events() <-chan platform.HotkeyEvent { return e.ch }
func (e *recordingHotkeyEngine) Keys() <-chan platform.KeyEvent      { return nil }
func (e *recordingHotkeyEngine) SetOpen(bool)                        {}
func (e *recordingHotkeyEngine) Close() error                        { return nil }

func (e *recordingHotkeyEngine) registeredIDs() map[int]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := map[int]bool{}
	for id := range e.registered {
		out[id] = true
	}
	return out
}

// recordingHotkeyPlatform swaps the fake's hotkey engine for a recording one.
type recordingHotkeyPlatform struct {
	*fake.Fake
	eng *recordingHotkeyEngine
}

func (p *recordingHotkeyPlatform) Hotkeys() platform.HotkeyEngine { return p.eng }

// focusRecorderPlatform records Focus calls behind its own mutex so a test can
// poll for the commit made by the hotkey-loop goroutine under -race.
type focusRecorderPlatform struct {
	*fake.Fake
	mu   sync.Mutex
	last domain.WindowID
}

func (p *focusRecorderPlatform) Focus(id domain.WindowID) error {
	p.mu.Lock()
	p.last = id
	p.mu.Unlock()
	return p.Fake.Focus(id)
}

func (p *focusRecorderPlatform) lastFocused() domain.WindowID {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

// thumbnailCall is one recorded ThumbnailDataURL invocation.
type thumbnailCall struct {
	id domain.WindowID
	px int
}

// thumbnailRecorderPlatform implements platform.ThumbnailSource, recording
// calls behind a mutex (captures run in goroutines).
type thumbnailRecorderPlatform struct {
	*fake.Fake
	mu    sync.Mutex
	calls []thumbnailCall
}

func (p *thumbnailRecorderPlatform) ThumbnailDataURL(id domain.WindowID, maxPx int) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, thumbnailCall{id: id, px: maxPx})
	return "data:image/png;base64,AAAA"
}

func (p *thumbnailRecorderPlatform) captureCalls() []thumbnailCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]thumbnailCall(nil), p.calls...)
}

// hapticRecorderPlatform implements platform.HapticFeedback with a tick counter.
type hapticRecorderPlatform struct {
	*fake.Fake
	mu    sync.Mutex
	ticks int
}

func (p *hapticRecorderPlatform) HapticTick() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ticks++
}

func (p *hapticRecorderPlatform) tickCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ticks
}

func TestGetSettings_ReturnsValidJSON(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	var s config.Settings
	if err := json.Unmarshal([]byte(a.GetSettings()), &s); err != nil {
		t.Fatalf("GetSettings not valid JSON: %v", err)
	}
	if len(s.Shortcuts) == 0 {
		t.Error("expected shortcuts in serialized settings")
	}
}

func TestSaveSettings_PersistsAndApplies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a := newApp(fake.New(), config.Default(), path)

	changed := config.Default()
	changed.Order = config.OrderAlphabetical
	b, _ := json.Marshal(changed)
	if err := a.SaveSettings(string(b)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if a.settings.Order != config.OrderAlphabetical {
		t.Errorf("settings not applied: order = %q", a.settings.Order)
	}
	reloaded, err := config.LoadFile(path)
	if err != nil || reloaded.Order != config.OrderAlphabetical {
		t.Errorf("settings not persisted: %+v err=%v", reloaded.Order, err)
	}
}

func TestSaveSettings_RejectsBadJSON(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	if err := a.SaveSettings("{not json"); err == nil {
		t.Error("expected error on malformed settings JSON")
	}
}

func TestSetPaused_PersistsAndAppliesToController(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	a := newApp(fake.New(), config.Default(), path)

	a.SetPaused(true)
	if !a.IsPaused() || !a.controller.Paused() {
		t.Fatalf("expected paused after SetPaused(true): app=%v ctrl=%v", a.IsPaused(), a.controller.Paused())
	}
	reloaded, err := config.LoadFile(path)
	if err != nil || !reloaded.Behavior.Paused {
		t.Errorf("paused not persisted: %+v err=%v", reloaded.Behavior, err)
	}

	if got := a.TogglePause(); got {
		t.Errorf("TogglePause should return false after resume, got %v", got)
	}
	if a.IsPaused() {
		t.Error("expected resumed after TogglePause")
	}
}

func TestRegisterHotkeys_DoesNotPanicWithFake(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	a.registerHotkeys()
	a.reRegisterHotkeys()
}

func TestRegisterHotkeys_SkipsDisabledAndInvalidChords(t *testing.T) {
	eng := newRecordingHotkeyEngine()
	p := &recordingHotkeyPlatform{Fake: fake.New(), eng: eng}
	s := config.Default()
	s.Shortcuts = []config.Shortcut{
		{ID: 1, Chord: "option+tab", Enabled: true},
		{ID: 2, Chord: "command+tab", Enabled: false},
		{ID: 3, Chord: "bogus++", Enabled: true},
	}
	a := newApp(p, s, "")

	a.registerHotkeys()
	if got := eng.registeredIDs(); len(got) != 1 || !got[1] {
		t.Fatalf("registerHotkeys registered %v, want only shortcut 1", got)
	}

	a.reRegisterHotkeys()
	if got := eng.registeredIDs(); len(got) != 1 || !got[1] {
		t.Fatalf("reRegisterHotkeys registered %v, want only shortcut 1", got)
	}
}

func TestHotkeyLoop_ActivatesController(t *testing.T) {
	p := &focusRecorderPlatform{Fake: fake.New()}
	p.SetWindows(appTestWindows())
	a := newApp(p, config.Default(), filepath.Join(t.TempDir(), "settings.json"))
	go a.hotkeyLoop()

	p.EmitHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	pollUntil(t, time.Second, "switcher to open on activate", a.controller.IsOpen)

	p.EmitHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
	pollUntil(t, time.Second, "release to focus a window", func() bool {
		return p.lastFocused() != 0
	})
}

func TestCapture_GuardsSkipPlatformCalls(t *testing.T) {
	p := &thumbnailRecorderPlatform{Fake: fake.New()}
	a := newApp(p, config.Default(), "")

	st := switcher.State{
		Style:      config.StyleAppIcons,
		Appearance: config.Default().Appearance,
		Entries:    []switcher.Entry{{WindowID: 10}, {WindowID: 20}},
		Selected:   0,
	}
	st.Appearance.PreviewSelected = false
	a.Show(st)
	time.Sleep(80 * time.Millisecond) // give any stray capture goroutine time to run
	if calls := p.captureCalls(); len(calls) != 0 {
		t.Fatalf("Show with app-icons style and preview off captured %v, want none", calls)
	}

	st.Appearance.PreviewSelected = true
	a.Update(st)
	pollUntil(t, time.Second, "one preview capture", func() bool {
		return len(p.captureCalls()) == 1
	})
	if c := p.captureCalls()[0]; c.id != 10 || c.px != 1024 {
		t.Fatalf("preview capture = %+v, want window 10 at 1024px", c)
	}

	st.Selected = -1
	a.Update(st)
	st.Selected = len(st.Entries)
	a.Update(st)
	time.Sleep(80 * time.Millisecond)
	if calls := p.captureCalls(); len(calls) != 1 {
		t.Fatalf("out-of-range Selected captured %v, want the single earlier call", calls)
	}
}

func TestTrayGlyph_MapsStyles(t *testing.T) {
	cases := map[config.MenubarIconStyle]string{
		config.MenubarIconDefault: "⌥⇥",
		config.MenubarIconOutline: "⧉",
		config.MenubarIconDot:     "●",
		"bogus":                   "⌥⇥",
	}
	for style, want := range cases {
		if got := trayGlyph(style); got != want {
			t.Errorf("trayGlyph(%q) = %q, want %q", style, got, want)
		}
	}
}

func TestSyncTray_NilTrayIsNoOp(t *testing.T) {
	// Without an injected Wails tray (tests, stub platforms) syncTray must not
	// touch the platform or panic.
	a := newApp(fake.New(), config.Default(), "")
	a.syncTray()
	a.SetPaused(true)
}

func TestSaveSettings_StartAtLoginTogglesLoginItem(t *testing.T) {
	f := fake.New()
	a := newApp(f, config.Default(), filepath.Join(t.TempDir(), "settings.json"))

	on := config.Default()
	on.Behavior.StartAtLogin = true
	onJSON, _ := json.Marshal(on)
	if err := a.SaveSettings(string(onJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if !f.Enabled() {
		t.Fatal("login item should be enabled after startAtLogin=true")
	}

	if err := a.SaveSettings(string(onJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if !f.Enabled() {
		t.Fatal("login item should stay enabled when the setting is unchanged")
	}

	off := config.Default()
	off.Behavior.StartAtLogin = false
	offJSON, _ := json.Marshal(off)
	if err := a.SaveSettings(string(offJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if f.Enabled() {
		t.Fatal("login item should be disabled after startAtLogin=false")
	}
}

func TestGetPermissions_ReportsStates(t *testing.T) {
	f := fake.New() // granted/granted by default
	a := newApp(f, config.Default(), "")

	want := `{"accessibility":"granted","screenRecording":"granted"}`
	if got := a.GetPermissions(); got != want {
		t.Fatalf("GetPermissions() = %s, want %s", got, want)
	}

	f.AccessibilityState = platform.PermDenied
	f.ScreenRecordingState = platform.PermUnknown
	want = `{"accessibility":"denied","screenRecording":"unknown"}`
	if got := a.GetPermissions(); got != want {
		t.Fatalf("GetPermissions() = %s, want %s", got, want)
	}
}

func TestPermissionRequests_Delegate(t *testing.T) {
	f := fake.New()
	a := newApp(f, config.Default(), "")

	a.RequestAccessibility()
	a.RequestScreenRecording()
	if len(f.RequestCalls) != 2 ||
		f.RequestCalls[0] != platform.PermAccessibility ||
		f.RequestCalls[1] != platform.PermScreenRecording {
		t.Fatalf("RequestCalls = %v, want [accessibility, screenRecording]", f.RequestCalls)
	}

	// Unknown kind and a platform without SettingsOpener: both must be no-ops.
	a.OpenPermissionSettings("bogus")
	a.OpenPermissionSettings("accessibility")
}

func TestGetVersion(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	v := a.GetVersion()
	if v == "" {
		t.Fatal("GetVersion() should not be empty")
	}
	if update.Newer(v, v) {
		t.Fatalf("version %q should parse as semver and not be newer than itself", v)
	}
}

func TestTrayMenuActions(t *testing.T) {
	// The menubar menu (wired in main.go) calls these App methods directly;
	// exercise the same paths the menu items trigger.
	f := fake.New()
	f.SetWindows(appTestWindows())
	a := newApp(f, config.Default(), "")

	// "Show Option Tab" activates the switcher as if the primary hotkey fired.
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if !a.controller.IsOpen() {
		t.Fatal("Show menu action should open the switcher")
	}

	// "Pause" suspends activation.
	a.TogglePause()
	if !a.IsPaused() {
		t.Fatal("Pause menu action should pause activation")
	}

	// "Settings…" opens the preferences window.
	a.OpenPreferences()
	if !a.prefsOpen {
		t.Fatal("Settings menu action should open the preferences window")
	}
}

func TestClosePreferencesWindow_ResetsState(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")

	a.prefsOpen = true
	a.closePreferencesWindow()
	if a.prefsOpen {
		t.Fatal("closePreferencesWindow should mark preferences closed")
	}
}

func TestShow_DismissesOpenPreferences(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	a.prefsOpen = true

	a.Show(switcher.State{Entries: []switcher.Entry{{WindowID: 1}}})
	if a.prefsOpen {
		t.Fatal("Show should dismiss the open preferences window")
	}
}

func TestUpdate_HapticOnlyOnSelectionMove(t *testing.T) {
	p := &hapticRecorderPlatform{Fake: fake.New()}
	a := newApp(p, config.Default(), "") // HapticFeedback defaults to true

	a.Update(switcher.State{Selected: 0}) // same as lastSelected: no tick
	if got := p.tickCount(); got != 0 {
		t.Fatalf("unchanged selection ticked %d times, want 0", got)
	}

	a.Update(switcher.State{Selected: 1})
	if got := p.tickCount(); got != 1 {
		t.Fatalf("selection move ticked %d times, want 1", got)
	}

	a.settings.Behavior.HapticFeedback = false
	a.Update(switcher.State{Selected: 2})
	if got := p.tickCount(); got != 1 {
		t.Fatalf("selection move with haptics off ticked %d times, want 1", got)
	}
}
