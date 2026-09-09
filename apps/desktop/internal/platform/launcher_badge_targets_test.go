package platform

import (
	"context"
	"fmt"
	"testing"
)

func TestLauncherBadgeTargetUnionBudget(t *testing.T) {
	targets := make([]LauncherBadgeTarget, 256)
	for i := range targets {
		targets[i] = badgeTarget()
		targets[i].ItemKey = fmt.Sprint(i)
	}
	if !validBadgeTargets(targets) {
		t.Fatal("256 target union refused")
	}
	targets = append(targets, badgeTarget())
	if validBadgeTargets(targets) {
		t.Fatal("unbounded union")
	}
}

func TestRunningBadgeResolutionRejectsPIDReuse(t *testing.T) {
	process := ProcessIdentity{PID: 3, StartSeconds: 7}
	calls := 0
	_, err := resolveRunningBadgeTarget(context.Background(), process, "fixture.app", func(context.Context, ProcessIdentity, string) (LauncherBadgeTarget, error) {
		calls++
		v := LauncherBadgeTarget{Process: process, BundleID: "fixture.app", CanonicalAppPath: "/tmp/Fixture.app"}
		if calls == 2 {
			v.Process.StartSeconds++
		}
		return v, nil
	})
	if err == nil || calls != 2 {
		t.Fatal("reused PID accepted", calls, err)
	}
}

func TestReferenceBadgeResolutionJoinsAndRechecks(t *testing.T) {
	expected := LauncherReference{ID: "id", Kind: "app", State: "ready", Revision: 4, BundleID: "fixture.app"}
	for _, mode := range []string{"revision", "process", "fingerprint", "cancel", "valid"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			closed := false
			checks := 0
			v, err := resolveReferenceBadgeTarget(ctx, expected, func() (badgeReferenceResolution, error) {
				actual := expected
				if mode == "revision" {
					actual.Revision++
				}
				if mode == "process" {
					actual.Process = ProcessIdentity{PID: 3, StartSeconds: 7}
				}
				return badgeReferenceResolution{Reference: actual, Path: "/tmp/Fixture.app", Close: func() { closed = true }, Current: func() bool {
					checks++
					if mode == "cancel" {
						cancel()
					}
					return mode != "fingerprint" || checks == 1
				}}, nil
			})
			if !closed {
				t.Fatal("scope not released")
			}
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, v, err)
			}
		})
	}
}

func TestReferenceBadgeCancelledResolutionJoinsScopeCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	closed := make(chan struct{})
	done := make(chan error, 1)
	expected := LauncherReference{ID: "id", Kind: "app", State: "ready", Revision: 4, BundleID: "fixture.app"}
	go func() {
		_, err := resolveReferenceBadgeTarget(ctx, expected, func() (badgeReferenceResolution, error) {
			close(entered)
			<-release
			return badgeReferenceResolution{Reference: expected, Path: "/tmp/Fixture.app", Current: func() bool { return true }, Close: func() { close(closed) }}, nil
		})
		done <- err
	}()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("returned before read joined")
	default:
	}
	close(release)
	if err := <-done; err != context.Canceled {
		t.Fatal(err)
	}
	select {
	case <-closed:
	default:
		t.Fatal("result preceded scope release")
	}
}

func TestRunningBadgeResolutionCopiesExactIdentity(t *testing.T) {
	process := ProcessIdentity{PID: 3, StartSeconds: 7}
	calls := 0
	v, err := resolveRunningBadgeTarget(context.Background(), process, "fixture.app", func(context.Context, ProcessIdentity, string) (LauncherBadgeTarget, error) {
		calls++
		return LauncherBadgeTarget{Process: process, BundleID: "fixture.app", CanonicalAppPath: "/tmp/Fixture.app"}, nil
	})
	if err != nil || calls != 2 || v.Process != process || v.ItemKey != "" || v.TargetRevision != 0 {
		t.Fatal(v, calls, err)
	}
}
