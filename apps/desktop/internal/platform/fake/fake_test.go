package fake

import (
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func TestFake_ImplementsPlatform(t *testing.T) {
	var _ platform.Platform = New()
}

func TestWindows_ReturnsConfigured(t *testing.T) {
	f := New()
	f.SetWindows([]domain.Window{{ID: 1}, {ID: 2}})
	ws, err := f.Windows()
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Errorf("got %d windows, want 2", len(ws))
	}
}

func TestWindows_Error(t *testing.T) {
	f := New()
	f.WindowsErr = errTest
	if _, err := f.Windows(); err == nil {
		t.Error("expected error")
	}
}

func TestFocus_RecordsAndTracks(t *testing.T) {
	f := New()
	f.SetWindows([]domain.Window{{ID: 7}})
	if err := f.Focus(7); err != nil {
		t.Fatal(err)
	}
	if f.LastFocused != 7 {
		t.Errorf("LastFocused = %d, want 7", f.LastFocused)
	}
	if len(f.FocusCalls) != 1 || f.FocusCalls[0] != 7 {
		t.Errorf("FocusCalls = %v, want [7]", f.FocusCalls)
	}
}

func TestClose_RemovesWindow(t *testing.T) {
	f := New()
	f.SetWindows([]domain.Window{{ID: 1}, {ID: 2}})
	if err := f.Close(1); err != nil {
		t.Fatal(err)
	}
	ws, _ := f.Windows()
	if len(ws) != 1 || ws[0].ID != 2 {
		t.Errorf("after Close(1), windows = %v", ws)
	}
}

func TestMinimize_SetsFlag(t *testing.T) {
	f := New()
	f.SetWindows([]domain.Window{{ID: 1}})
	if err := f.Minimize(1); err != nil {
		t.Fatal(err)
	}
	ws, _ := f.Windows()
	if !ws[0].Minimized {
		t.Error("Minimize should set the Minimized flag")
	}
}

func TestHotkeys_EmitDelivers(t *testing.T) {
	f := New()
	eng := f.Hotkeys()
	f.EmitHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	select {
	case ev := <-eng.Events():
		if ev.Kind != platform.HotkeyActivate || ev.ShortcutID != 1 {
			t.Errorf("unexpected event %+v", ev)
		}
	default:
		t.Fatal("expected an event to be queued")
	}
}

func TestPermissionsAndLoginItem(t *testing.T) {
	f := New()
	f.AccessibilityState = platform.PermGranted
	f.ScreenRecordingState = platform.PermDenied
	if f.Accessibility() != platform.PermGranted || f.ScreenRecording() != platform.PermDenied {
		t.Error("permission states not reported")
	}
	if err := f.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !f.Enabled() {
		t.Error("login item should be enabled")
	}
}

func TestThumbnail_ReturnsImage(t *testing.T) {
	f := New()
	img, err := f.Thumbnail(1, 64)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() == 0 {
		t.Error("expected a non-empty image")
	}
}
