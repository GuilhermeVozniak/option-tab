// Package mru tracks the most-recently-used order of windows. The platform
// layer calls Touch on every focus change; the switcher uses the resulting
// order (or Stamp) to present windows recency-first. It is safe for concurrent
// use because focus events arrive on a platform goroutine.
package mru

import (
	"sync"
	"time"

	"option-tab/internal/domain"
)

// Tracker maintains an ordered list of window ids, most-recent first.
type Tracker struct {
	mu    sync.Mutex
	order []domain.WindowID // index 0 == most recently used
}

// New returns an empty Tracker.
func New() *Tracker { return &Tracker{} }

// Touch marks id as the most recently used window, moving it to the front.
func (t *Tracker) Touch(id domain.WindowID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removeLocked(id)
	t.order = append([]domain.WindowID{id}, t.order...)
}

// Remove drops id from the tracker. Unknown ids are ignored.
func (t *Tracker) Remove(id domain.WindowID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removeLocked(id)
}

func (t *Tracker) removeLocked(id domain.WindowID) {
	for i, x := range t.order {
		if x == id {
			t.order = append(t.order[:i], t.order[i+1:]...)
			return
		}
	}
}

// Order returns a copy of the tracked ids, most-recent first.
func (t *Tracker) Order() []domain.WindowID {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]domain.WindowID, len(t.order))
	copy(out, t.order)
	return out
}

// Rank returns the zero-based position of id (0 == most recent) and whether it
// is tracked.
func (t *Tracker) Rank(id domain.WindowID) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, x := range t.order {
		if x == id {
			return i, true
		}
	}
	return 0, false
}

// Len returns the number of tracked windows.
func (t *Tracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.order)
}

// Stamp returns a copy of wins with LastFocused set from the tracker's recency
// order, so a more-recently-used window gets a later timestamp. Untracked
// windows keep their existing (possibly zero) LastFocused. This lets the order
// package sort by recency without the platform providing real timestamps.
func (t *Tracker) Stamp(wins []domain.Window) []domain.Window {
	t.mu.Lock()
	rank := make(map[domain.WindowID]int, len(t.order))
	n := len(t.order)
	for i, id := range t.order {
		rank[id] = i
	}
	t.mu.Unlock()

	base := time.Unix(1<<31, 0) // a fixed, far reference point
	out := make([]domain.Window, len(wins))
	copy(out, wins)
	for i := range out {
		if r, ok := rank[out[i].ID]; ok {
			// rank 0 (most recent) -> largest timestamp.
			out[i].LastFocused = base.Add(time.Duration(n-r) * time.Second)
		}
	}
	return out
}
