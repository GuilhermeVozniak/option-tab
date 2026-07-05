// Package platform defines the port between the pure-Go switcher core and the
// operating system. Backends (a macOS CGO adapter, a portable stub, and an
// in-memory fake for tests) implement these interfaces. The switcher depends
// only on the interfaces, never on a concrete backend, so all of its logic is
// testable without touching the OS.
package platform

import (
	"image"

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

// TrayCommand identifies a menubar (status item) action chosen by the user.
type TrayCommand int

const (
	// TrayPreferences opens the preferences UI.
	TrayPreferences TrayCommand = iota
	// TrayTogglePause suspends or resumes activation.
	TrayTogglePause
	// TrayQuit quits the application.
	TrayQuit
)

// Tray is an optional menubar status-item controller. Only the native macOS
// backend implements it; the stub and fake omit it, so consumers type-assert.
type Tray interface {
	// InstallTray shows the menubar icon and returns the channel of user
	// commands. The wiring layer guards against calling it more than once.
	InstallTray() <-chan TrayCommand
	// SetTrayPaused updates the Pause/Resume menu item to reflect state.
	SetTrayPaused(paused bool)
	// SetTrayStyle switches the status-item glyph ("default", "outline", "dot").
	SetTrayStyle(style string)
	// RemoveTray hides the menubar icon.
	RemoveTray()
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

// HotkeyEngine registers global chords and streams their events.
type HotkeyEngine interface {
	Register(shortcutID int, chord hotkey.Chord) error
	Unregister(shortcutID int) error
	Events() <-chan HotkeyEvent
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

// WindowModer flips the single app window between the borderless floating
// overlay and a regular titled preferences window. Optional: only the native
// macOS backend implements it, so consumers type-assert for it.
type WindowModer interface {
	SetPrefsWindowMode(on bool)
}

// OverlayWindowPreparer makes the app window a fully transparent overlay so
// only the rendered switcher panel is visible (no opaque window backdrop),
// and sizes it to the screen before each show. Optional: only the native
// macOS backend implements it, so consumers type-assert for it.
type OverlayWindowPreparer interface {
	PrepareOverlayWindow()
	FitOverlayToScreen()
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
