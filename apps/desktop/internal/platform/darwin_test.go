//go:build darwin

package platform

import "testing"

// These are smoke tests: the darwin backend translates OS calls and holds no
// decision logic, so we only assert it constructs and that read-only queries
// run without panicking or erroring on a CI macOS host (which may have no
// permissions granted — that must degrade, not crash).

func TestNew_Constructs(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if p.Name() != "darwin" {
		t.Errorf("Name() = %q, want darwin", p.Name())
	}
}

func TestWindows_DoesNotError(t *testing.T) {
	p, _ := New()
	if _, err := p.Windows(); err != nil {
		t.Errorf("Windows() error: %v", err)
	}
}

func TestPermissionQueries_DoNotPanic(t *testing.T) {
	p, _ := New()
	_ = p.Accessibility()
	_ = p.ScreenRecording()
	_ = p.ActiveApp()
	_ = p.Screens()
	_ = p.Enabled()
}

func TestKeycodeFor(t *testing.T) {
	if c, ok := keycodeFor("tab"); !ok || c != 48 {
		t.Errorf("keycodeFor(tab) = %d,%v want 48,true", c, ok)
	}
	if _, ok := keycodeFor("nope"); ok {
		t.Error("keycodeFor(unknown) should be false")
	}
}
