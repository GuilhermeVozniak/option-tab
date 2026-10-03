//go:build darwin

package platform

import (
	"errors"
	"testing"
	"time"
)

func TestDockPanelNilHostDoesNotWaitForAppKitLoop(t *testing.T) {
	done := make(chan error, 1)
	go func() { _, err := (&darwinPlatform{}).CreateDockPanel(nil); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrDockPanelHostClosed) {
			t.Fatal(err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("nil host attempted synchronous AppKit dispatch")
	}
}
