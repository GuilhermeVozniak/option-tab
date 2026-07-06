//go:build darwin

package platform

import (
	"testing"

	"option-tab/internal/domain"
)

// mapRawWindows is the pure JSON->domain translation, tested without CGO.
func TestMapRawWindows_FieldsAndRecency(t *testing.T) {
	raws := []rawWindow{
		{
			ID: 10, PID: 100, App: "Editor", Bundle: "com.ex.editor", Title: "main.go",
			X: 1, Y: 2, W: 800, H: 600, OnScreen: true,
			Screen: 1, Space: 5, ZOrder: 0,
		},
		{
			ID: 11, PID: 101, App: "Browser", Bundle: "com.ex.browser", Title: "",
			OnScreen: false, Minimized: true, Screen: 2, Space: 6, ZOrder: 1,
		},
		{
			ID: 12, PID: 102, App: "Term", Bundle: "com.ex.term", Title: "zsh",
			OnScreen: false, Hidden: true, Fullscreen: true, ZOrder: 2,
		},
	}

	out := mapRawWindows(raws)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}

	// Field mapping.
	w0 := out[0]
	if w0.ID != 10 || w0.AppID != 100 || w0.AppName != "Editor" || w0.BundleID != "com.ex.editor" {
		t.Errorf("w0 identity wrong: %+v", w0)
	}
	if w0.ScreenID != domain.ScreenID(1) || w0.SpaceID != domain.SpaceID(5) {
		t.Errorf("w0 screen/space = %d/%d, want 1/5", w0.ScreenID, w0.SpaceID)
	}
	if w0.Bounds.W != 800 || w0.Bounds.H != 600 {
		t.Errorf("w0 bounds = %+v", w0.Bounds)
	}
	if !out[1].Minimized {
		t.Error("w1 should be minimized")
	}
	if !out[2].Hidden || !out[2].Fullscreen {
		t.Error("w2 should be hidden + fullscreen")
	}

	// Recency: lower ZOrder (frontmost) -> later LastFocused.
	if !out[0].LastFocused.After(out[1].LastFocused) {
		t.Error("frontmost window (zorder 0) should be more recent than zorder 1")
	}
	if !out[1].LastFocused.After(out[2].LastFocused) {
		t.Error("zorder 1 should be more recent than zorder 2")
	}
}

// TestMapRawWindows_Degradation covers the safe-fallback contract: when the
// native side yields nothing useful (no Accessibility/CGS access), an all-zero
// rawWindow must still map to a valid zero-value domain.Window without
// panicking, so Space/Screen filters degrade to "all" rather than hiding it.
func TestMapRawWindows_Degradation(t *testing.T) {
	out := mapRawWindows([]rawWindow{{}})
	if len(out) != 1 {
		t.Fatalf("len = %d, want 1", len(out))
	}
	w := out[0]
	if w.ID != 0 || w.AppID != 0 || w.AppName != "" || w.BundleID != "" || w.Title != "" {
		t.Errorf("zero rawWindow should map to zero-value identity, got %+v", w)
	}
	if w.ScreenID != domain.ScreenID(0) || w.SpaceID != domain.SpaceID(0) {
		t.Errorf("zero screen/space, got %d/%d", w.ScreenID, w.SpaceID)
	}
	if w.Bounds.W != 0 || w.Bounds.H != 0 {
		t.Errorf("zero bounds, got %+v", w.Bounds)
	}
	if w.Minimized || w.Hidden || w.Fullscreen || w.OnScreen {
		t.Errorf("zero flags expected, got %+v", w)
	}
}

// TestMapRawWindows_Empty covers the empty enumeration: no windows in, no
// windows out (and never a nil-deref).
func TestMapRawWindows_Empty(t *testing.T) {
	if out := mapRawWindows(nil); len(out) != 0 {
		t.Errorf("nil input -> %d windows, want 0", len(out))
	}
	if out := mapRawWindows([]rawWindow{}); len(out) != 0 {
		t.Errorf("empty input -> %d windows, want 0", len(out))
	}
}
