package main

import (
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// nativeWindow is the slice of *application.WebviewWindow the app drives.
// Depending on the interface (rather than the concrete window) lets tests
// exercise the window lifecycle without a running Wails app.
type nativeWindow interface {
	Show() application.Window
	Hide() application.Window
	Focus()
	SetAlwaysOnTop(bool) application.Window
	NativeWindow() unsafe.Pointer
}

// liveWindow guards a Wails window against use after macOS destroyed it.
//
// Wails' WebviewWindow.Hide/Show only check that a window still has an impl,
// never its destroyed flag, and the impl is never cleared — so a call on a
// destroyed window reaches [NSWindow orderOut:] with a freed pointer and
// aborts the whole process (SIGABRT, which no recover() can catch; see the
// crash in issue #14). The window announces its own death via
// mac:WindowWillClose, which flips this wrapper closed so nothing is ever
// sent to it again; callers ask for a fresh window instead.
type liveWindow struct {
	mu     sync.Mutex
	win    nativeWindow
	closed bool
}

func newLiveWindow(w nativeWindow) *liveWindow {
	return &liveWindow{win: w}
}

// alive reports whether the window can still be messaged.
func (l *liveWindow) alive() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.win != nil && !l.closed
}

// markClosed records that the native window is gone.
func (l *liveWindow) markClosed() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
}

// wraps reports whether this wrapper holds w, so a close notification from a
// window we already replaced cannot retire its successor.
func (l *liveWindow) wraps(w nativeWindow) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.win == w
}

// get returns the window while it is alive, nil once it is not.
func (l *liveWindow) get() nativeWindow {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	return l.win
}

func (l *liveWindow) show() {
	if w := l.get(); w != nil {
		w.Show()
	}
}

func (l *liveWindow) hide() {
	if w := l.get(); w != nil {
		w.Hide()
	}
}

func (l *liveWindow) focus() {
	if w := l.get(); w != nil {
		w.Focus()
	}
}

func (l *liveWindow) setAlwaysOnTop(b bool) {
	if w := l.get(); w != nil {
		w.SetAlwaysOnTop(b)
	}
}

// native returns the NSWindow pointer, or nil when the window is gone — the
// caller must not hand a stale pointer to the platform layer.
func (l *liveWindow) native() unsafe.Pointer {
	if w := l.get(); w != nil {
		return w.NativeWindow()
	}
	return nil
}

// markOverlayClosed retires the switcher overlay after macOS destroyed it.
func (a *App) markOverlayClosed() {
	dlog("overlay window closed by the system; retiring it")
	a.overlay.markClosed()
}

// markPrefsClosedIf retires the preferences window when the window that
// reported closing is the one currently held (a late notification from an
// earlier window must not retire its replacement).
func (a *App) markPrefsClosedIf(w nativeWindow) {
	if a.prefs.wraps(w) {
		dlog("preferences window closed by the system; retiring it")
		a.prefs.markClosed()
	}
}
