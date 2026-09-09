//go:build darwin

package platform

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNativeStreamSmoke(t *testing.T) {
	if os.Getenv("OPTIONTAB_STREAM_SMOKE") != "1" {
		t.Skip("opt-in native window capture smoke")
	}
	p := &darwinPlatform{hotkeys: newDarwinHotkeys()}
	if p.ScreenRecording() != PermGranted {
		t.Skip("Screen Recording not granted to test host")
	}
	windows, err := p.Windows()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range windows {
		if !w.OnScreen || w.PID == os.Getpid() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		frames := 0
		err = p.StreamWindow(ctx, w.ID, 256, func(url string) {
			if strings.HasPrefix(url, "data:image/png;base64,") {
				frames++
				cancel()
			}
		})
		if frames == 0 {
			t.Fatalf("window %d: no native frame: %v", w.ID, err)
		}
		t.Logf("received PNG SCStream frame for window %d; cancellation returned: %v", w.ID, err)
		return
	}
	t.Skip("no on-screen window")
}
