//go:build darwin

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"option-tab/internal/domain"
)

// TestNativeStreamAnimationSmoke owns its fixture and captures no user window.
func TestNativeStreamAnimationSmoke(t *testing.T) {
	if os.Getenv("OPTIONTAB_STREAM_SMOKE") != "1" {
		t.Skip("opt-in native animated capture smoke")
	}
	p := &darwinPlatform{hotkeys: newDarwinHotkeys()}
	if p.ScreenRecording() != PermGranted {
		t.Skip("Screen Recording not granted to test host")
	}
	t.Logf("Accessibility status: %v", p.Accessibility())
	dir := t.TempDir()
	src := filepath.Join(dir, "fixture.m")
	binary := filepath.Join(dir, "fixture")
	fixtureSource, err := os.ReadFile(filepath.Join("testdata", "stream_fixture.m"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, fixtureSource, 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", src, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile fixture: %v: %s", err, out)
	}
	fixture := exec.Command(binary)
	fixture.Stderr = os.Stderr
	stdout, err := fixture.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := fixture.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = fixture.Process.Kill(); _ = fixture.Wait() })
	ready := make(chan string, 16)
	go func() {
		s := bufio.NewScanner(stdout)
		for s.Scan() {
			ready <- s.Text()
		}
	}()
	var id domain.WindowID
	select {
	case line := <-ready:
		n, err := strconv.ParseUint(line, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		id = domain.WindowID(n)
	case <-time.After(5 * time.Second):
		t.Fatal("fixture did not show")
	}
	for cycle := 0; cycle < 5; cycle++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		distinct := map[string]bool{}
		var cancelled time.Time
		started := time.Now()
		err := p.StreamWindow(ctx, id, 256, func(url string) {
			if strings.HasPrefix(url, "data:image/png;base64,") {
				distinct[url] = true
			}
			if len(distinct) >= 3 && cancelled.IsZero() {
				cancelled = time.Now()
				cancel()
			}
		})
		cancel()
		if len(distinct) < 3 {
			t.Fatalf("cycle %d: only %d differing frames: %v", cycle, len(distinct), err)
		}
		if time.Since(cancelled) > time.Second {
			t.Fatalf("cycle %d: slow cancellation %s", cycle, time.Since(cancelled))
		}
		remaining := 0
		streamRegistry.Range(func(_, _ any) bool { remaining++; return true })
		if remaining != 0 {
			t.Fatalf("cycle %d: %d retained Go sessions", cycle, remaining)
		}
		t.Logf("cycle %d: %d differing PNG frames, elapsed %s, cancellation %s, registry empty", cycle+1, len(distinct), time.Since(started).Round(time.Millisecond), time.Since(cancelled).Round(time.Millisecond))
	}

	command := func(name string) {
		t.Helper()
		if _, err := fmt.Fprintln(stdin, name); err != nil {
			t.Fatal(err)
		}
		select {
		case ack := <-ready:
			if ack != "done "+name {
				t.Fatalf("unexpected fixture acknowledgement %q", ack)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("fixture did not acknowledge %s", name)
		}
	}
	for _, mode := range []string{"minimize", "hide"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		first := make(chan struct{}, 1)
		ended := make(chan error, 1)
		go func() {
			ended <- p.StreamWindow(ctx, id, 256, func(string) {
				select {
				case first <- struct{}{}:
				default:
				}
			})
		}()
		select {
		case <-first:
		case err := <-ended:
			cancel()
			t.Fatalf("before %s: %v", mode, err)
		case <-ctx.Done():
			cancel()
			t.Fatal("missing first frame")
		}
		command(mode)
		select {
		case err := <-ended:
			cancel()
			t.Fatalf("%s incorrectly terminated stream: %v", mode, err)
		case <-time.After(1300 * time.Millisecond):
		}
		command("restore")
		cancel()
		select {
		case <-ended:
		case <-time.After(time.Second):
			t.Fatal("survival stream cancellation timed out")
		}
		t.Logf("%s survived 1.3 seconds, including owner-monitor tick", mode)
	}
	for _, mode := range []string{"close", "destroy"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		closed := false
		var closedAt time.Time
		err = p.StreamWindow(ctx, id, 256, func(string) {
			if !closed {
				closed = true
				closedAt = time.Now()
				command(mode)
			}
		})
		if !closed {
			cancel()
			t.Fatalf("no frame before %s: %v", mode, err)
		}
		if ctx.Err() != nil {
			cancel()
			t.Fatalf("%s did not terminate stream before deadline: %v", mode, err)
		}
		cancel()
		if _, ok := err.(WindowUnavailableError); !ok {
			t.Fatalf("%s termination error = %v", mode, err)
		}
		t.Logf("%s terminated native stream in %s: %v", mode, time.Since(closedAt).Round(time.Millisecond), err)
		if mode == "close" {
			command("restore")
		}
	}
}
