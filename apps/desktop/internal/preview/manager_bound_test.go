package preview

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type queuedBoundSource struct {
	current  atomic.Pointer[platform.AutomationWindowIdentity]
	captured chan platform.AutomationWindowIdentity
	legacy   atomic.Int32
}

func (s *queuedBoundSource) WindowIdentity(domain.WindowID) (platform.AutomationWindowIdentity, error) {
	return *s.current.Load(), nil
}

func (s *queuedBoundSource) WindowIdentityCurrent(id platform.AutomationWindowIdentity) bool {
	return id == *s.current.Load()
}

func (s *queuedBoundSource) StreamWindow(context.Context, domain.WindowID, int, func(string)) error {
	s.legacy.Add(1)
	return nil
}

func (s *queuedBoundSource) ThumbnailDataURL(domain.WindowID, int) string { s.legacy.Add(1); return "" }

func (s *queuedBoundSource) StreamWindowWithIdentity(_ context.Context, id platform.AutomationWindowIdentity, _ int, _ func(string)) error {
	s.captured <- id
	return nil
}

func (s *queuedBoundSource) ThumbnailDataURLWithIdentity(_ context.Context, id platform.AutomationWindowIdentity, _ int) (string, error) {
	s.captured <- id
	return "", nil
}

func TestBoundCaptureNeverAdoptsReusedWindowWhileQueued(t *testing.T) {
	old := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 2, StartSeconds: 3}}
	replacement := old
	replacement.Process.PID = 4
	s := &queuedBoundSource{captured: make(chan platform.AutomationWindowIdentity, 1)}
	s.current.Store(&old)
	m := New(s, func(domain.WindowID, string) {})
	for range cap(m.streams) {
		m.streams <- struct{}{}
	}
	m.UpdateBound([]domain.WindowID{1}, 1, 64, map[domain.WindowID]platform.AutomationWindowIdentity{1: old})
	s.current.Store(&replacement)
	<-m.streams
	m.work.Wait()
	<-m.CloseAndDrain()
	select {
	case id := <-s.captured:
		t.Fatalf("captured replacement without presentation admission: %+v", id)
	default:
	}
	if s.legacy.Load() != 0 {
		t.Fatal("identity refusal fell back to legacy capture")
	}
}

func TestBoundCaptureCopiesAdmissionAndReplacesChangedIdentity(t *testing.T) {
	first := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 2, StartSeconds: 3}}
	second := first
	second.Process.StartSeconds++
	s := &queuedBoundSource{captured: make(chan platform.AutomationWindowIdentity, 4)}
	s.current.Store(&first)
	m := New(s, func(domain.WindowID, string) {})
	defer m.Close()
	for range cap(m.streams) {
		m.streams <- struct{}{}
	}
	identities := map[domain.WindowID]platform.AutomationWindowIdentity{1: first}
	m.UpdateBound([]domain.WindowID{1}, 1, 64, identities)
	identities[1] = second
	<-m.streams
	m.work.Wait()
	if got := <-s.captured; got != first {
		t.Fatalf("caller changed admitted job: %+v", got)
	}
	s.current.Store(&second)
	m.UpdateBound([]domain.WindowID{1}, 1, 64, identities)
	m.work.Wait()
	select {
	case got := <-s.captured:
		if got != second {
			t.Fatalf("wrong replacement %+v", got)
		}
	default:
		t.Fatal("same numeric ID suppressed new identity admission")
	}
}

func TestBoundCaptureInvalidAndUnsupportedNeverFallBack(t *testing.T) {
	id := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 2, StartSeconds: 3}}
	for _, expected := range []map[domain.WindowID]platform.AutomationWindowIdentity{nil, {}, {1: {}}, {1: {ID: 2, Process: id.Process}}, {1: {ID: 1, Process: platform.ProcessIdentity{PID: 2}}}} {
		s := &queuedBoundSource{captured: make(chan platform.AutomationWindowIdentity, 1)}
		s.current.Store(&id)
		m := New(s, func(domain.WindowID, string) {})
		m.UpdateBound([]domain.WindowID{1}, 1, 64, expected)
		m.work.Wait()
		<-m.CloseAndDrain()
		if len(s.captured) != 0 || s.legacy.Load() != 0 {
			t.Fatal("invalid bound identity entered capture")
		}
	}
	s := &source{active: map[domain.WindowID]func(string){}}
	m := New(s, func(domain.WindowID, string) {})
	m.UpdateBound([]domain.WindowID{1}, 1, 64, map[domain.WindowID]platform.AutomationWindowIdentity{1: id})
	m.work.Wait()
	<-m.CloseAndDrain()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.active) != 0 || s.snapshots != 0 {
		t.Fatal("unsupported source used unbound fallback")
	}
}

type boundFallbackSource struct {
	*queuedBoundSource
	streamDone chan struct{}
}

func (s *boundFallbackSource) StreamWindowWithIdentity(context.Context, platform.AutomationWindowIdentity, int, func(string)) error {
	close(s.streamDone)
	return errors.New("stream unavailable")
}

func TestBoundFallbackRechecksAfterSnapshotBudgetWait(t *testing.T) {
	original := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 2, StartSeconds: 3}}
	replacement := original
	replacement.Process.PID++
	s := &boundFallbackSource{queuedBoundSource: &queuedBoundSource{captured: make(chan platform.AutomationWindowIdentity, 1)}, streamDone: make(chan struct{})}
	s.current.Store(&original)
	m := New(s, func(domain.WindowID, string) {})
	m.snapshots <- struct{}{}
	m.UpdateBound([]domain.WindowID{1}, 1, 64, map[domain.WindowID]platform.AutomationWindowIdentity{1: original})
	<-s.streamDone
	s.current.Store(&replacement)
	<-m.snapshots
	m.work.Wait()
	<-m.CloseAndDrain()
	if len(s.captured) != 0 || s.legacy.Load() != 0 {
		t.Fatal("fallback captured replaced identity after waiting")
	}
}
