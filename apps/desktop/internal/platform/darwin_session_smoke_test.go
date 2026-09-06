//go:build darwin

package platform

import (
	"context"
	"os"
	"testing"
	"time"
)

// This read-only opt-in probe runs in its own test process. It neither posts
// system notifications nor changes lock/sleep/user-session state.
func TestSessionNativeBaselineAndCancellation(t *testing.T) {
	if os.Getenv("OPTION_TAB_SESSION_NATIVE_SMOKE") != "1" {
		t.Skip("set OPTION_TAB_SESSION_NATIVE_SMOKE=1 for read-only native lifecycle probe")
	}
	poller, err := newNativeSessionPoller()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := poller.Poll()
	poller.Close()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("native seed valid=%v onConsole=%v loginDone=%v privateLockKeyPresent=%v", raw.valid, raw.onConsole, raw.loginDone, raw.lockState >= 0)
	var previous uint64
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		calls := 0
		started := time.Now()
		err := (&darwinPlatform{}).ObserveSession(ctx, func(state SessionState) {
			calls++
			t.Logf("session=%d generation=%d inactive=%v reason=%q", i, state.Generation, state.Inactive, state.Reason)
			if state.Generation <= previous {
				t.Error("generation reused across native invocations")
			}
			previous = state.Generation
			cancel()
		})
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("native sample count=%d", calls)
		}
		if time.Since(started) > time.Second {
			t.Fatalf("native baseline/cancel exceeded 1s: %v", time.Since(started))
		}
	}
}
