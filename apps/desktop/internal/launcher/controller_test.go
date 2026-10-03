package launcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type fixture struct {
	mu        sync.Mutex
	emit      func(platform.LauncherEnvironment)
	starts    chan struct{}
	joins     chan struct{}
	views     chan State
	apps      int
	blockApps chan struct{}
}

func (f *fixture) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	f.mu.Lock()
	f.emit = emit
	f.mu.Unlock()
	f.starts <- struct{}{}
	<-ctx.Done()
	if f.joins != nil {
		<-f.joins
	}
	return ctx.Err()
}

func (f *fixture) Apps() ([]domain.App, error) {
	f.mu.Lock()
	f.apps++
	f.mu.Unlock()
	if f.blockApps != nil {
		<-f.blockApps
	}
	return []domain.App{{ID: 42, Name: "Fixture", BundleID: "fixture"}}, nil
}

func (*fixture) ProcessIdentity(id domain.AppID) (platform.ProcessIdentity, error) {
	return platform.ProcessIdentity{PID: id, StartSeconds: 1}, nil
}
func (f *fixture) Publish(s State) { f.views <- s }
func (f *fixture) send(e platform.LauncherEnvironment) {
	f.mu.Lock()
	emit := f.emit
	f.mu.Unlock()
	emit(e)
}

func env() platform.LauncherEnvironment {
	return platform.LauncherEnvironment{Generation: 1, Sequence: 1, ObservedAt: time.Now(), Complete: true, Status: "ready", PointerKnown: true, PointerX: 300, PointerY: 200, NativeDock: platform.LauncherNativeDock{Process: platform.ProcessIdentity{PID: 1, StartSeconds: 1}, Edge: "bottom", Visibility: "hidden", Confidence: "known", Bounds: domain.Bounds{X: 400, Y: 780, W: 200, H: 20}}, Displays: []platform.LauncherDisplay{{UUID: "11111111-1111-1111-1111-111111111111", Main: true, Frame: domain.Bounds{W: 1000, H: 800}, UsableFrame: domain.Bounds{Y: 25, W: 1000, H: 775}, Scale: 2, SpaceID: 1, SpaceKind: "ordinary", SpaceStatus: "known"}}}
}

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
		var z T
		return z
	}
}

func visible(t *testing.T, f *fixture) Presentation {
	t.Helper()
	for {
		s := recv(t, f.views)
		for _, p := range s.Presentations {
			if p.Visible && len(p.Items) > 0 {
				return p
			}
		}
	}
}

func start(t *testing.T, f *fixture) (*Controller, context.CancelFunc, chan error) {
	t.Helper()
	c := New(Deps{Environment: f, Applications: f, Identities: f, View: f})
	s := config.DefaultReplacementDock()
	s.Enabled = true
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	recv(t, f.starts)
	f.send(env())
	return c, cancel, done
}

func TestLauncherRetiresScopeBeforeBlockedSourceJoin(t *testing.T) {
	f := &fixture{starts: make(chan struct{}, 4), joins: make(chan struct{}), views: make(chan State, 64)}
	c, cancel, done := start(t, f)
	p := visible(t, f)
	c.Suspend(true)
	c.Suspend(false)
	if err := c.Activate(context.Background(), p.Scope, p.Items[0].ID); err != ErrRetired {
		t.Fatalf("old scope admitted: %v", err)
	}
	select {
	case <-f.starts:
		t.Fatal("restarted before join")
	default:
	}
	close(f.joins)
	recv(t, f.starts)
	cancel()
	if err := recv(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestLauncherSnapshotCopiesAndUnknownYields(t *testing.T) {
	f := &fixture{starts: make(chan struct{}, 4), views: make(chan State, 64)}
	c, cancel, done := start(t, f)
	p := visible(t, f)
	snap := c.Snapshot()
	snap.Presentations[0].Items[0].Name = "mutated"
	if c.Snapshot().Presentations[0].Items[0].Name == "mutated" {
		t.Fatal("snapshot alias")
	}
	e := env()
	e.Sequence = 2
	e.Displays[0].SpaceKind = "unknown"
	f.send(e)
	for {
		s := recv(t, f.views)
		if len(s.Presentations) > 0 && !s.Presentations[0].Visible {
			break
		}
	}
	if err := c.Activate(context.Background(), p.Scope, p.Items[0].ID); err != ErrRetired {
		t.Fatal("unknown space admitted")
	}
	cancel()
	if err := recv(t, done); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestLauncherGeometryProtectedEdgeAndInvalidSpace(t *testing.T) {
	e := env()
	p := config.DefaultReplacementDock().Profiles[0]
	g := Layout(p, e.Displays[0], 100, nil)
	if g.Status != "ready" || g.Bounds.Y+g.Bounds.H > 768 || g.Bounds.W > 800 {
		t.Fatalf("unsafe bounds %+v", g)
	}
	e.Displays[0].UsableFrame = domain.Bounds{W: 20, H: 20}
	if Layout(p, e.Displays[0], 1, nil).Status == "ready" {
		t.Fatal("unusable display admitted")
	}
}
