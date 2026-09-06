//go:build darwin

package platform

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNativeIconHealthDistinguishesDenialAndPreservesDownReplay(t *testing.T) {
	for _, tt := range []struct {
		name       string
		mask, want int
		timeout    bool
	}{{"AX revoked", 14, 1, false}, {"listening revoked with AX", 13, 2, false}, {"port invalid", 11, 3, false}, {"reenable refused", 7, 4, true}} {
		t.Run(tt.name, func(t *testing.T) {
			status, replays := nativeIconHealthProbe(tt.mask, tt.timeout)
			if status != tt.want || replays != 1 {
				t.Fatalf("health=%d replays=%d", status, replays)
			}
		})
	}
}

func TestNativeIconHealthFailureReturnsAfterJoinedCleanup(t *testing.T) {
	for code, want := range map[int]string{1: "accessibility", 2: "event listening", 3: "port", 4: "recovery"} {
		t.Run(want, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			calls := 0
			err := runNativeDockInputHealth(ctx, DockInputPolicy{ClickToHide: true}, make(chan DockInputTarget), func(DockInputEvent) { t.Error("empty cache emitted") }, true, func() int {
				calls++
				if calls == 1 {
					return 0
				}
				return code
			})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("runtime failure was not reported: %v", err)
			}
			if nativeDockInputTestAlive() != 0 {
				t.Fatal("error returned before native cleanup")
			}
			if ctx.Err() != nil {
				t.Fatal("waited for caller cancellation instead of terminal health")
			}
		})
	}
}
