package platform

import (
	"sync"

	"option-tab/internal/domain"
)

// Process start time disambiguates PID reuse. Titles and screen visibility are
// deliberately absent: neither establishes closure or a new process lifetime.
type windowIdentity struct {
	Window    domain.WindowID `json:"window"`
	PID       int             `json:"pid"`
	StartSec  uint64          `json:"startSec"`
	StartUsec uint64          `json:"startUsec"`
}

func (i windowIdentity) valid() bool {
	return i.Window != 0 && i.PID > 0 && i.StartSec != 0 && i.StartUsec < 1_000_000
}

type windowObservation struct {
	identity   windowIdentity
	generation uint64
}
type retiredWindow struct {
	windowObservation
	revision uint64
	retired  bool
}
type windowRetirements struct {
	mu       sync.Mutex
	sequence uint64
	windows  map[domain.WindowID]retiredWindow
}

func (r *windowRetirements) observe(id windowIdentity) windowObservation {
	if !id.valid() {
		return windowObservation{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.windows == nil {
		r.windows = make(map[domain.WindowID]retiredWindow)
	}
	previous := r.windows[id.Window]
	r.sequence++
	ticket := windowObservation{id, r.sequence}
	r.windows[id.Window] = retiredWindow{ticket, r.sequence, previous.identity == id && previous.retired}
	return ticket
}

// destroyed is called only for an exact AX destruction notification. Generic
// stream errors/cancellation use finish and never create retirement evidence.
func (r *windowRetirements) destroyed(ticket windowObservation) bool {
	if !ticket.identity.valid() {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.windows[ticket.identity.Window]
	if !ok || current.windowObservation != ticket {
		return false
	}
	r.sequence++
	current.revision = r.sequence
	current.retired = true
	r.windows[ticket.identity.Window] = current
	return true
}

func (r *windowRetirements) finish(ticket windowObservation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if current, ok := r.windows[ticket.identity.Window]; ok && current.windowObservation == ticket && !current.retired {
		delete(r.windows, ticket.identity.Window)
	}
}

// filter never invokes native queries under mu. Only a complete successful CG
// inventory may prune. A failed/unknown identity fails open without erasing the
// positive evidence. Fresh AX-root presence can clear it, but never if another
// destruction or observation arrived while that validation was in flight.
func (r *windowRetirements) filter(windows []domain.Window, inventory func() ([]windowIdentity, bool), reappeared func(windowIdentity) bool) []domain.Window {
	r.mu.Lock()
	snapshots := make(map[domain.WindowID]retiredWindow)
	for id, state := range r.windows {
		if state.retired {
			snapshots[id] = state
		}
	}
	r.mu.Unlock()
	if len(snapshots) == 0 || inventory == nil {
		return windows
	}
	identities, complete := inventory()
	if !complete {
		return windows
	}
	currentIDs := make(map[domain.WindowID]windowIdentity, len(identities))
	for _, id := range identities {
		currentIDs[id.Window] = id
	}
	candidates := make(map[domain.WindowID]int, len(windows))
	for _, w := range windows {
		candidates[w.ID] = w.PID
	}
	excluded := make(map[domain.WindowID]bool)
	for id, previous := range snapshots {
		identity, exists := currentIDs[id]
		// A missing CG identity, changed owner, or positively known new launch can
		// retire old evidence. An unreadable process launch cannot.
		changed := !exists || identity.PID != previous.identity.PID || (identity.valid() && identity != previous.identity)
		matches := exists && identity.valid() && identity == previous.identity
		present := false
		if matches && candidates[id] == identity.PID && reappeared != nil {
			present = reappeared(identity)
		}
		r.mu.Lock()
		current, ok := r.windows[id]
		if ok && current.identity == previous.identity {
			sameRevision := current.revision == previous.revision
			if changed && sameRevision {
				delete(r.windows, id)
			} else if matches {
				if present && sameRevision {
					delete(r.windows, id) // Invalidates old observer tickets and releases finished state.
					current.retired = false
				}
				excluded[id] = current.retired && candidates[id] == identity.PID
			}
		}
		r.mu.Unlock()
	}
	result := make([]domain.Window, 0, len(windows))
	for _, w := range windows {
		if !excluded[w.ID] {
			result = append(result, w)
		}
	}
	return result
}
