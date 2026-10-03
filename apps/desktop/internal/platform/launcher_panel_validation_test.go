package platform

import (
	"context"
	"errors"
	"testing"
)

func TestLauncherPanelValidationAdmissionAndLateRetirement(t *testing.T) {
	for _, mode := range []string{"cancelled", "retired", "nativeRefusal", "ready"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			closed := false
			calls := 0
			err := validateLauncherPanel(ctx, "display", func() bool { return closed }, func(string) bool {
				calls++
				if mode == "cancelled" {
					cancel()
				}
				if mode == "retired" {
					closed = true
				}
				return mode != "nativeRefusal"
			})
			if calls != 1 {
				t.Fatal("native validator missing")
			}
			if mode == "ready" && err != nil {
				t.Fatal(err)
			}
			if mode != "ready" && err == nil {
				t.Fatal("stale native result admitted")
			}
			if mode == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
		})
	}
}

func TestLauncherPanelValidationNoReadForInvalidAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, v := range []struct {
		ctx     context.Context
		display string
		closed  bool
	}{{nil, "display", false}, {ctx, "display", false}, {context.Background(), "", false}, {context.Background(), "display", true}} {
		calls := 0
		err := validateLauncherPanel(v.ctx, v.display, func() bool { return v.closed }, func(string) bool { calls++; return true })
		if err == nil || calls != 0 {
			t.Fatal("invalid admission reached native read")
		}
	}
}
