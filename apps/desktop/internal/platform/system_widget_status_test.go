package platform

import (
	"context"
	"testing"
	"time"
)

func TestSystemWidgetRatesRequireSameMonotonicCounter(t *testing.T) {
	at := time.Now()
	a := systemCounter{Identity: "one", Upload: 100, Download: 200, Valid: true, At: at}
	b := a
	b.At = at.Add(time.Second)
	b.Upload = 150
	b.Download = 300
	up, down := systemRates(a, b)
	if up == nil || down == nil || *up != 50 || *down != 100 {
		t.Fatal("valid rates missing")
	}
	for _, bad := range []systemCounter{{Identity: "two", Valid: true, At: b.At, Upload: 150, Download: 300}, {Identity: "one", Valid: true, At: at, Upload: 150, Download: 300}, {Identity: "one", Valid: true, At: b.At, Upload: 99, Download: 300}, {Identity: "one", Valid: false, At: b.At}} {
		u, d := systemRates(a, bad)
		if u != nil || d != nil {
			t.Fatal("invalid deltas fabricated rates")
		}
	}
}

func TestSystemWidgetNetworkUsageOffAndCancelledRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan time.Time)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	counters, emitted := 0, 0
	go func() {
		done <- observeSystemNetwork(ctx, false, func(NetworkSnapshot) { emitted++ }, systemNetworkOps{Status: func() systemNetworkState {
			close(entered)
			<-release
			return systemNetworkState{Snapshot: NetworkSnapshot{Status: "ready"}}
		}, Counters: func(systemNetworkState) systemCounter { counters++; return systemCounter{} }, Now: time.Now, Ticks: ticks})
	}()
	<-entered
	cancel()
	close(release)
	<-done
	if counters != 0 || emitted != 0 {
		t.Fatal("cancelled/usage-off source read counters or emitted")
	}
}

func TestSystemWidgetUsageOffNeverReadsCounters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	connected := true
	calls := 0
	err := observeSystemNetwork(ctx, false, func(s NetworkSnapshot) {
		if s.UploadRate != nil || s.DownloadRate != nil {
			t.Error("usage-off leaked rates")
		}
		*s.Connected = false
		cancel()
	}, systemNetworkOps{Status: func() systemNetworkState {
		return systemNetworkState{Snapshot: NetworkSnapshot{Status: "ready", Connected: &connected, Category: "wifi"}, Interface: "one", Index: 1}
	}, Counters: func(systemNetworkState) systemCounter { calls++; return systemCounter{} }, Now: time.Now, Ticks: make(chan time.Time)})
	if err != context.Canceled || calls != 0 || !connected {
		t.Fatalf("usage/copy boundary: err=%v calls=%d copied=%v", err, calls, connected)
	}
}

func TestSystemWidgetBatteryCancellationJoinsAndDrops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	emitted := false
	go func() {
		done <- observeSystemBattery(ctx, func(BatterySnapshot) { emitted = true }, func() BatterySnapshot { close(entered); <-release; return BatterySnapshot{Status: "ready"} }, make(chan time.Time))
	}()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("read not joined")
	default:
	}
	close(release)
	if err := <-done; err != context.Canceled || emitted {
		t.Fatal("cancelled battery emitted")
	}
}

func TestSystemWidgetUsageDoesNotCatchUpAfterSlowStatusRead(t *testing.T) {
	now := time.Unix(100, 0)
	calls := 0
	connected := true
	ticks := make(chan time.Time, 1)
	ticks <- now.Add(time.Second)
	close(ticks)
	err := observeSystemNetwork(context.Background(), true, func(NetworkSnapshot) { now = time.Unix(101, 0) }, systemNetworkOps{Status: func() systemNetworkState {
		now = now.Add(200 * time.Millisecond)
		return systemNetworkState{Snapshot: NetworkSnapshot{Status: "ready", Connected: &connected, Category: "wifi"}, Interface: "one", Index: 1}
	}, Counters: func(systemNetworkState) systemCounter {
		calls++
		return systemCounter{Identity: "one", Valid: true, Upload: uint64(calls) * 100}
	}, Now: func() time.Time { return now }, Ticks: ticks})
	if err != nil || calls != 1 {
		t.Fatalf("counter read again only800ms after previous sample: calls=%d err=%v", calls, err)
	}
}
