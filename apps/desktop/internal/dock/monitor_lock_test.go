package dock

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/platform"
)

type monitorTestRun struct {
	ctx     context.Context
	policy  platform.DockMonitorLockPolicy
	emit    func(platform.DockMonitorLockState)
	release chan struct{}
}
type monitorTestSource struct {
	runs      chan monitorTestRun
	placement func(context.Context, platform.DockPlacementRequest) (platform.DockPlacementResult, error)
	attempts  atomic.Int32
	fail      bool
}

func (s *monitorTestSource) DockMonitorLockDisplays(context.Context) ([]platform.DockLockDisplay, error) {
	return nil, nil
}

func (s *monitorTestSource) ObserveDockMonitorLock(ctx context.Context, p platform.DockMonitorLockPolicy, emit func(platform.DockMonitorLockState)) error {
	s.attempts.Add(1)
	if s.fail {
		return errors.New("permission refused")
	}
	r := monitorTestRun{ctx, p, emit, make(chan struct{})}
	s.runs <- r
	<-ctx.Done()
	<-r.release
	return nil
}

func (s *monitorTestSource) PlaceDock(ctx context.Context, r platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
	return s.placement(ctx, r)
}

func monitorReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case x := <-ch:
		return x
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
		var zero T
		return zero
	}
}

func monitorReady(r monitorTestRun, sequence, generation uint64) {
	r.emit(platform.DockMonitorLockState{Session: r.policy.Session, Revision: r.policy.Revision, Sequence: sequence, Generation: generation, Status: "awaitingPlacement", Displays: []platform.DockLockDisplay{{UUID: "original"}}})
}

func monitorController(t *testing.T, s *monitorTestSource) *MonitorLockController {
	t.Helper()
	c := NewMonitorLockController(MonitorLockControllerDeps{Source: s})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); monitorReceive(t, done) })
	return c
}

func TestMonitorLockDisabledAndJoinsBeforeReplacement(t *testing.T) {
	s := &monitorTestSource{runs: make(chan monitorTestRun, 4)}
	c := monitorController(t, s)
	if c.Snapshot().Status != "disabled" || s.attempts.Load() != 0 {
		t.Fatal("default installed source")
	}
	p := platform.DockMonitorLockPolicy{Target: "main"}
	c.Configure(true, p)
	first := monitorReceive(t, s.runs)
	monitorReady(first, 1, 7)
	c.Configure(false, p)
	if c.Snapshot().Status != "disabled" {
		t.Fatal("disable not synchronous")
	}
	monitorReceive(t, first.ctx.Done())
	c.Configure(true, p)
	select {
	case <-s.runs:
		t.Fatal("replacement before join")
	default:
	}
	monitorReady(first, 2, 8)
	if c.Snapshot().Status != "starting" {
		t.Fatal("retired source overwrote state")
	}
	close(first.release)
	second := monitorReceive(t, s.runs)
	defer close(second.release)
	if second.policy.Session <= first.policy.Session || second.policy.Revision <= first.policy.Revision {
		t.Fatal("scope reused")
	}
	monitorReady(second, 3, 2)
	monitorReady(second, 2, 1)
	snapshot := c.Snapshot()
	if snapshot.Generation != 2 {
		t.Fatal("stale sequence admitted")
	}
	snapshot.Displays[0].UUID = "mutated"
	if c.Snapshot().Displays[0].UUID != "original" {
		t.Fatal("snapshot aliases")
	}
	p.Session = 999
	p.Revision = 999
	c.Configure(true, p)
	if c.Snapshot().Session != second.policy.Session {
		t.Fatal("caller transport scope restarted source")
	}
}

func TestMonitorLockPlacementCancellationRejectsLateSuccess(t *testing.T) {
	entered := make(chan platform.DockPlacementRequest, 1)
	release := make(chan struct{})
	cancelled := make(chan struct{})
	s := &monitorTestSource{runs: make(chan monitorTestRun, 4), placement: func(ctx context.Context, r platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
		entered <- r
		<-ctx.Done()
		close(cancelled)
		<-release
		return platform.DockPlacementResult{RequestID: r.RequestID, Verified: true}, nil
	}}
	c := monitorController(t, s)
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	r := monitorReceive(t, s.runs)
	defer close(r.release)
	monitorReady(r, 1, 42)
	done := make(chan error, 1)
	go func() { _, err := c.Place(context.Background()); done <- err }()
	req := monitorReceive(t, entered)
	if req.Generation != 42 || req.Session != r.policy.Session {
		t.Fatal("wrong placement scope")
	}
	if _, err := c.Place(context.Background()); err == nil {
		t.Fatal("concurrent placement admitted")
	}
	c.CancelPlacement()
	monitorReceive(t, cancelled)
	if _, err := c.Place(context.Background()); err == nil {
		t.Fatal("blocked operation replaced before join")
	}
	close(release)
	if monitorReceive(t, done) == nil {
		t.Fatal("cancelled success escaped")
	}
}

func TestMonitorLockRetriesAreBounded(t *testing.T) {
	s := &monitorTestSource{fail: true}
	c := monitorController(t, s)
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	deadline := time.Now().Add(time.Second)
	for s.attempts.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 100; i++ {
		c.Configure(true, platform.DockMonitorLockPolicy{Target: "main", Session: uint64(i)})
	}
	time.Sleep(100 * time.Millisecond)
	if s.attempts.Load() != 1 {
		t.Fatal("failure retries spun", s.attempts.Load())
	}
	if c.Snapshot().Status != "unavailable" {
		t.Fatal("source error hidden")
	}
	firstSession := c.Snapshot().Session
	deadline = time.Now().Add(2500 * time.Millisecond)
	for s.attempts.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if s.attempts.Load() != 2 || c.Snapshot().Session <= firstSession {
		t.Fatal("failed source did not recover with a fresh lifetime")
	}
}

func TestMonitorLockGenerationAndCallerCancellation(t *testing.T) {
	for _, reason := range []string{"generation", "configure", "caller", "shutdown"} {
		t.Run(reason, func(t *testing.T) {
			entered := make(chan platform.DockPlacementRequest, 1)
			release := make(chan struct{})
			cancelled := make(chan struct{})
			s := &monitorTestSource{runs: make(chan monitorTestRun, 2), placement: func(ctx context.Context, r platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
				entered <- r
				<-ctx.Done()
				close(cancelled)
				<-release
				return platform.DockPlacementResult{RequestID: r.RequestID, Verified: true}, nil
			}}
			c := NewMonitorLockController(MonitorLockControllerDeps{Source: s})
			lifetime, stop := context.WithCancel(context.Background())
			joined := make(chan struct{})
			go func() { c.Run(lifetime); close(joined) }()
			c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
			r := monitorReceive(t, s.runs)
			monitorReady(r, 1, 10)
			caller, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := c.Place(caller); done <- err }()
			monitorReceive(t, entered)
			switch reason {
			case "generation":
				monitorReady(r, 2, 11)
			case "configure":
				c.Configure(false, r.policy)
			case "caller":
				cancel()
			case "shutdown":
				stop()
			}
			monitorReceive(t, cancelled)
			if reason == "shutdown" {
				select {
				case <-joined:
					t.Fatal("shutdown did not join operation")
				default:
				}
			}
			close(release)
			if monitorReceive(t, done) == nil {
				t.Fatal("obsolete success escaped")
			}
			stop()
			close(r.release)
			monitorReceive(t, joined)
			if c.Snapshot().Status != "disabled" {
				t.Fatal("shutdown state remains active")
			}
			c.Configure(true, r.policy)
			if c.Snapshot().Status != "disabled" {
				t.Fatal("terminal controller restarted")
			}
		})
	}
}

func TestMonitorLockChangedReentrantAndNativeCopies(t *testing.T) {
	s := &monitorTestSource{runs: make(chan monitorTestRun, 2)}
	var c *MonitorLockController
	changed := make(chan platform.DockMonitorLockState, 16)
	c = NewMonitorLockController(MonitorLockControllerDeps{Source: s, Changed: func(state platform.DockMonitorLockState) {
		_ = c.Snapshot()
		c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
		if len(state.Displays) > 0 {
			state.Displays[0].Name = "consumer changed"
		}
		changed <- state
	}})
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	r := monitorReceive(t, s.runs)
	data := platform.DockMonitorLockState{Session: r.policy.Session, Revision: r.policy.Revision, Sequence: 1, Generation: 9, Status: "awaitingPlacement", Displays: []platform.DockLockDisplay{{Name: "source"}}}
	r.emit(data)
	data.Displays[0].Name = "producer changed"
	for {
		state := monitorReceive(t, changed)
		if state.Generation == 9 {
			break
		}
	}
	if c.Snapshot().Displays[0].Name != "source" {
		t.Fatal("native or consumer state aliases storage")
	}
	r.emit(platform.DockMonitorLockState{Session: r.policy.Session + 1, Revision: r.policy.Revision, Sequence: 99, Generation: 99, Status: "protected"})
	if c.Snapshot().Generation != 9 {
		t.Fatal("wrong native scope admitted")
	}
	cancel()
	close(r.release)
	monitorReceive(t, done)
}

func TestMonitorLockSuccessfulPlacementPreservesExactResult(t *testing.T) {
	s := &monitorTestSource{runs: make(chan monitorTestRun, 1), placement: func(_ context.Context, r platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
		return platform.DockPlacementResult{RequestID: r.RequestID, Status: "protected", ActualUUID: "target", Verified: true, CursorRestored: true}, nil
	}}
	c := monitorController(t, s)
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	r := monitorReceive(t, s.runs)
	defer close(r.release)
	monitorReady(r, 1, 8)
	result, err := c.Place(context.Background())
	if err != nil || !result.Verified || !result.CursorRestored || result.ActualUUID != "target" {
		t.Fatal(result, err)
	}
}

func TestMonitorLockCurrentNativeErrorSurfaces(t *testing.T) {
	refused := errors.New("native target cannot be reached")
	s := &monitorTestSource{runs: make(chan monitorTestRun, 1), placement: func(context.Context, platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
		return platform.DockPlacementResult{}, refused
	}}
	c := monitorController(t, s)
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	r := monitorReceive(t, s.runs)
	defer close(r.release)
	monitorReady(r, 1, 1)
	if _, err := c.Place(context.Background()); !errors.Is(err, refused) {
		t.Fatalf("native error lost: %v", err)
	}
}

func TestMonitorLockScopedPlacementRejectsRetiredIntent(t *testing.T) {
	var calls atomic.Int32
	s := &monitorTestSource{runs: make(chan monitorTestRun, 2), placement: func(_ context.Context, r platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
		calls.Add(1)
		return platform.DockPlacementResult{RequestID: r.RequestID, Verified: true}, nil
	}}
	c := monitorController(t, s)
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	first := monitorReceive(t, s.runs)
	monitorReady(first, 1, 1)
	old := c.Snapshot()
	monitorReady(first, 2, 2)
	if _, err := c.PlaceScoped(context.Background(), old.Session, old.Revision, old.Generation); !errors.Is(err, ErrMonitorLockRetired) {
		t.Fatal("old generation admitted", err)
	}
	current := c.Snapshot()
	c.Configure(true, platform.DockMonitorLockPolicy{Target: "display", DisplayUUID: "replacement"})
	close(first.release)
	second := monitorReceive(t, s.runs)
	defer close(second.release)
	monitorReady(second, 1, 2)
	if _, err := c.PlaceScoped(context.Background(), current.Session, current.Revision, current.Generation); !errors.Is(err, ErrMonitorLockRetired) {
		t.Fatal("old policy intent adopted replacement", err)
	}
	if calls.Load() != 0 {
		t.Fatal("obsolete intent reached native placement")
	}
	fresh := c.Snapshot()
	if _, err := c.PlaceScoped(context.Background(), fresh.Session, fresh.Revision, fresh.Generation); err != nil {
		t.Fatal("current scoped placement refused", err)
	}
	if calls.Load() != 1 {
		t.Fatal("current placement not dispatched")
	}
}
