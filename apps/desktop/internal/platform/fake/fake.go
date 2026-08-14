// Package fake is an in-memory implementation of platform.Platform for tests.
// It records calls, lets tests script window lists and hotkey events, and
// mutates its own state so a full activate->cycle->commit sequence can be
// exercised without any OS interaction. It lives in its own package so it is
// never linked into the production binary.
package fake

import (
	"errors"
	"image"
	"sync"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
)

var errTest = errors.New("fake: error")

// Fake is a scriptable platform backend.
type Fake struct {
	mu      sync.Mutex
	windows []domain.Window

	// Injected errors.
	WindowsErr error
	FocusErr   error

	// Recorded interactions.
	FocusCalls      []domain.WindowID
	CloseCalls      []domain.WindowID
	MinimizeCalls   []domain.WindowID
	FullscreenCalls []domain.WindowID
	WarpCalls       []domain.WindowID
	QuitCalls       []domain.AppID
	HideCalls       []domain.AppID
	LastFocused     domain.WindowID

	// Environment.
	ActiveAppID    domain.AppID
	ActiveSpaceID  domain.SpaceID
	ScreenList     []domain.Screen
	ActiveScreenID domain.ScreenID
	CursorScreenID domain.ScreenID

	// Permissions / login item.
	AccessibilityState   platform.PermState
	ScreenRecordingState platform.PermState
	RequestCalls         []platform.PermKind
	loginEnabled         bool

	engine *hotkeyEngine
}

// New returns a Fake with sensible empty defaults and one screen/space active.
func New() *Fake {
	return &Fake{
		ActiveSpaceID:        1,
		ActiveScreenID:       1,
		CursorScreenID:       1,
		ScreenList:           []domain.Screen{{ID: 1, Main: true}},
		AccessibilityState:   platform.PermGranted,
		ScreenRecordingState: platform.PermGranted,
		engine:               newHotkeyEngine(),
	}
}

// SetWindows replaces the window list.
func (f *Fake) SetWindows(ws []domain.Window) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.windows = append([]domain.Window(nil), ws...)
}

// Name identifies the backend.
func (f *Fake) Name() string { return "fake" }

// Windows returns a copy of the configured windows.
func (f *Fake) Windows() ([]domain.Window, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.WindowsErr != nil {
		return nil, f.WindowsErr
	}
	return append([]domain.Window(nil), f.windows...), nil
}

// Focus records the call and remembers the most recently focused window.
func (f *Fake) Focus(id domain.WindowID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.FocusErr != nil {
		return f.FocusErr
	}
	f.FocusCalls = append(f.FocusCalls, id)
	f.LastFocused = id
	return nil
}

// Close records the call and removes the window from the list.
func (f *Fake) Close(id domain.WindowID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.CloseCalls = append(f.CloseCalls, id)
	for i, w := range f.windows {
		if w.ID == id {
			f.windows = append(f.windows[:i], f.windows[i+1:]...)
			break
		}
	}
	return nil
}

// Minimize records the call and sets the window's Minimized flag.
func (f *Fake) Minimize(id domain.WindowID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.MinimizeCalls = append(f.MinimizeCalls, id)
	for i := range f.windows {
		if f.windows[i].ID == id {
			f.windows[i].Minimized = true
			f.windows[i].OnScreen = false
		}
	}
	return nil
}

// Fullscreen records the call and toggles the window's Fullscreen flag.
func (f *Fake) Fullscreen(id domain.WindowID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.FullscreenCalls = append(f.FullscreenCalls, id)
	for i := range f.windows {
		if f.windows[i].ID == id {
			f.windows[i].Fullscreen = !f.windows[i].Fullscreen
		}
	}
	return nil
}

// WarpCursorToWindow records the call (implements platform.CursorWarper).
func (f *Fake) WarpCursorToWindow(id domain.WindowID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.WarpCalls = append(f.WarpCalls, id)
	return nil
}

// QuitApp records the call and removes the app's windows.
func (f *Fake) QuitApp(id domain.AppID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.QuitCalls = append(f.QuitCalls, id)
	kept := f.windows[:0:0]
	for _, w := range f.windows {
		if w.AppID != id {
			kept = append(kept, w)
		}
	}
	f.windows = kept
	return nil
}

// HideApp records the call and marks the app's windows hidden.
func (f *Fake) HideApp(id domain.AppID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.HideCalls = append(f.HideCalls, id)
	for i := range f.windows {
		if f.windows[i].AppID == id {
			f.windows[i].Hidden = true
			f.windows[i].OnScreen = false
		}
	}
	return nil
}

// Thumbnail returns a small solid image sized to maxPx.
func (f *Fake) Thumbnail(_ domain.WindowID, maxPx int) (image.Image, error) {
	if maxPx <= 0 {
		maxPx = 1
	}
	return image.NewRGBA(image.Rect(0, 0, maxPx, maxPx)), nil
}

// ActiveApp reports the configured active app.
func (f *Fake) ActiveApp() domain.AppID { return f.ActiveAppID }

// ActiveSpace reports the configured active space.
func (f *Fake) ActiveSpace() domain.SpaceID { return f.ActiveSpaceID }

// Screens reports the configured screens.
func (f *Fake) Screens() []domain.Screen { return f.ScreenList }

// ActiveScreen reports the configured active screen.
func (f *Fake) ActiveScreen() domain.ScreenID { return f.ActiveScreenID }

// CursorScreen reports the configured cursor screen.
func (f *Fake) CursorScreen() domain.ScreenID { return f.CursorScreenID }

// Accessibility reports the configured accessibility permission state.
func (f *Fake) Accessibility() platform.PermState { return f.AccessibilityState }

// ScreenRecording reports the configured screen-recording permission state.
func (f *Fake) ScreenRecording() platform.PermState { return f.ScreenRecordingState }

// Request records a permission request.
func (f *Fake) Request(k platform.PermKind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RequestCalls = append(f.RequestCalls, k)
}

// Enabled reports the login-item state.
func (f *Fake) Enabled() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loginEnabled
}

// SetEnabled sets the login-item state.
func (f *Fake) SetEnabled(v bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loginEnabled = v
	return nil
}

// Hotkeys returns the fake hotkey engine.
func (f *Fake) Hotkeys() platform.HotkeyEngine { return f.engine }

// EmitHotkey pushes an event onto the engine's channel for tests.
func (f *Fake) EmitHotkey(ev platform.HotkeyEvent) { f.engine.emit(ev) }

// hotkeyEngine is a buffered-channel fake of platform.HotkeyEngine.
type hotkeyEngine struct {
	ch         chan platform.HotkeyEvent
	registered map[int]hotkey.Chord
}

func newHotkeyEngine() *hotkeyEngine {
	return &hotkeyEngine{
		ch:         make(chan platform.HotkeyEvent, 64),
		registered: map[int]hotkey.Chord{},
	}
}

func (e *hotkeyEngine) Register(id int, c hotkey.Chord) error {
	e.registered[id] = c
	return nil
}

func (e *hotkeyEngine) Unregister(id int) error {
	delete(e.registered, id)
	return nil
}

func (e *hotkeyEngine) Events() <-chan platform.HotkeyEvent { return e.ch }

func (e *hotkeyEngine) Keys() <-chan platform.KeyEvent { return nil }

func (e *hotkeyEngine) SetOpen(bool) {}

func (e *hotkeyEngine) Close() error { return nil }

func (e *hotkeyEngine) emit(ev platform.HotkeyEvent) { e.ch <- ev }
