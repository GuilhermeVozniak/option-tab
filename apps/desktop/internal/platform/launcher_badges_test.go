package platform

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func badgeTarget() LauncherBadgeTarget {
	return LauncherBadgeTarget{ItemKey: "pin:a", TargetRevision: 1, BundleID: "test.fixture", CanonicalAppPath: "/tmp/Fixture.app"}
}

func TestLauncherBadgesJoinedCancellationAndCopies(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s := newLauncherBadgeSource(func(ctx context.Context, targets []LauncherBadgeTarget) (badgeObservation, error) {
		calls.Add(1)
		close(entered)
		<-release
		v := uint32(0)
		return badgeObservation{Dock: ProcessIdentity{PID: 10, StartSeconds: 1}, Status: BadgeReady, Entries: []LauncherBadgeEntry{{ItemKey: targets[0].ItemKey, TargetRevision: 1, State: BadgeKnown, Kind: BadgeCount, Count: &v}}}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var emits atomic.Int32
	go func() {
		done <- s.ObserveLauncherBadges(ctx, []LauncherBadgeTarget{badgeTarget()}, func(LauncherBadgeSnapshot) { emits.Add(1) })
	}()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("returned before native read joined")
	default:
	}
	close(release)
	<-done
	if emits.Load() != 0 || calls.Load() != 1 {
		t.Fatal("late observation")
	}
}

func TestLauncherBadgeClassification(t *testing.T) {
	for _, tt := range []struct {
		text  string
		kind  LauncherBadgeKind
		count uint32
	}{{"", BadgeAbsent, 0}, {"0", BadgeCount, 0}, {"42", BadgeCount, 42}, {"9999999", BadgeIndicator, 0}, {"1,234", BadgeIndicator, 0}, {"•", BadgeIndicator, 0}} {
		k, n := classifyLauncherBadge(tt.text)
		if k != tt.kind || (n != nil && *n != tt.count) {
			t.Fatal(tt.text, k, n)
		}
	}
}

func TestLauncherBadgesCoalesceAndRate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls, emits atomic.Int32
	var first time.Time
	source := newLauncherBadgeSource(func(context.Context, []LauncherBadgeTarget) (badgeObservation, error) {
		n := calls.Add(1)
		switch n {
		case 1:
			first = time.Now()
		case 3:
			if time.Since(first) < time.Second {
				t.Error("poll faster than1Hz")
			}
			defer cancel()
		}
		count := uint32(4)
		return badgeObservation{Status: BadgeReady, Dock: ProcessIdentity{PID: 10, StartSeconds: 1}, Entries: []LauncherBadgeEntry{{ItemKey: "pin:a", TargetRevision: 1, State: BadgeKnown, Kind: BadgeCount, Count: &count}}}, nil
	})
	_ = source.ObserveLauncherBadges(ctx, []LauncherBadgeTarget{badgeTarget()}, func(s LauncherBadgeSnapshot) {
		emits.Add(1)
		*s.Entries[0].Count = 99
		s.Entries[0].State = BadgeUnavailable
	})
	if calls.Load() != 3 || emits.Load() != 1 {
		t.Fatal(calls.Load(), emits.Load())
	}
}
