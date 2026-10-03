//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/domain"
)

func TestNativeDockInputReceiverSmoke(t *testing.T) {
	fixture := os.Getenv("OPTION_TAB_INPUT_RECEIVER")
	if fixture == "" {
		t.Skip("set OPTION_TAB_INPUT_RECEIVER after coordinating exclusive desktop input")
	}
	prefix := filepath.Join(t.TempDir(), "receiver")
	child := exec.Command(fixture, prefix)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()
	defer func() {
		_ = os.WriteFile(prefix+".command", []byte("stop"), 0o600)
		select {
		case <-childDone:
		case <-time.After(2 * time.Second):
			_ = child.Process.Kill()
			<-childDone
		}
	}()
	read := func() (int, int, int) {
		data, _ := os.ReadFile(prefix + ".state")
		var pid, down, up int
		_, _ = fmt.Sscanf(string(data), "%d %d %d", &pid, &down, &up)
		return pid, down, up
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		pid, _, _ := read()
		if pid == child.Process.Pid {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("receiver did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	platform := &darwinPlatform{}
	ctx, cancel := context.WithCancel(context.Background())
	targets := make(chan DockInputTarget, 1)
	done := make(chan error, 1)
	var emissions atomic.Int32
	installs, passes := nativeDockInputCounters()
	go func() {
		done <- platform.ObserveDockInput(ctx, DockInputPolicy{ClickToHide: true}, targets, func(DockInputEvent) { emissions.Add(1) })
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("native source did not join")
		}
	}()
	deadline = time.Now().Add(2 * time.Second)
	for {
		now, _ := nativeDockInputCounters()
		if now > installs {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("production tap did not install")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// An intentionally matching cache stresses synthetic pass-through. No worker
	// target validation or app action is requested by this test.
	targets <- DockInputTarget{Generation: 100, DockPID: 1, ObservedAt: time.Now(), Item: &DockItem{AppID: domain.AppID(child.Process.Pid), BundleID: "input.receiver.fixture", Path: "/InputReceiver.app", ScreenID: 1, Bounds: domain.Bounds{X: 120, Y: 160, W: 220, H: 120}}}
	if err := os.WriteFile(prefix+".command", []byte("click"), 0o600); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		_, down, up := read()
		_, current := nativeDockInputCounters()
		if down == 1 && up == 1 && current >= passes+2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("receiver down/up=%d/%d, synthetic tap passes=%d", down, up, current-passes)
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(550 * time.Millisecond) // fixture restores saved pointer at 500ms
	if emissions.Load() != 0 {
		t.Fatal("synthetic input produced a gesture")
	}
	t.Logf("real tap installed; fixture PID=%d received exactly one down/up; synthetic callbacks passed=%d; no owned gestures", child.Process.Pid, func() uint64 { _, n := nativeDockInputCounters(); return n - passes }())
}
