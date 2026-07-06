package main

import (
	"context"
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

// trayRecorderPlatform implements platform.Tray and records every call.
type trayRecorderPlatform struct {
	*fake.Fake
	mu       sync.Mutex
	installs int
	removes  int
	styles   []string
	paused   []bool
	cmds     chan platform.TrayCommand
}

func newTrayRecorderPlatform() *trayRecorderPlatform {
	return &trayRecorderPlatform{Fake: fake.New(), cmds: make(chan platform.TrayCommand)}
}

func (p *trayRecorderPlatform) InstallTray() <-chan platform.TrayCommand {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.installs++
	return p.cmds
}

func (p *trayRecorderPlatform) RemoveTray() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.removes++
}

func (p *trayRecorderPlatform) SetTrayStyle(style string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.styles = append(p.styles, style)
}

func (p *trayRecorderPlatform) SetTrayPaused(paused bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = append(p.paused, paused)
}

func (p *trayRecorderPlatform) counts() (installs, removes int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.installs, p.removes
}

func (p *trayRecorderPlatform) lastStyle() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.styles) == 0 {
		return ""
	}
	return p.styles[len(p.styles)-1]
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

func TestSyncTray_InstallRemoveAndStyle(t *testing.T) {
	p := newTrayRecorderPlatform()
	s := config.Default()
	s.Behavior.ShowMenubarIcon = false
	a := newApp(p, s, "")

	on := config.Default()
	on.Behavior.ShowMenubarIcon = true
	on.Behavior.MenubarIconStyle = config.MenubarIconDot
	onJSON, _ := json.Marshal(on)
	if err := a.SaveSettings(string(onJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	installs, removes := p.counts()
	if installs != 1 || removes != 0 {
		t.Fatalf("after enabling: installs=%d removes=%d, want 1/0", installs, removes)
	}
	if !a.trayInstalled {
		t.Fatal("trayInstalled should be true after enabling the menubar icon")
	}
	if got := p.lastStyle(); got != "dot" {
		t.Fatalf("SetTrayStyle = %q, want %q", got, "dot")
	}

	// Saving the same settings again must not reinstall.
	if err := a.SaveSettings(string(onJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if installs, _ := p.counts(); installs != 1 {
		t.Fatalf("repeat save installed again: installs=%d, want 1", installs)
	}

	off := on
	off.Behavior.ShowMenubarIcon = false
	offJSON, _ := json.Marshal(off)
	if err := a.SaveSettings(string(offJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	installs, removes = p.counts()
	if installs != 1 || removes != 1 {
		t.Fatalf("after disabling: installs=%d removes=%d, want 1/1", installs, removes)
	}
	if a.trayInstalled {
		t.Fatal("trayInstalled should be false after disabling the menubar icon")
	}

	// Saving disabled settings again must not remove twice.
	if err := a.SaveSettings(string(offJSON)); err != nil {
		t.Fatalf("SaveSettings error: %v", err)
	}
	if _, removes := p.counts(); removes != 1 {
		t.Fatalf("repeat save removed again: removes=%d, want 1", removes)
	}
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

func TestTrayLoop_RoutesCommands(t *testing.T) {
	f := fake.New()
	f.SetWindows(appTestWindows())
	a := newApp(f, config.Default(), "")

	// Route each command in its own synchronous loop run; TrayShow goes first
	// because pausing would gate activation.
	route := func(cmd platform.TrayCommand) {
		cmds := make(chan platform.TrayCommand, 1)
		cmds <- cmd
		close(cmds)
		a.trayLoop(cmds)
	}

	route(platform.TrayShow)
	if !a.controller.IsOpen() {
		t.Fatal("TrayShow should open the switcher")
	}

	route(platform.TrayTogglePause)
	if !a.IsPaused() {
		t.Fatal("TrayTogglePause should pause activation")
	}

	route(platform.TrayPreferences)
	if !a.prefsOpen {
		t.Fatal("TrayPreferences should open the preferences window")
	}
}

func TestBeforeClose_PrefsOpenPreventsQuit(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")

	a.prefsOpen = true
	if !a.beforeClose(context.Background()) {
		t.Fatal("beforeClose with preferences open should prevent quitting")
	}
	if a.prefsOpen {
		t.Fatal("beforeClose should have dismissed the preferences window")
	}

	if a.beforeClose(context.Background()) {
		t.Fatal("beforeClose with preferences closed should allow quitting")
	}
}

func TestShow_DismissesOpenPreferences(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	a.prefsOpen = true

	a.Show(switcher.State{Entries: []switcher.Entry{{WindowID: 1}}})
	if a.prefsOpen {
		t.Fatal("Show should dismiss the open preferences window (shared window)")
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
