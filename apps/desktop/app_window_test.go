package main

import (
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

// fakeWindow records the window operations the app performs, standing in for
// a *application.WebviewWindow (which cannot be created without a running
// Wails app).
type fakeWindow struct {
	calls  []string
	onTop  bool
	native unsafe.Pointer
}

func (f *fakeWindow) Show() application.Window {
	f.calls = append(f.calls, "show")
	return nil
}

func (f *fakeWindow) Hide() application.Window {
	f.calls = append(f.calls, "hide")
	return nil
}

func (f *fakeWindow) Focus() { f.calls = append(f.calls, "focus") }

func (f *fakeWindow) SetAlwaysOnTop(b bool) application.Window {
	f.calls = append(f.calls, "alwaysOnTop")
	f.onTop = b
	return nil
}

func (f *fakeWindow) NativeWindow() unsafe.Pointer {
	f.calls = append(f.calls, "native")
	return f.native
}

func TestLiveWindowForwardsWhileAlive(t *testing.T) {
	w := &fakeWindow{native: unsafe.Pointer(new(int))}
	lw := newLiveWindow(w)

	lw.show()
	lw.hide()
	lw.focus()
	lw.setAlwaysOnTop(true)
	if got := lw.native(); got != w.native {
		t.Fatalf("native() = %v, want the window's pointer", got)
	}
	if !lw.alive() {
		t.Fatal("window should still be alive")
	}
	if !w.onTop {
		t.Error("setAlwaysOnTop(true) did not reach the window")
	}
	want := []string{"show", "hide", "focus", "alwaysOnTop", "native"}
	if len(w.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", w.calls, want)
	}
}

// A destroyed window must never be messaged again: Wails' Hide() does not
// check its destroyed flag, and [NSWindow orderOut:] on a released window
// aborts the process (SIGABRT), which no recover() can catch.
func TestLiveWindowSendsNothingAfterClose(t *testing.T) {
	w := &fakeWindow{native: unsafe.Pointer(new(int))}
	lw := newLiveWindow(w)
	lw.markClosed()

	lw.show()
	lw.hide()
	lw.focus()
	lw.setAlwaysOnTop(true)

	if len(w.calls) != 0 {
		t.Fatalf("closed window received %v, want no calls", w.calls)
	}
	if lw.alive() {
		t.Error("alive() should be false after markClosed")
	}
	if got := lw.native(); got != nil {
		t.Errorf("native() = %v, want nil for a closed window", got)
	}
}

func TestLiveWindowNilIsNotAlive(t *testing.T) {
	var lw *liveWindow
	lw.hide() // must not panic
	if lw.alive() {
		t.Error("nil liveWindow should not be alive")
	}
	if newLiveWindow(nil).alive() {
		t.Error("liveWindow wrapping nil should not be alive")
	}
}

// ---- App-level: the crash path from issue #14 ----

func newWindowTestApp(t *testing.T) (*App, *fakeWindow, *fakeWindow, *int) {
	t.Helper()
	p := fake.New()
	p.SetWindows(appTestWindows())
	a := newApp(p, config.Default(), filepath.Join(t.TempDir(), "settings.json"))
	overlay, prefs := &fakeWindow{}, &fakeWindow{}
	built := 0
	a.setRuntime(nil, overlay, prefs, func() nativeWindow {
		built++
		return &fakeWindow{}
	})
	return a, overlay, prefs, &built
}

// Closing preferences after the window died must not message the dead window.
func TestClosePreferencesSkipsClosedWindow(t *testing.T) {
	a, _, prefs, _ := newWindowTestApp(t)
	a.markPrefsClosedIf(prefs)

	a.closePreferencesWindow()

	if len(prefs.calls) != 0 {
		t.Fatalf("closed prefs window received %v, want no calls", prefs.calls)
	}
}

// Reopening preferences after the window died builds a fresh one instead of
// showing the dead one.
func TestOpenPreferencesRecreatesClosedWindow(t *testing.T) {
	a, _, prefs, built := newWindowTestApp(t)
	a.markPrefsClosedIf(prefs)

	a.OpenPreferences()

	if *built != 1 {
		t.Fatalf("factory built %d windows, want 1", *built)
	}
	if len(prefs.calls) != 0 {
		t.Fatalf("dead prefs window received %v, want no calls", prefs.calls)
	}
}

// A stale close notification for a window we already replaced must not kill
// the current one.
func TestMarkPrefsClosedIgnoresStaleWindow(t *testing.T) {
	a, _, prefs, _ := newWindowTestApp(t)
	a.markPrefsClosedIf(&fakeWindow{}) // some other, older window

	a.closePreferencesWindow()

	if len(prefs.calls) == 0 {
		t.Fatal("current prefs window should still be used")
	}
}

// The switcher's hide path must not message a destroyed overlay either.
func TestSwitcherHideSkipsClosedOverlay(t *testing.T) {
	a, overlay, _, _ := newWindowTestApp(t)
	a.markOverlayClosed()

	a.Hide()

	if len(overlay.calls) != 0 {
		t.Fatalf("closed overlay received %v, want no calls", overlay.calls)
	}
}
