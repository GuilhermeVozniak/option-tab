package widgets

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

type (
	runtimeFakeRun struct {
		ctx     context.Context
		emit    func(Sample)
		release chan struct{}
		caps    []string
	}
	runtimeFakeProvider struct {
		runs    chan runtimeFakeRun
		perform func(context.Context, ProviderAction, func() error) error
	}
)

func (p *runtimeFakeProvider) Observe(ctx context.Context, caps []string, emit func(Sample)) error {
	run := runtimeFakeRun{ctx, emit, make(chan struct{}), caps}
	p.runs <- run
	<-ctx.Done()
	<-run.release
	return ctx.Err()
}

func (p *runtimeFakeProvider) Perform(ctx context.Context, a ProviderAction, g func() error) error {
	return p.perform(ctx, a, g)
}

func runtimeReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("runtime timed out")
		var v T
		return v
	}
}

func runtimePackage(t *testing.T, provider string) *Package {
	t.Helper()
	m := testManifest()
	m.ID = "org.example." + provider
	m.RequiredCapabilities = []string{provider + ".read"}
	if provider == "battery" {
		m.Root = Node{Kind: "progress", Binding: &Binding{Provider: "battery", Field: "charge", Formatter: "percent"}}
	}
	p, err := Preview(context.Background(), bytes.NewReader(testZIP(t, m, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runtimeRequest(p *Package) Request {
	return Request{Package: p, ControllerEpoch: 1, DisplayUUID: "screen", Session: 1, ProfileID: "profile", InstanceID: "instance", Enabled: true, Grants: []string{"battery.read"}}
}

func TestRuntimeRequiredGrantAndSharedJoin(t *testing.T) {
	p := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 4)}
	r := NewRuntime(Deps{Providers: Providers{Battery: p}})
	req := runtimeRequest(runtimePackage(t, "battery"))
	req.Grants = nil
	if err := r.Configure([]Request{req}); err != nil {
		t.Fatal(err)
	}
	if r.Snapshot()[0].Status != "grantRequired" {
		t.Fatal("missing grant admitted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	req.Grants = []string{"battery.read"}
	second := req
	second.DisplayUUID = "second"
	if err := r.Configure([]Request{req, second}); err != nil {
		t.Fatal(err)
	}
	run := runtimeReceive(t, p.runs)
	if err := r.Configure([]Request{second}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-run.ctx.Done():
		t.Fatal("remaining consumer lost provider")
	default:
	}
	old := r.Snapshot()[0].Lease
	if err := r.Configure(nil); err != nil {
		t.Fatal(err)
	}
	runtimeReceive(t, run.ctx.Done())
	if err := r.Configure([]Request{second}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.runs:
		t.Fatal("replacement overlapped old source")
	default:
	}
	if _, err := r.Asset(old, "anything"); err != ErrRetired {
		t.Fatal("removed lease not retired")
	}
	close(run.release)
	next := runtimeReceive(t, p.runs)
	cancel()
	close(next.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestRuntimeSnapshotsAndSampleRetirement(t *testing.T) {
	var mu sync.Mutex
	states := make(chan []InstanceState, 32)
	p := &runtimeFakeProvider{runs: make(chan runtimeFakeRun, 4)}
	r := NewRuntime(Deps{Providers: Providers{Battery: p}, Changed: func(s []InstanceState) { mu.Lock(); states <- s; mu.Unlock() }})
	req := runtimeRequest(runtimePackage(t, "battery"))
	if err := r.Configure([]Request{req}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	run := runtimeReceive(t, p.runs)
	charge := .5
	run.emit(Sample{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Status: "ready", Fields: map[string]Value{"charge": {Number: &charge}}})
	for {
		s := runtimeReceive(t, states)
		if len(s) > 0 && s[0].Root.Progress != nil {
			if *s[0].Root.Progress != .5 {
				t.Fatal("wrong charge")
			}
			*s[0].Root.Progress = 0
			break
		}
	}
	if *r.Snapshot()[0].Root.Progress != .5 {
		t.Fatal("snapshot alias")
	}
	req.Enabled = false
	if err := r.Configure([]Request{req}); err != nil {
		t.Fatal(err)
	}
	charge = .9
	run.emit(Sample{Generation: 1, Sequence: 2, ObservedAt: time.Now(), Status: "ready", Fields: map[string]Value{"charge": {Number: &charge}}})
	if r.Snapshot()[0].Status != "disabled" || r.Snapshot()[0].Root.Progress != nil {
		t.Fatal("stale sample entered disabled instance")
	}
	cancel()
	close(run.release)
	if err := runtimeReceive(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}
