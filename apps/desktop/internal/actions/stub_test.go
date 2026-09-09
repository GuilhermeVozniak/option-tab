//go:build !darwin

package actions

import (
	"strings"
	"testing"

	"option-tab/internal/platform"
)

func TestProductionStubRejectsSingleActions(t *testing.T) {
	backend, err := platform.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"focus", "close", "minimize", "fullscreen", "hide", "quit", "forceQuit", "newWindow"} {
		t.Run(kind, func(t *testing.T) {
			result, err := New(backend).Perform(kind, 1, 1)
			if err == nil || !strings.Contains(err.Error(), "unsupported") || result.Succeeded != 0 || len(result.Failures) != 1 {
				t.Fatalf("stub pretended to perform %s: %+v err=%v", kind, result, err)
			}
		})
	}
}

func TestProductionStubBulkReportsUnsupportedPerWindow(t *testing.T) {
	backend, err := platform.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"closeAll", "minimizeAll"} {
		t.Run(kind, func(t *testing.T) {
			result, err := New(backend).Perform(kind, 0, 1)
			if err != nil || result.Succeeded != 0 || len(result.Failures) != 1 || result.Failures[0].WindowID != 1 || !strings.Contains(result.Failures[0].Error, "unsupported") {
				t.Fatalf("stub pretended to perform %s: %+v err=%v", kind, result, err)
			}
		})
	}
}
