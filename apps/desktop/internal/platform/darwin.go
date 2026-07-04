//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ApplicationServices -framework ServiceManagement -framework ScreenCaptureKit
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
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

// darwinPlatform is the native macOS backend.
type darwinPlatform struct {
	hotkeys *darwinHotkeys
	tray    *darwinTray
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

// ---- Menubar tray ----

func (p *darwinPlatform) InstallTray() <-chan TrayCommand {
	if p.tray == nil {
		p.tray = newDarwinTray()
	}
	activeTrayChan = p.tray.ch
	C.ot_tray_install()
	return p.tray.ch
}

func (p *darwinPlatform) SetTrayStyle(style string) {
	cs := C.CString(style)
	defer C.free(unsafe.Pointer(cs))
	C.ot_tray_set_style(cs)
}

// HideDockIcon implements platform.DockHider: accessory apps have no Dock icon.
func (p *darwinPlatform) HideDockIcon() { C.ot_hide_dock_icon() }

// SetPrefsWindowMode implements platform.WindowModer, flipping the single
// window between overlay and titled-preferences chrome.
func (p *darwinPlatform) SetPrefsWindowMode(on bool) {
	flag := C.int(0)
	if on {
		flag = 1
	}
	C.ot_window_set_prefs_mode(flag)
}

// WarpCursorToWindow implements platform.CursorWarper (cursor follows focus).
func (p *darwinPlatform) WarpCursorToWindow(id domain.WindowID) error {
	C.ot_warp_cursor(C.uint32_t(id))
	return nil
}

func (p *darwinPlatform) SetTrayPaused(paused bool) {
	flag := C.int(0)
	if paused {
		flag = 1
	}
	C.ot_tray_set_paused(flag)
}

func (p *darwinPlatform) RemoveTray() { C.ot_tray_remove() }

// darwinTray streams menubar commands from the native status item to Go.
type darwinTray struct {
	ch chan TrayCommand
}

func newDarwinTray() *darwinTray { return &darwinTray{ch: make(chan TrayCommand, 8)} }

// activeTrayChan is the channel the exported C callback delivers to. There is a
// single tray per process.
var activeTrayChan chan TrayCommand

//export goTrayCommand
func goTrayCommand(cmd C.int) {
	if activeTrayChan == nil {
		return
	}
	var c TrayCommand
	switch int(cmd) {
	case 0:
		c = TrayPreferences
	case 1:
		c = TrayTogglePause
	case 2:
		c = TrayQuit
	}
	select {
	case activeTrayChan <- c:
	default:
	}
}

func permFromC(v C.int) PermState {
	if v == 1 {
		return PermGranted
	}
	return PermDenied
}

// ---- Hotkey engine ----

// darwinHotkeys drives the CGEventTap in darwin.m and streams events to Go.
type darwinHotkeys struct {
	ch      chan HotkeyEvent
	started bool
}

func newDarwinHotkeys() *darwinHotkeys {
	return &darwinHotkeys{ch: make(chan HotkeyEvent, 64)}
}

// activeHotkeyChan is the channel the exported C callback delivers to. There is
// a single hotkey engine per process.
var activeHotkeyChan chan HotkeyEvent

func (h *darwinHotkeys) ensureStarted() {
	if h.started {
		return
	}
	activeHotkeyChan = h.ch
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

func (h *darwinHotkeys) Events() <-chan HotkeyEvent { return h.ch }

func (h *darwinHotkeys) Close() error {
	C.ot_hotkey_stop()
	return nil
}

//export goHotkeyEvent
func goHotkeyEvent(kind, id C.int) {
	if activeHotkeyChan == nil {
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
	select {
	case activeHotkeyChan <- ev:
	default: // drop if the consumer is behind rather than block the tap thread
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
