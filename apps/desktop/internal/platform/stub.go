//go:build !darwin

// Package platform: stub backend for non-macOS builds. It provides synthetic
// windows so the app builds and the UI is demoable everywhere, while the full
// native backend (window enumeration, focus, thumbnails, global hotkeys) ships
// for macOS in this version. Linux/Windows native backends are future work.
package platform

import (
	"image"
	"image/color"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

// stub is the non-macOS Platform implementation.
type stub struct {
	engine *stubHotkeys
}

// New returns the platform backend for the current OS. On non-macOS systems it
// returns a stub with synthetic data.
func New() (Platform, error) {
	return &stub{engine: &stubHotkeys{ch: make(chan HotkeyEvent)}}, nil
}

func (s *stub) Name() string { return "stub" }

func (s *stub) Windows() ([]domain.Window, error) {
	return []domain.Window{
		{ID: 1, AppID: 1, AppName: "Demo Editor", Title: "main.go — option-tab", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 2, AppID: 2, AppName: "Demo Browser", Title: "alt-tab.app", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 3, AppID: 3, AppName: "Demo Terminal", Title: "zsh", OnScreen: true, SpaceID: 1, ScreenID: 1},
	}, nil
}

func (s *stub) Focus(domain.WindowID) error      { return nil }
func (s *stub) Close(domain.WindowID) error      { return nil }
func (s *stub) Minimize(domain.WindowID) error   { return nil }
func (s *stub) Fullscreen(domain.WindowID) error { return nil }
func (s *stub) QuitApp(domain.AppID) error       { return nil }
func (s *stub) HideApp(domain.AppID) error       { return nil }

func (s *stub) Thumbnail(_ domain.WindowID, maxPx int) (image.Image, error) {
	if maxPx <= 0 {
		maxPx = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, maxPx, maxPx))
	for y := 0; y < maxPx; y++ {
		for x := 0; x < maxPx; x++ {
			img.Set(x, y, color.RGBA{R: 40, G: 44, B: 52, A: 255})
		}
	}
	return img, nil
}

func (s *stub) ActiveApp() domain.AppID       { return 1 }
func (s *stub) ActiveSpace() domain.SpaceID   { return 1 }
func (s *stub) Screens() []domain.Screen      { return []domain.Screen{{ID: 1, Main: true}} }
func (s *stub) ActiveScreen() domain.ScreenID { return 1 }
func (s *stub) CursorScreen() domain.ScreenID { return 1 }

func (s *stub) Accessibility() PermState   { return PermGranted }
func (s *stub) ScreenRecording() PermState { return PermGranted }
func (s *stub) Request(PermKind)           {}

func (s *stub) Enabled() bool         { return false }
func (s *stub) SetEnabled(bool) error { return nil }

func (s *stub) Hotkeys() HotkeyEngine { return s.engine }

// stubHotkeys is a no-op hotkey engine: registration succeeds but no native
// events are produced.
type stubHotkeys struct {
	ch chan HotkeyEvent
}

func (h *stubHotkeys) Register(int, hotkey.Chord) error { return nil }
func (h *stubHotkeys) Unregister(int) error             { return nil }
func (h *stubHotkeys) Events() <-chan HotkeyEvent       { return h.ch }
func (h *stubHotkeys) Keys() <-chan KeyEvent            { return nil }
func (h *stubHotkeys) SetOpen(bool)                     {}
func (h *stubHotkeys) Close() error                     { close(h.ch); return nil }
