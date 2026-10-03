package preview

import (
	"context"
	"errors"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type drainSource struct {
	started chan domain.WindowID
	release chan struct{}
}

func (s *drainSource) StreamWindow(ctx context.Context, id domain.WindowID, _ int, _ func(string)) error {
	s.started <- id
	<-ctx.Done()
	<-s.release
	return ctx.Err()
}

func assertOpenDrain(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("receipt closed before work exited")
	default:
	}
}

func awaitDrain(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("drain did not complete")
	}
}

func TestDrainIncludesCancelledRemovedJobsAndQueuedJobs(t *testing.T) {
	s := &drainSource{make(chan domain.WindowID, 8), make(chan struct{})}
	m := New(s, func(domain.WindowID, string) {})
	m.Update([]domain.WindowID{1, 2, 3, 4}, 1, 64)
	for range 4 {
		<-s.started
	}
	m.Hide()
	m.Update([]domain.WindowID{5, 6, 7, 8}, 5, 64)
	receipt := m.CloseAndDrain()
	if receipt != m.CloseAndDrain() {
		t.Fatal("non-idempotent receipt")
	}
	assertOpenDrain(t, receipt)
	close(s.release)
	awaitDrain(t, receipt)
	select {
	case id := <-s.started:
		t.Fatalf("cancelled queued job entered source %d", id)
	default:
	}
}

func TestDrainReturnsWhileEmitBlockedAndPreservesSibling(t *testing.T) {
	s := &source{active: map[domain.WindowID]func(string){}}
	entered, release := make(chan struct{}), make(chan struct{})
	m := New(s, func(domain.WindowID, string) { close(entered); <-release })
	peer := m.NewPeer(func(domain.WindowID, string) {})
	m.Update([]domain.WindowID{1}, 1, 64)
	peer.Update([]domain.WindowID{2}, 2, 64)
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.active) == 2 })
	s.mu.Lock()
	emit := s.active[1]
	s.mu.Unlock()
	emitDone := make(chan struct{})
	go func() { emit("frame"); close(emitDone) }()
	<-entered
	returned := make(chan (<-chan struct{}), 1)
	go func() { returned <- m.CloseAndDrain() }()
	var receipt <-chan struct{}
	select {
	case receipt = <-returned:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("close waited on callback")
	}
	assertOpenDrain(t, receipt)
	close(release)
	<-emitDone
	awaitDrain(t, receipt)
	s.mu.Lock()
	alive := s.active[2] != nil
	s.mu.Unlock()
	if !alive {
		t.Fatal("sibling cancelled")
	}
	awaitDrain(t, peer.CloseAndDrain())
}

func TestDrainJoinsSnapshotFallbackAndCancelsItsWaiters(t *testing.T) {
	s := &failingPeerStreamSource{streams: make(chan struct{}, 4), snapshots: make(chan struct{}, 4), release: make(chan struct{})}
	m := New(s, func(domain.WindowID, string) { t.Error("closed fallback emitted") })
	m.Update([]domain.WindowID{1, 2, 3, 4}, 1, 64)
	for range 4 {
		<-s.streams
	}
	<-s.snapshots
	receipt := m.CloseAndDrain()
	assertOpenDrain(t, receipt)
	close(s.release)
	awaitDrain(t, receipt)
	if len(s.snapshots) != 0 || len(m.streams) != 0 || len(m.snapshots) != 0 {
		t.Fatal("fallback or queued budget not drained")
	}
}

type drainBoundSource struct {
	*identifiedSource
	entered, cancelled, release chan struct{}
}

func (s *drainBoundSource) StreamWindowWithIdentity(context.Context, platform.AutomationWindowIdentity, int, func(string)) error {
	return errors.New("fallback")
}

func (s *drainBoundSource) ThumbnailDataURLWithIdentity(ctx context.Context, _ platform.AutomationWindowIdentity, _ int) (string, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.cancelled)
	<-s.release
	return "", ctx.Err()
}

func TestDrainJoinsIdentityBoundThumbnailCleanup(t *testing.T) {
	s := &drainBoundSource{identifiedSource: &identifiedSource{source: &source{active: map[domain.WindowID]func(string){}}, identity: platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 23, StartSeconds: 1}}}, entered: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
	s.current.Store(true)
	m := New(s, func(domain.WindowID, string) { t.Error("cancelled bound capture emitted") })
	m.Update([]domain.WindowID{1}, 1, 64)
	<-s.entered
	receipt := m.CloseAndDrain()
	<-s.cancelled
	assertOpenDrain(t, receipt)
	close(s.release)
	awaitDrain(t, receipt)
}
