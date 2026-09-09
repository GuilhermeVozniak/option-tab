//go:build darwin

package platform

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"option-tab/internal/domain"
)

// Deliberately excluded from normal test activity. No constructors, subprocesses,
// NSApplication creation, badge setters or remote AX reads occur before this gate.
func TestLauncherBadgesLiveOwnedFixture(t *testing.T) {
	if os.Getenv("OPTION_TAB_LIVE_BADGE_FIXTURE") != "1" {
		t.Skip("requires explicitly coordinated own-fixture live acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, "BadgeFixture.app")
	build := exec.CommandContext(ctx, "sh", "testdata/launcher-badges-live/build.sh", bundle)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture compilation failed: %v (%d output bytes)", err, len(output))
	}
	metadata, err := exec.CommandContext(ctx, "/usr/bin/plutil", "-extract", "CFBundleIdentifier", "raw", "-o", "-", filepath.Join(bundle, "Contents", "Info.plist")).Output()
	if err != nil {
		t.Fatal("fixture bundle metadata unavailable")
	}
	expectedBundleID := strings.TrimSpace(string(metadata))
	executable := filepath.Join(bundle, "Contents", "MacOS", "BadgeFixture")
	cmd := exec.CommandContext(ctx, executable)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal("fixture launch failed")
	}
	exited := make(chan error, 1)
	protocolDone := make(chan struct{})
	go func() { exited <- cmd.Wait() }()
	defer func() {
		_, _ = fmt.Fprintln(in, "quit")
		_ = in.Close()
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			// Only the process created by this test; never a PID discovered in inventory.
			_ = cmd.Process.Kill()
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
				t.Error("fixture cleanup did not join")
			}
		}
		cancel()
		select {
		case <-protocolDone:
		case <-time.After(time.Second):
			t.Error("fixture protocol did not join")
		}
	}()
	packets := make(chan map[string]json.RawMessage, 8)
	go func() {
		defer close(packets)
		defer close(protocolDone)
		scanner := bufio.NewScanner(out)
		scanner.Buffer(make([]byte, 512), 4096)
		for scanner.Scan() {
			var p map[string]json.RawMessage
			if json.Unmarshal(scanner.Bytes(), &p) != nil {
				return
			}
			select {
			case packets <- p:
			case <-ctx.Done():
				return
			}
		}
	}()
	receive := func() map[string]json.RawMessage {
		t.Helper()
		select {
		case p, ok := <-packets:
			if !ok {
				t.Fatal("fixture protocol closed")
			}
			return p
		case <-time.After(5 * time.Second):
			t.Fatal("fixture acknowledgement timed out")
		case <-ctx.Done():
			t.Fatal("fixture lifetime expired")
		}
		return nil
	}
	ready := receive()
	var announced struct {
		Ready               bool
		PID                 int
		Seconds, Micros     uint64
		BundleID, BundleURL string
	}
	encoded, _ := json.Marshal(ready)
	if json.Unmarshal(encoded, &announced) != nil || !announced.Ready || announced.PID != cmd.Process.Pid {
		t.Fatal("fixture readiness identity mismatch")
	}
	if announced.BundleID != expectedBundleID || !strings.HasPrefix(announced.BundleID, "org.optiontab.fixture.launcher-badges-live.") {
		t.Fatal("fixture bundle identity mismatch")
	}
	announcedURL, err := url.Parse(announced.BundleURL)
	if err != nil {
		t.Fatal("fixture bundle URL invalid")
	}
	if announcedURL.Scheme != "file" || announcedURL.Host != "" || announcedURL.RawQuery != "" || announcedURL.Fragment != "" || !filepath.IsAbs(announcedURL.Path) {
		t.Fatal("fixture bundle URL mismatch")
	}
	// Foundation can shorten /private/var to /var. Preserve its native path for
	// source comparisons, and independently resolve it to the exact owned bundle.
	nativeBundlePath := filepath.Clean(announcedURL.Path)
	resolvedBundle, err := filepath.EvalSymlinks(nativeBundlePath)
	if err != nil || resolvedBundle != bundle {
		t.Fatal("fixture bundle filesystem identity mismatch")
	}
	backend := &darwinPlatform{}
	identity, err := backend.ProcessIdentity(domain.AppID(cmd.Process.Pid))
	if err != nil || identity.StartSeconds != announced.Seconds || identity.StartMicros != announced.Micros {
		t.Fatal("fixture process incarnation mismatch")
	}
	verify := func() LauncherBadgeTarget {
		t.Helper()
		target, err := backend.ResolveRunningLauncherBadgeTarget(ctx, identity, announced.BundleID)
		if err != nil || target.Process != identity || target.BundleID != announced.BundleID || target.CanonicalAppPath != nativeBundlePath {
			t.Fatal("fixture current target verification refused")
		}
		resolved, err := filepath.EvalSymlinks(target.CanonicalAppPath)
		if err != nil || resolved != bundle {
			t.Fatal("fixture current target filesystem identity changed")
		}
		target.ItemKey = "owned-fixture"
		target.TargetRevision = 1
		return target
	}
	target := verify()
	observationCtx, stop := context.WithCancel(ctx)
	joined := make(chan error, 1)
	snapshots := make(chan LauncherBadgeSnapshot, 16)
	go func() {
		joined <- NewLauncherBadgeSource().ObserveLauncherBadges(observationCtx, []LauncherBadgeTarget{target}, func(s LauncherBadgeSnapshot) {
			select {
			case snapshots <- s:
			case <-observationCtx.Done():
			}
		})
	}()
	defer func() {
		stop()
		select {
		case <-joined:
		case <-time.After(4 * time.Second):
			t.Error("production badge source did not join")
		}
	}()
	var sequence uint64
	for _, step := range []struct {
		command string
		kind    LauncherBadgeKind
	}{{"count", BadgeCount}, {"indicator", BadgeIndicator}, {"empty", BadgeAbsent}} {
		if verify() != target {
			t.Fatal("fixture target changed")
		}
		commandStarted := time.Now()
		if _, err = fmt.Fprintln(in, step.command); err != nil {
			t.Fatal("fixture command failed")
		}
		packet := receive()
		var ack string
		if json.Unmarshal(packet["ack"], &ack) != nil || ack != step.command {
			t.Fatal("fixture acknowledgement mismatch")
		}
		deadline := time.NewTimer(7 * time.Second)
		status := BadgeSourceUnavailable
		var lastState LauncherBadgeState
		var lastKind LauncherBadgeKind
		var lastCount bool
		matched := false
		for !matched {
			select {
			case snapshot := <-snapshots:
				status = snapshot.Status
				if snapshot.Generation == 0 || snapshot.Sequence <= sequence || snapshot.ObservedAt.Before(commandStarted) || len(snapshot.Entries) != 1 {
					continue
				}
				sequence = snapshot.Sequence
				entry := snapshot.Entries[0]
				lastState, lastKind, lastCount = entry.State, entry.Kind, entry.Count != nil
				matched = status == BadgeReady && entry.ItemKey == target.ItemKey && entry.TargetRevision == target.TargetRevision
				switch step.kind {
				case BadgeCount:
					matched = matched && entry.State == BadgeKnown && entry.Kind == BadgeCount && entry.Count != nil && *entry.Count == 7
				case BadgeIndicator:
					matched = matched && entry.State == BadgeKnown && entry.Kind == BadgeIndicator && entry.Count == nil
				case BadgeAbsent:
					// Empty labels can become unreadable in Dock AX. That must clear
					// prior values without turning unavailable data into a known zero.
					cleared := (entry.State == BadgeKnown && entry.Kind == BadgeAbsent) || (entry.State == BadgeUnavailable && entry.Kind == "")
					matched = matched && cleared && entry.Count == nil
				}
			case <-deadline.C:
				t.Fatalf("owned fixture %s not observable: source=%s state=%s kind=%s countPresent=%t (no capability claim)", step.command, status, lastState, lastKind, lastCount)
			case <-ctx.Done():
				t.Fatal("live acceptance lifetime expired")
			}
		}
		deadline.Stop()
		if verify() != target {
			t.Fatal("fixture target changed after observation")
		}
		if step.kind == BadgeAbsent && lastState == BadgeUnavailable {
			t.Log("owned fixture empty: prior badge cleared to unavailable; known absence remains unverified")
		} else {
			t.Logf("owned fixture %s: production typed observation matched", step.command)
		}
	}
}
