//go:build darwin

package platform

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa -framework CoreGraphics -framework ApplicationServices -framework ServiceManagement
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
	ID       uint64  `json:"id"`
	PID      int     `json:"pid"`
	App      string  `json:"app"`
	Bundle   string  `json:"bundle"`
	Title    string  `json:"title"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	W        float64 `json:"w"`
	H        float64 `json:"h"`
	OnScreen bool    `json:"onscreen"`
}

func (p *darwinPlatform) Windows() ([]domain.Window, error) {
	cstr := C.ot_list_windows_json()
	defer C.free(unsafe.Pointer(cstr))
	data := C.GoString(cstr)

	var raws []rawWindow
	if err := json.Unmarshal([]byte(data), &raws); err != nil {
		return nil, err
	}
	out := make([]domain.Window, 0, len(raws))
	for _, r := range raws {
		out = append(out, domain.Window{
			ID:       domain.WindowID(r.ID),
			AppID:    domain.AppID(r.PID),
			AppName:  r.App,
			BundleID: r.Bundle,
			Title:    r.Title,
			PID:      r.PID,
			Bounds:   domain.Bounds{X: r.X, Y: r.Y, W: r.W, H: r.H},
			OnScreen: r.OnScreen,
		})
	}
	return out, nil
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

func (p *darwinPlatform) QuitApp(id domain.AppID) error {
	C.ot_quit_app(C.int(id))
	return nil
}

func (p *darwinPlatform) HideApp(id domain.AppID) error {
	C.ot_hide_app(C.int(id))
	return nil
}

// findPID resolves the owning pid of a window id via a fresh enumeration. The
// CGWindowID alone is not enough for the AX APIs, which are addressed per pid.
func (p *darwinPlatform) findPID(id domain.WindowID) int {
	wins, err := p.Windows()
	if err != nil {
		return 0
	}
	for _, w := range wins {
		if w.ID == id {
			return w.PID
		}
	}
	return 0
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

func (p *darwinPlatform) ActiveApp() domain.AppID {
	return domain.AppID(C.ot_active_app_pid())
}

// Space/screen detail is not resolved natively in this version; on-screen
// enumeration already scopes to the current space, and the default filters use
// "all" so these zero values are inert.
func (p *darwinPlatform) ActiveSpace() domain.SpaceID   { return 0 }
func (p *darwinPlatform) ActiveScreen() domain.ScreenID { return 0 }
func (p *darwinPlatform) CursorScreen() domain.ScreenID { return 0 }

func (p *darwinPlatform) Screens() []domain.Screen {
	return []domain.Screen{{ID: 0, Main: true}}
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
