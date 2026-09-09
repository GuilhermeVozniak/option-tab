package dock

import (
	"context"
	"errors"
	"testing"

	"option-tab/internal/platform"
)

type receiptInputRun struct {
	ctx     context.Context
	release chan struct{}
}

type receiptInputSource struct{ runs chan receiptInputRun }

func (s *receiptInputSource) ObserveDockInput(ctx context.Context, _ platform.DockInputPolicy, _ <-chan platform.DockInputTarget, _ func(platform.DockInputEvent)) error {
	r := receiptInputRun{ctx: ctx, release: make(chan struct{})}
	s.runs <- r
	<-ctx.Done()
	<-r.release
	return nil
}

func releaseRetirement(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func requireRetirementPending(t *testing.T, receipt <-chan struct{}) {
	t.Helper()
	select {
	case <-receipt:
		t.Fatal("retirement completed before native owner joined")
	default:
	}
}

func TestInputRetirementReceiptWaitsForSourceAndAllowsLaterEnable(t *testing.T) {
	s := &receiptInputSource{runs: make(chan receiptInputRun, 2)}
	c := NewInputController(InputControllerDeps{Source: s, SelfAppID: 99})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	defer func() { cancel(); monitorReceive(t, done) }()
	p := platform.DockInputPolicy{ClickToHide: true}
	c.Configure(true, p)
	first := monitorReceive(t, s.runs)
	defer releaseRetirement(first.release)
	receipt := c.RetireSource()
	monitorReceive(t, first.ctx.Done())
	requireRetirementPending(t, receipt)
	c.Configure(true, p)
	select {
	case <-s.runs:
		t.Fatal("new input source overlapped retiring source")
	default:
	}
	releaseRetirement(first.release)
	monitorReceive(t, receipt)
	second := monitorReceive(t, s.runs)
	defer releaseRetirement(second.release)
	secondReceipt := c.RetireSource()
	monitorReceive(t, second.ctx.Done())
	requireRetirementPending(t, secondReceipt)
	releaseRetirement(second.release)
	monitorReceive(t, secondReceipt)
}

func TestMonitorRetirementReceiptWaitsForObservationAndPlacement(t *testing.T) {
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer releaseRetirement(release)
	s := &monitorTestSource{runs: make(chan monitorTestRun, 2), placement: func(ctx context.Context, req platform.DockPlacementRequest) (platform.DockPlacementResult, error) {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		<-release
		return platform.DockPlacementResult{RequestID: req.RequestID, Verified: true}, nil
	}}
	c := monitorController(t, s)
	p := platform.DockMonitorLockPolicy{Target: "main"}
	c.Configure(true, p)
	first := monitorReceive(t, s.runs)
	defer releaseRetirement(first.release)
	monitorReady(first, 1, 7)
	placed := make(chan error, 1)
	go func() { _, err := c.Place(context.Background()); placed <- err }()
	monitorReceive(t, entered)
	receipt := c.RetireSource()
	if c.Snapshot().Status != "disabled" || c.RetireSource() != receipt {
		t.Fatal("retirement did not immediately disable/reuse its receipt")
	}
	monitorReceive(t, first.ctx.Done())
	monitorReceive(t, cancelled)
	requireRetirementPending(t, receipt)
	releaseRetirement(first.release)
	requireRetirementPending(t, receipt)
	releaseRetirement(release)
	if err := monitorReceive(t, placed); !errors.Is(err, ErrMonitorLockRetired) {
		t.Fatalf("retired placement accepted: %v", err)
	}
	monitorReceive(t, receipt)
	c.Configure(true, p)
	second := monitorReceive(t, s.runs)
	defer releaseRetirement(second.release)
	if second.policy.Session <= first.policy.Session {
		t.Fatal("retirement prevented a fresh source session")
	}
}

func TestRetirementBeforeRunPreventsSourceStartup(t *testing.T) {
	input := &receiptInputSource{runs: make(chan receiptInputRun, 1)}
	ic := NewInputController(InputControllerDeps{Source: input})
	ic.Configure(true, platform.DockInputPolicy{ClickToHide: true})
	monitorReceive(t, ic.RetireSource())
	monitor := &monitorTestSource{runs: make(chan monitorTestRun, 1)}
	mc := NewMonitorLockController(MonitorLockControllerDeps{Source: monitor})
	mc.Configure(true, platform.DockMonitorLockPolicy{Target: "main"})
	monitorReceive(t, mc.RetireSource())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ic.Run(ctx)
	mc.Run(ctx)
	if len(input.runs) != 0 || monitor.attempts.Load() != 0 {
		t.Fatal("retirement before Run installed a source")
	}
}
