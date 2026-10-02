//go:build darwin

package platform

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSwitcherWheelNativeCapabilitiesAndNilHost(t *testing.T) {
	host, ok := any(&darwinPlatform{}).(SwitcherWheelHost)
	if !ok {
		t.Fatal("native switcher wheel host is missing")
	}
	if _, ok := any(&darwinPlatform{}).(GuardedPointerPerformer); !ok {
		t.Fatal("guarded pointer action port is missing")
	}
	done := make(chan error, 1)
	go func() { _, err := host.CreateSwitcherWheel(nil); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrDockPanelHostClosed) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("nil host waited for AppKit")
	}
}

func TestDarwinSwitcherWheelNativeSeam(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "switcher-wheel")
	out, err := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "ApplicationServices", "testdata/switcher-wheel/main.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	if out, err = exec.CommandContext(ctx, binary).CombinedOutput(); err != nil {
		t.Fatalf("native seam: %v\n%s", err, out)
	}
	t.Log(string(out))
}
