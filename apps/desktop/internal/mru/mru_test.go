package mru

import (
	"sync"
	"testing"

	"option-tab/internal/domain"
)

func TestTouch_MovesToFront(t *testing.T) {
	tr := New()
	tr.Touch(1)
	tr.Touch(2)
	tr.Touch(3)
	if got := tr.Order(); !eq(got, 3, 2, 1) {
		t.Errorf("Order = %v, want [3 2 1]", got)
	}
	tr.Touch(1) // re-touch oldest
	if got := tr.Order(); !eq(got, 1, 3, 2) {
		t.Errorf("Order after re-touch = %v, want [1 3 2]", got)
	}
}

func TestRank(t *testing.T) {
	tr := New()
	tr.Touch(10)
	tr.Touch(20)
	if r, ok := tr.Rank(20); !ok || r != 0 {
		t.Errorf("Rank(20) = %d,%v want 0,true", r, ok)
	}
	if r, ok := tr.Rank(10); !ok || r != 1 {
		t.Errorf("Rank(10) = %d,%v want 1,true", r, ok)
	}
	if _, ok := tr.Rank(99); ok {
		t.Error("Rank(unknown) should be !ok")
	}
}

func TestRemove(t *testing.T) {
	tr := New()
	tr.Touch(1)
	tr.Touch(2)
	tr.Touch(3)
	tr.Remove(2)
	if got := tr.Order(); !eq(got, 3, 1) {
		t.Errorf("Order after remove = %v, want [3 1]", got)
	}
	tr.Remove(99) // no-op
	if tr.Len() != 2 {
		t.Errorf("Len = %d, want 2", tr.Len())
	}
}

func TestStamp_SetsLastFocusedByRecency(t *testing.T) {
	tr := New()
	tr.Touch(1)
	tr.Touch(2) // 2 most recent
	ws := []domain.Window{{ID: 1}, {ID: 2}, {ID: 3}}
	out := tr.Stamp(ws)
	// 2 should have a later LastFocused than 1; 3 (untracked) should be zero.
	var w1, w2, w3 domain.Window
	for _, w := range out {
		switch w.ID {
		case 1:
			w1 = w
		case 2:
			w2 = w
		case 3:
			w3 = w
		}
	}
	if !w2.LastFocused.After(w1.LastFocused) {
		t.Error("more-recent window should have a later LastFocused")
	}
	if !w3.LastFocused.IsZero() {
		t.Error("untracked window should keep zero LastFocused")
	}
}

func TestStamp_DoesNotMutateInput(t *testing.T) {
	tr := New()
	tr.Touch(1)
	ws := []domain.Window{{ID: 1}}
	_ = tr.Stamp(ws)
	if !ws[0].LastFocused.IsZero() {
		t.Error("Stamp must not mutate the input slice")
	}
}

func TestConcurrentTouch_NoRace(t *testing.T) {
	tr := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tr.Touch(domain.WindowID(n % 5))
			_ = tr.Order()
			_, _ = tr.Rank(domain.WindowID(n % 5))
		}(i)
	}
	wg.Wait()
	if tr.Len() == 0 || tr.Len() > 5 {
		t.Errorf("Len = %d, want between 1 and 5", tr.Len())
	}
}

func eq(a []domain.WindowID, b ...domain.WindowID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
