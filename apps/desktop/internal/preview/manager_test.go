package preview

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type source struct {
	mu          sync.Mutex
	active      map[domain.WindowID]func(string)
	max         int
	unsupported bool
	snapshots   int
}

func (s *source) StreamWindow(ctx context.Context, id domain.WindowID, px int, frame func(string)) error {
	if s.unsupported {
		return errors.New("unsupported")
	}
	s.mu.Lock()
	s.active[id] = frame
	if len(s.active) > s.max {
		s.max = len(s.active)
	}
	s.mu.Unlock()
	<-ctx.Done()
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	return nil
}

func (s *source) ThumbnailDataURL(id domain.WindowID, px int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots++
	return "snapshot"
}

func eventually(t *testing.T, f func() bool) {
	t.Helper()
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		if f() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestPrioritiesCancellationAndStaleFrames(t *testing.T) {
	s := &source{active: map[domain.WindowID]func(string){}}
	var mu sync.Mutex
	emitted := 0
	m := New(s, func(domain.WindowID, string) { mu.Lock(); emitted++; mu.Unlock() })
	defer m.Hide()
	m.Update([]domain.WindowID{1, 2, 3, 4, 5}, 5, 256)
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.active) == 4 && s.active[5] != nil })
	s.mu.Lock()
	late := s.active[5]
	s.mu.Unlock()
	late("old")
	m.Update([]domain.WindowID{1, 2, 3, 4, 5}, 4, 256)
	eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.active) == 4 && s.active[4] != nil && s.active[5] == nil
	})
	m.Update([]domain.WindowID{1, 2, 3, 4}, 4, 256)
	if _, ok := m.Cached()[5]; ok {
		t.Fatal("closed window cached")
	}
	m.Hide()
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.active) == 0 })
	mu.Lock()
	before := emitted
	mu.Unlock()
	late("late")
	mu.Lock()
	defer mu.Unlock()
	if emitted != before {
		t.Fatal("late frame emitted")
	}
	if s.max > 4 {
		t.Fatalf("streams=%d", s.max)
	}
}

func TestUnsupportedUsesSnapshot(t *testing.T) {
	s := &source{unsupported: true}
	out := make(chan string, 1)
	m := New(s, func(_ domain.WindowID, url string) { out <- url })
	defer m.Hide()
	m.Update([]domain.WindowID{1}, 1, 256)
	select {
	case got := <-out:
		if got != "snapshot" {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("missing snapshot fallback")
	}
}

func TestClosePermanentlyRejectsQueuedUpdates(t *testing.T) {
	s := &source{active: map[domain.WindowID]func(string){}}
	m := New(s, func(domain.WindowID, string) {})
	m.Update([]domain.WindowID{1}, 1, 256)
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.active[1] != nil })
	m.Close()
	m.Update([]domain.WindowID{2}, 2, 256)
	m.mu.Lock()
	remaining := len(m.jobs)
	m.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("shutdown admitted %d jobs", remaining)
	}
	eventually(t, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.active) == 0 })
}

type unavailableError struct{}

func (unavailableError) Error() string           { return "window closed" }
func (unavailableError) WindowUnavailable() bool { return true }

type closedSource struct{ snapshots int }

func (s *closedSource) StreamWindow(ctx context.Context, id domain.WindowID, px int, frame func(string)) error {
	frame("visible")
	return unavailableError{}
}

func (s *closedSource) ThumbnailDataURL(domain.WindowID, int) string {
	s.snapshots++
	return "stale snapshot"
}

func TestClosedNativeWindowClearsCacheWithoutSnapshotRetry(t *testing.T) {
	s := &closedSource{}
	out := make(chan string, 4)
	m := New(s, func(_ domain.WindowID, url string) { out <- url })
	defer m.Close()
	m.Update([]domain.WindowID{1}, 1, 256)
	if got := <-out; got != "visible" {
		t.Fatal(got)
	}
	select {
	case got := <-out:
		if got != "" {
			t.Fatalf("closed window emitted %q instead of clearing image", got)
		}
	case <-time.After(time.Second):
		t.Fatal("closed image was not cleared")
	}
	if len(m.Cached()) != 0 {
		t.Fatal("closed image remained cached")
	}
	if s.snapshots != 0 {
		t.Fatal("closed window retried as snapshot")
	}
	m.mu.Lock()
	endedJob := m.jobs[1]
	m.mu.Unlock()
	m.Update([]domain.WindowID{1}, 1, 1024)
	m.mu.Lock()
	restarted := m.jobs[1] != endedJob
	m.mu.Unlock()
	if restarted {
		t.Fatal("resolution update restarted unavailable window")
	}
}
