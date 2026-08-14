//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ApplicationServices -framework ServiceManagement -framework ScreenCaptureKit -framework Carbon -F/System/Library/PrivateFrameworks -framework SkyLight
#include <stdlib.h>
#include <CoreGraphics/CoreGraphics.h>
#include "darwin.h"
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/png"
	"strings"
	"sync"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

// darwinPlatform is the native macOS backend.
type darwinPlatform struct {
	hotkeys *darwinHotkeys
}

// New returns the native macOS platform backend.
func New() (Platform, error) {
	return &darwinPlatform{hotkeys: newDarwinHotkeys()}, nil
}

func (p *darwinPlatform) Name() string { return "darwin" }

// rawWindow mirrors the JSON produced by ot_list_windows_json.
type rawWindow struct {
	ID         uint64  `json:"id"`
	PID        int     `json:"pid"`
	App        string  `json:"app"`
	Bundle     string  `json:"bundle"`
	Title      string  `json:"title"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	OnScreen   bool    `json:"onscreen"`
	Minimized  bool    `json:"minimized"`
	Hidden     bool    `json:"hidden"`
	Fullscreen bool    `json:"fullscreen"`
	Screen     uint32  `json:"screen"`
	Space      uint64  `json:"space"`
	ZOrder     int     `json:"zorder"`
}

// zOrderBase anchors the synthetic LastFocused timestamps derived from window
// z-order. It sits far below the MRU tracker's reference point (time.Unix(1<<31))
// so any window the user has explicitly switched to in-session still sorts ahead
// of a z-order-only guess, while z-order still gives a real recency signal for
// windows that have never been touched through the switcher.
var zOrderBase = time.Unix(1_000_000_000, 0) // 2001-09-09 UTC

// mapRawWindows is the pure translation from the native JSON payload to domain
// windows. It is split out from Windows so it can be unit-tested without CGO.
func mapRawWindows(raws []rawWindow) []domain.Window {
	n := len(raws)
	out := make([]domain.Window, 0, n)
	for _, r := range raws {
		w := domain.Window{
			ID:         domain.WindowID(r.ID),
			AppID:      domain.AppID(r.PID),
			AppName:    r.App,
			BundleID:   r.Bundle,
			Title:      r.Title,
			PID:        r.PID,
			Bounds:     domain.Bounds{X: r.X, Y: r.Y, W: r.W, H: r.H},
			ScreenID:   domain.ScreenID(r.Screen),
			SpaceID:    domain.SpaceID(r.Space),
			OnScreen:   r.OnScreen,
			Minimized:  r.Minimized,
			Hidden:     r.Hidden,
			Fullscreen: r.Fullscreen,
		}
		// Frontmost (zorder 0) is most recent; later in the z-stack is older.
		if r.ZOrder >= 0 {
			w.LastFocused = zOrderBase.Add(time.Duration(n-r.ZOrder) * time.Second)
		}
		out = append(out, w)
	}
	return out
}

func (p *darwinPlatform) Windows() ([]domain.Window, error) {
	cstr := C.ot_list_windows_json()
	defer C.free(unsafe.Pointer(cstr))
	data := C.GoString(cstr)

	var raws []rawWindow
	if err := json.Unmarshal([]byte(data), &raws); err != nil {
		return nil, err
	}
	return mapRawWindows(raws), nil
}

func (p *darwinPlatform) Focus(id domain.WindowID) error {
	w := p.findPID(id)
	C.ot_focus_window(C.uint32_t(id), C.int(w))
	return nil
}

func (p *darwinPlatform) Close(id domain.WindowID) error {
	C.ot_close_window(C.uint32_t(id), C.int(p.findPID(id)))
	return nil
}

func (p *darwinPlatform) Minimize(id domain.WindowID) error {
	C.ot_minimize_window(C.uint32_t(id), C.int(p.findPID(id)))
	return nil
}

func (p *darwinPlatform) Fullscreen(id domain.WindowID) error {
	C.ot_fullscreen_window(C.uint32_t(id), C.int(p.findPID(id)))
	return nil
}

func (p *darwinPlatform) QuitApp(id domain.AppID) error {
	C.ot_quit_app(C.int(id))
	return nil
}

func (p *darwinPlatform) HideApp(id domain.AppID) error {
	C.ot_hide_app(C.int(id))
	return nil
}

// findPID resolves the owning pid of a window id. The CGWindowID alone is not
// enough for the AX APIs, which are addressed per pid. This uses a lightweight
// native lookup rather than a full enrichment pass, so the action path stays
// fast.
func (p *darwinPlatform) findPID(id domain.WindowID) int {
	return int(C.ot_window_pid(C.uint32_t(id)))
}

func (p *darwinPlatform) Thumbnail(id domain.WindowID, maxPx int) (image.Image, error) {
	cstr := C.ot_thumbnail_png_base64(C.uint32_t(id), C.int(maxPx))
	defer C.free(unsafe.Pointer(cstr))
	b64 := C.GoString(cstr)
	if b64 == "" {
		return nil, errNoThumbnail
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(raw))
}

// AppIcon returns the icon of the app owning pid as a base64 PNG data URL,
// scaled to at most maxPx. Empty string when unavailable.
func (p *darwinPlatform) AppIcon(pid, maxPx int) string {
	cstr := C.ot_app_icon_png_base64(C.int(pid), C.int(maxPx))
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

// ThumbnailDataURL returns a base64 PNG data URL snapshot of the window via
// ScreenCaptureKit, scaled to at most maxPx. Empty string when unavailable.
func (p *darwinPlatform) ThumbnailDataURL(id domain.WindowID, maxPx int) string {
	cstr := C.ot_thumbnail_dataurl(C.uint32_t(id), C.int(maxPx))
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr)
}

func (p *darwinPlatform) ActiveApp() domain.AppID {
	return domain.AppID(C.ot_active_app_pid())
}

func (p *darwinPlatform) ActiveSpace() domain.SpaceID {
	return domain.SpaceID(C.ot_active_space())
}

func (p *darwinPlatform) ActiveScreen() domain.ScreenID {
	return domain.ScreenID(C.ot_active_screen())
}

func (p *darwinPlatform) CursorScreen() domain.ScreenID {
	return domain.ScreenID(C.ot_cursor_screen())
}

func (p *darwinPlatform) Screens() []domain.Screen {
	cstr := C.ot_screens_json()
	defer C.free(unsafe.Pointer(cstr))
	var raws []rawScreen
	if err := json.Unmarshal([]byte(C.GoString(cstr)), &raws); err != nil || len(raws) == 0 {
		return []domain.Screen{{ID: 0, Main: true}}
	}
	out := make([]domain.Screen, 0, len(raws))
	for _, r := range raws {
		out = append(out, domain.Screen{
			ID:      domain.ScreenID(r.ID),
			Main:    r.Main,
			Bounds:  domain.Bounds{X: r.X, Y: r.Y, W: r.W, H: r.H},
			Visible: domain.Bounds{X: r.VX, Y: r.VY, W: r.VW, H: r.VH},
		})
	}
	return out
}

// rawScreen mirrors the JSON produced by ot_screens_json.
type rawScreen struct {
	ID   uint32  `json:"id"`
	Main bool    `json:"main"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
	VX   float64 `json:"vx"`
	VY   float64 `json:"vy"`
	VW   float64 `json:"vw"`
	VH   float64 `json:"vh"`
}

func (p *darwinPlatform) Accessibility() PermState {
	return permFromC(C.ot_perm_accessibility())
}

func (p *darwinPlatform) ScreenRecording() PermState {
	return permFromC(C.ot_perm_screen_recording())
}

func (p *darwinPlatform) Request(k PermKind) {
	switch k {
	case PermAccessibility:
		C.ot_request_accessibility()
	case PermScreenRecording:
		C.ot_request_screen_recording()
	}
}

// OpenPrivacySettings opens the System Settings privacy pane for the permission.
func (p *darwinPlatform) OpenPrivacySettings(k PermKind) {
	switch k {
	case PermAccessibility:
		C.ot_open_privacy_settings(0)
	case PermScreenRecording:
		C.ot_open_privacy_settings(1)
	}
}

func (p *darwinPlatform) Enabled() bool {
	return C.ot_login_item_enabled() == 1
}

func (p *darwinPlatform) SetEnabled(v bool) error {
	flag := C.int(0)
	if v {
		flag = 1
	}
	C.ot_login_item_set(flag)
	return nil
}

func (p *darwinPlatform) Hotkeys() HotkeyEngine { return p.hotkeys }

// HideDockIcon implements platform.DockHider: accessory apps have no Dock icon.
func (p *darwinPlatform) HideDockIcon() { C.ot_hide_dock_icon() }

// ActivateForPrefs implements platform.AppActivator: the preferences window
// needs keyboard focus, so the accessory app flips to regular and activates.
func (p *darwinPlatform) ActivateForPrefs() { C.ot_activate_prefs() }

// HideAppIfActive implements platform.AppActivator, dropping an accidental
// click-activation once the overlay hides.
func (p *darwinPlatform) HideAppIfActive() { C.ot_app_hide() }

// FitOverlayToScreen implements platform.OverlayWindowFitter, sizing the
// transparent window to the chosen screen so the panel can use the full area.
func (p *darwinPlatform) FitOverlayToScreen(win unsafe.Pointer, screen domain.ScreenID) {
	C.ot_window_fit_screen(win, C.uint32_t(screen))
}

// HapticTick implements platform.HapticFeedback with a subtle alignment tap.
func (p *darwinPlatform) HapticTick() { C.ot_haptic_tick() }

// WarpCursorToWindow implements platform.CursorWarper (cursor follows focus).
func (p *darwinPlatform) WarpCursorToWindow(id domain.WindowID) error {
	C.ot_warp_cursor(C.uint32_t(id))
	return nil
}

func permFromC(v C.int) PermState {
	if v == 1 {
		return PermGranted
	}
	return PermDenied
}

// ---- Hotkey engine ----

// eventQueue is an unbounded ordered hand-off from the C tap thread to a Go
// consumer. The tap thread must never block (a stalled tap gets disabled by the
// system), so the exported callbacks append here and a feeder goroutine does
// the (potentially blocking) channel send. Unlike the old non-blocking send
// this never drops events — a dropped release used to leave the switcher stuck.
type eventQueue[T any] struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []T
	closed bool
}

func newEventQueue[T any]() *eventQueue[T] {
	q := &eventQueue[T]{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *eventQueue[T]) push(v T) {
	q.mu.Lock()
	if !q.closed {
		q.items = append(q.items, v)
		q.cond.Signal()
	}
	q.mu.Unlock()
}

// pop returns the next item, blocking until one is available or the queue
// closes (ok=false).
func (q *eventQueue[T]) pop() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		var zero T
		return zero, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, true
}

func (q *eventQueue[T]) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

// darwinHotkeys drives the CGEventTap in darwin.m and streams events to Go.
type darwinHotkeys struct {
	events   *eventQueue[HotkeyEvent]
	keys     *eventQueue[KeyEvent]
	eventsCh chan HotkeyEvent
	keysCh   chan KeyEvent
	started  bool
}

func newDarwinHotkeys() *darwinHotkeys {
	return &darwinHotkeys{
		events:   newEventQueue[HotkeyEvent](),
		keys:     newEventQueue[KeyEvent](),
		eventsCh: make(chan HotkeyEvent, 8),
		keysCh:   make(chan KeyEvent, 32),
	}
}

// activeEngine is the engine the exported C callbacks deliver to. There is a
// single hotkey engine per process.
var activeEngine *darwinHotkeys

func (h *darwinHotkeys) ensureStarted() {
	if h.started {
		return
	}
	activeEngine = h
	go func() {
		for {
			ev, ok := h.events.pop()
			if !ok {
				close(h.eventsCh)
				return
			}
			h.eventsCh <- ev
		}
	}()
	go func() {
		for {
			ev, ok := h.keys.pop()
			if !ok {
				close(h.keysCh)
				return
			}
			h.keysCh <- ev
		}
	}()
	C.ot_hotkey_start()
	h.started = true
}

func (h *darwinHotkeys) Register(id int, c hotkey.Chord) error {
	h.ensureStarted()
	keycode, ok := keycodeFor(c.Key)
	if !ok {
		return errUnknownKey
	}
	mods := modMask(c)
	// Base chord (advance / activate) plus a shift variant (reverse).
	C.ot_hotkey_register(C.int(id), C.uint64_t(mods), C.uint16_t(keycode), 0)
	C.ot_hotkey_register(C.int(id), C.uint64_t(mods), C.uint16_t(keycode), 1)
	return nil
}

func (h *darwinHotkeys) Unregister(id int) error {
	C.ot_hotkey_unregister(C.int(id))
	return nil
}

func (h *darwinHotkeys) Events() <-chan HotkeyEvent { return h.eventsCh }

func (h *darwinHotkeys) Keys() <-chan KeyEvent { return h.keysCh }

// SetOpen mirrors the overlay's visibility into the native tap, which consumes
// and forwards all keyboard input while the switcher is open.
func (h *darwinHotkeys) SetOpen(open bool) {
	if !h.started {
		return
	}
	flag := C.int(0)
	if open {
		flag = 1
	}
	C.ot_hotkey_set_open(flag)
}

func (h *darwinHotkeys) Close() error {
	C.ot_hotkey_stop()
	h.events.close()
	h.keys.close()
	return nil
}

// CoreGraphics modifier-flag masks (stable public CGEventFlags values), kept
// as Go constants so chordFromCapture stays unit-testable.
const (
	cgFlagControl = 0x00040000
	cgFlagShift   = 0x00020000
	cgFlagOption  = 0x00080000
	cgFlagCommand = 0x00100000
)

// pendingCapture receives the one-shot recorded chord ("" = cancelled).
var (
	captureMu      sync.Mutex
	pendingCapture chan string
)

// chordFromCapture converts a tap-captured (modifier flags, keycode) pair to
// the canonical chord string, or "" for the cancel sentinel or unknown keys.
func chordFromCapture(flags uint64, keycode uint16) string {
	if keycode == 0xFFFF {
		return ""
	}
	var c hotkey.Chord
	if flags&cgFlagControl != 0 {
		c.Mods = c.Mods.With(hotkey.ModControl)
	}
	if flags&cgFlagOption != 0 {
		c.Mods = c.Mods.With(hotkey.ModOption)
	}
	if flags&cgFlagShift != 0 {
		c.Mods = c.Mods.With(hotkey.ModShift)
	}
	if flags&cgFlagCommand != 0 {
		c.Mods = c.Mods.With(hotkey.ModCommand)
	}
	if c.Mods.Len() == 0 {
		return ""
	}
	for name, code := range macKeycodes {
		if code == keycode {
			c.Key = name
			return c.String()
		}
	}
	return ""
}

//export goHotkeyCaptured
func goHotkeyCaptured(modflags C.uint64_t, keycode C.uint16_t) {
	captureMu.Lock()
	ch := pendingCapture
	pendingCapture = nil
	captureMu.Unlock()
	if ch != nil {
		ch <- chordFromCapture(uint64(modflags), uint16(keycode))
	}
}

// CaptureShortcut implements platform.ShortcutCapturer: arms the event tap's
// one-shot recording and blocks until a chord, Escape, or the timeout.
func (p *darwinPlatform) CaptureShortcut(timeout time.Duration) string {
	p.hotkeys.ensureStarted()
	ch := make(chan string, 1)
	captureMu.Lock()
	if prev := pendingCapture; prev != nil {
		prev <- "" // resolve a stale capture so its caller unblocks
	}
	pendingCapture = ch
	captureMu.Unlock()
	C.ot_hotkey_capture_start()
	select {
	case s := <-ch:
		return s
	case <-time.After(timeout):
		C.ot_hotkey_capture_stop()
		captureMu.Lock()
		if pendingCapture == ch {
			pendingCapture = nil
		}
		captureMu.Unlock()
		return ""
	}
}

// CancelShortcutCapture implements platform.ShortcutCapturer, disarming a
// pending recording (e.g. the recorder input lost focus).
func (p *darwinPlatform) CancelShortcutCapture() {
	C.ot_hotkey_capture_stop()
	captureMu.Lock()
	ch := pendingCapture
	pendingCapture = nil
	captureMu.Unlock()
	if ch != nil {
		ch <- ""
	}
}

//export goHotkeyEvent
func goHotkeyEvent(kind, id C.int) {
	if activeEngine == nil {
		return
	}
	ev := HotkeyEvent{ShortcutID: int(id)}
	switch int(kind) {
	case 0:
		ev.Kind = HotkeyActivate
	case 1:
		ev.Kind = HotkeyAdvance
	case 2:
		ev.Kind = HotkeyReverse
	case 3:
		ev.Kind = HotkeyRelease
	case 4:
		ev.Kind = HotkeyCancel
	}
	activeEngine.events.push(ev)
}

//export goKeyEvent
func goKeyEvent(keycode C.int, flags C.uint64_t, text *C.char) {
	if activeEngine == nil {
		return
	}
	activeEngine.keys.push(keyEventFromTap(uint16(keycode), uint64(flags), C.GoString(text)))
}

// specialKeys maps macOS virtual keycodes to the DOM KeyboardEvent.key names
// the frontend keymap matches on (the tap's unicode text for these is a
// control character, not a printable key name).
var specialKeys = map[uint16]string{
	36:  "Enter",
	48:  "Tab",
	51:  "Backspace",
	53:  "Escape",
	123: "ArrowLeft",
	124: "ArrowRight",
	125: "ArrowDown",
	126: "ArrowUp",
}

// domCodes maps macOS virtual keycodes to DOM KeyboardEvent.code (physical key)
// values. Only the codes the frontend keymap acts on need to be exact: letters
// and digits map to KeyX/DigitN, plus the named special keys.
var domCodes = map[uint16]string{
	36: "Enter", 48: "Tab", 49: "Space", 50: "Backquote", 51: "Backspace", 53: "Escape",
	123: "ArrowLeft", 124: "ArrowRight", 125: "ArrowDown", 126: "ArrowUp",
}

func init() {
	for name, code := range macKeycodes {
		k := string(name)
		if len(k) == 1 && k[0] >= 'a' && k[0] <= 'z' {
			domCodes[code] = "Key" + strings.ToUpper(k)
		} else if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
			domCodes[code] = "Digit" + k
		}
	}
}

// keyEventFromTap builds the DOM-shaped KeyEvent the frontend keymap consumes
// from a tap-captured key press.
func keyEventFromTap(keycode uint16, flags uint64, text string) KeyEvent {
	key := text
	if special, ok := specialKeys[keycode]; ok {
		key = special
	}
	return KeyEvent{
		Key:   key,
		Code:  domCodes[keycode],
		Shift: flags&cgFlagShift != 0,
		Ctrl:  flags&cgFlagControl != 0,
		Alt:   flags&cgFlagOption != 0,
		Meta:  flags&cgFlagCommand != 0,
	}
}

// modMask converts a chord's modifiers to the CGEventFlags mask (excluding
// shift, which the C layer handles via its withShift variant).
func modMask(c hotkey.Chord) uint64 {
	var mask uint64
	if c.Mods.Has(hotkey.ModControl) {
		mask |= uint64(C.kCGEventFlagMaskControl)
	}
	if c.Mods.Has(hotkey.ModOption) {
		mask |= uint64(C.kCGEventFlagMaskAlternate)
	}
	if c.Mods.Has(hotkey.ModCommand) {
		mask |= uint64(C.kCGEventFlagMaskCommand)
	}
	if c.Mods.Has(hotkey.ModShift) {
		mask |= uint64(C.kCGEventFlagMaskShift)
	}
	return mask
}

// keycodeFor maps a normalized hotkey key to its macOS virtual keycode.
func keycodeFor(k hotkey.Key) (uint16, bool) {
	code, ok := macKeycodes[k]
	return code, ok
}

// macKeycodes are the ANSI virtual keycodes for the keys a chord may bind.
var macKeycodes = map[hotkey.Key]uint16{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4, "g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
	"b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17, "o": 31, "u": 32,
	"i": 34, "p": 35, "l": 37, "j": 38, "k": 40, "n": 45, "m": 46,
	"1": 18, "2": 19, "3": 20, "4": 21, "5": 23, "6": 22, "7": 26, "8": 28, "9": 25, "0": 29,
	"return": 36, "tab": 48, "space": 49, "grave": 50, "escape": 53,
	"left": 123, "right": 124, "down": 125, "up": 126,
}
