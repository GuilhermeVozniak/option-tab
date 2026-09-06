//go:build !darwin

package platform

import (
	"context"
	"strings"
	"testing"
)

func TestStubDockObservationIsUnsupported(t *testing.T) {
	called := false
	err := (&stub{}).ObserveDock(context.Background(), func(DockObservation) { called = true })
	if err == nil || !strings.Contains(err.Error(), "unsupported") || called {
		t.Fatalf("stub pretended to observe Dock: err=%v emitted=%v", err, called)
	}
}
