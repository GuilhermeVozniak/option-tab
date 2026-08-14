// Package platform defines the port between the pure-Go switcher core and the
// operating system. Backends (a macOS CGO adapter, a portable stub, and an
// in-memory fake for tests) implement these interfaces. The switcher depends
// only on the interfaces, never on a concrete backend, so all of its logic is
// testable without touching the OS.
package platform

import (
	"image"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

// WindowSource enumerates the windows currently known to the system.
type WindowSource interface {
	Windows() ([]domain.Window, error)
}

// Focuser manipulates windows and their owning applications.
type Focuser interface {
	Focus(domain.WindowID) error
	Close(domain.WindowID) error
	Minimize(domain.WindowID) error
	Fullscreen(domain.WindowID) error
	QuitApp(domain.AppID) error
	HideApp(domain.AppID) error
}

// Thumbnailer captures a snapshot image of a window, scaled so its largest side
// is at most maxPx pixels.
type Thumbnailer interface {
	Thumbnail(id domain.WindowID, maxPx int) (image.Image, error)
}

// IconSource provides application icons as base64 PNG data URLs. It is an
// optional capability: the native macOS backend implements it, while the stub
// and fake do not, so consumers must type-assert for it.
type IconSource interface {
	AppIcon(pid, maxPx int) string
}

// ThumbnailSource provides window snapshots as base64 PNG data URLs. Like
// IconSource it is optional: only the native macOS backend implements it.
type ThumbnailSource interface {
	ThumbnailDataURL(id domain.WindowID, maxPx int) string
}

// CursorWarper moves the mouse cursor to a window, used by the "cursor follows
// focus" behavior. Optional: only the native macOS backend implements it.
type CursorWarper interface {
	WarpCursorToWindow(domain.WindowID) error
}

// Environment reports the current foreground context used for filtering.
type Environment interface {
	ActiveApp() domain.AppID
	ActiveSpace() domain.SpaceID
	Screens() []domain.Screen
	ActiveScreen() domain.ScreenID
	CursorScreen() domain.ScreenID
}

// HotkeyEventKind classifies a hotkey engine event.
type HotkeyEventKind int

const (
	// HotkeyActivate fires when a registered chord is first pressed.
	HotkeyActivate HotkeyEventKind = iota
	// HotkeyAdvance fires when the trigger key is tapped again while the
	// modifier is still held (move to the next window).
	HotkeyAdvance
	// HotkeyReverse fires when shift+trigger is tapped while held (previous).
	HotkeyReverse
	// HotkeyRelease fires when the chord's modifiers are released (commit).
	HotkeyRelease
	// HotkeyCancel fires when the user presses Escape while the switcher is up.
	HotkeyCancel
)

// HotkeyEvent is emitted by a HotkeyEngine to drive the switcher.
type HotkeyEvent struct {
	Kind       HotkeyEventKind
	ShortcutID int
}

// KeyEvent is a raw key press captured by the native event tap and forwarded
// to the frontend while the switcher overlay is open. The overlay never
// activates the app (so it never becomes key), which makes the tap the only
// keyboard source: navigation, actions, and type-to-search all ride on these.
// Key/Code mirror the DOM KeyboardEvent fields the frontend keymap consumes.
type KeyEvent struct {
	Key   string `json:"key"`
	Code  string `json:"code"`
	Shift bool   `json:"shift"`
	Ctrl  bool   `json:"ctrl"`
	Alt   bool   `json:"alt"`
	Meta  bool   `json:"meta"`
}

// HotkeyEngine registers global chords and streams their events.
type HotkeyEngine interface {
	Register(shortcutID int, chord hotkey.Chord) error
	Unregister(shortcutID int) error
	Events() <-chan HotkeyEvent
	// Keys streams the raw key presses captured while the overlay is open.
	// It may return nil on backends without key forwarding (stub/fake).
	Keys() <-chan KeyEvent
	// SetOpen tells the engine whether the switcher overlay is currently open.
	// While open the engine consumes every key press (so typing never leaks
	// into the previously active app) and forwards it on Keys.
	SetOpen(open bool)
	Close() error
}

// PermState is the grant state of an OS permission.
type PermState int

const (
	PermUnknown PermState = iota
	PermGranted
	PermDenied
)

// PermKind identifies an OS permission the switcher may need.
type PermKind int

const (
	// PermAccessibility is required to focus/close/minimize windows.
	PermAccessibility PermKind = iota
	// PermScreenRecording is required to capture window thumbnails.
	PermScreenRecording
)

// Permissions reports and requests OS permissions.
type Permissions interface {
	Accessibility() PermState
	ScreenRecording() PermState
	Request(PermKind)
}

// LoginItem controls whether the app launches at login.
type LoginItem interface {
	Enabled() bool
	SetEnabled(bool) error
}

// DockHider hides the app's Dock icon (accessory activation policy), so the
// switcher behaves like a background utility. Optional: only the native macOS
// backend implements it, so consumers type-assert for it.
type DockHider interface {
	HideDockIcon()
}

// AppActivator flips the accessory app to a regular, activated app (used when
// the preferences window opens, which needs keyboard focus), and drops
// accidental activations (e.g. a click on the overlay) once the overlay hides.
// Optional: only the native macOS backend implements it.
type AppActivator interface {
	ActivateForPrefs()
	HideAppIfActive()
}

// OverlayWindowFitter sizes the transparent overlay window to the chosen screen
// before each show. The win handle is the Wails window's native NSWindow
// pointer. Optional: only the native macOS backend implements it, so consumers
// type-assert for it.
type OverlayWindowFitter interface {
	// FitOverlayToScreen sizes the window to the screen with the given display
	// id (0 = keep the window's current screen).
	FitOverlayToScreen(win unsafe.Pointer, screen domain.ScreenID)
}

// HapticFeedback performs a subtle trackpad tap, used when the switcher
// selection changes. Optional: only the native macOS backend implements it.
type HapticFeedback interface {
	HapticTick()
}

// ShortcutCapturer records the next chord pressed anywhere via the native
// event tap — including chords the OS or the app's own hotkeys would
// otherwise swallow (Command+Tab, the active switcher chord). Optional: only
// the native macOS backend implements it.
type ShortcutCapturer interface {
	// CaptureShortcut arms a one-shot system-wide capture and blocks until a
	// chord is pressed, Escape cancels, or the timeout elapses ("" on
	// cancel/timeout).
	CaptureShortcut(timeout time.Duration) string
	// CancelShortcutCapture disarms a pending capture (e.g. the recorder
	// input lost focus).
	CancelShortcutCapture()
}

// SettingsOpener opens the OS settings pane for a permission, used to guide the
// user when a permission was denied and the system prompt no longer appears.
// Optional: only the native macOS backend implements it, so consumers
// type-assert for it.
type SettingsOpener interface {
	OpenPrivacySettings(PermKind)
}

// Platform aggregates every capability a backend provides. The wiring layer
// holds a Platform; the switcher consumes the narrower interfaces above.
type Platform interface {
	WindowSource
	Focuser
	Thumbnailer
	Environment
	Permissions
	LoginItem
	Hotkeys() HotkeyEngine
	// Name identifies the backend (e.g. "darwin", "stub", "fake").
	Name() string
}
