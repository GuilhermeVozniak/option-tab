// Package domain holds the core value types of the window switcher: the
// windows, apps, screens and spaces the switcher reasons about. These types
// carry no platform dependencies and only pure, easily-tested behavior.
package domain

import "time"

// WindowID uniquely identifies a window for the lifetime of the process.
// On macOS this maps to a CGWindowID.
type WindowID uint64

// AppID uniquely identifies a running application. On macOS this maps to the
// owning process id (pid).
type AppID int

// SpaceID identifies a macOS Space (virtual desktop). Zero means "unknown".
type SpaceID uint64

// ScreenID identifies a physical display. Zero means "unknown".
type ScreenID uint32

// Bounds is an axis-aligned rectangle in global screen coordinates, origin
// top-left, measured in points.
type Bounds struct {
	X, Y, W, H float64
}

// Area returns the rectangle's area. Negative dimensions clamp to zero so a
// degenerate rectangle never reports negative area.
func (b Bounds) Area() float64 {
	w, h := b.W, b.H
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return w * h
}

// Center returns the rectangle's center point.
func (b Bounds) Center() (x, y float64) {
	return b.X + b.W/2, b.Y + b.H/2
}

// ContainsPoint reports whether (x,y) lies within the rectangle. The top and
// left edges are inclusive; the bottom and right edges are exclusive.
func (b Bounds) ContainsPoint(x, y float64) bool {
	return x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H
}

// Intersects reports whether two rectangles overlap on a positive area. Merely
// touching edges does not count as intersecting.
func (b Bounds) Intersects(o Bounds) bool {
	return b.X < o.X+o.W && o.X < b.X+b.W && b.Y < o.Y+o.H && o.Y < b.Y+b.H
}

// App describes a running application that owns windows.
type App struct {
	ID       AppID
	Name     string
	BundleID string
	Hidden   bool
}

// Window describes a single on-screen (or off-screen) window.
type Window struct {
	ID          WindowID
	AppID       AppID
	AppName     string
	BundleID    string
	Title       string
	Bounds      Bounds
	ScreenID    ScreenID
	SpaceID     SpaceID
	PID         int
	Minimized   bool
	Hidden      bool
	Fullscreen  bool
	OnScreen    bool
	LastFocused time.Time
}

// IsVisible reports whether the window is currently shown on a screen: it is on
// screen and neither minimized nor belonging to a hidden app.
func (w Window) IsVisible() bool {
	return w.OnScreen && !w.Minimized && !w.Hidden
}

// Screen describes a physical display.
type Screen struct {
	ID      ScreenID
	Bounds  Bounds
	Main    bool
	Visible Bounds // bounds minus menubar/dock (the usable frame)
}

// Space describes a macOS Space (virtual desktop).
type Space struct {
	ID     SpaceID
	Index  int
	Active bool
}
