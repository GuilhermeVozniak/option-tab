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
	QuitApp(domain.AppID) error
	HideApp(domain.AppID) error
}

// Thumbnailer captures a snapshot image of a window, scaled so its largest side
// is at most maxPx pixels.
type Thumbnailer interface {
	Thumbnail(id domain.WindowID, maxPx int) (image.Image, error)
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
