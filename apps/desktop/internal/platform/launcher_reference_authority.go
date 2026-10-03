package platform

import (
	"sync"
	"sync/atomic"
)

var launcherReferenceSequence atomic.Uint64

type (
	launcherReferenceEntry struct {
		ticket, store, revision uint64
		fingerprint             string
	}
	launcherReferenceAuthority struct {
		mu    sync.Mutex
		items map[string]launcherReferenceEntry
	}
)

func newLauncherReferenceAuthority() *launcherReferenceAuthority {
	return &launcherReferenceAuthority{items: make(map[string]launcherReferenceEntry)}
}

func (a *launcherReferenceAuthority) start(id string) uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.items[id]
	e.ticket = launcherReferenceSequence.Add(1)
	a.items[id] = e
	return e.ticket
}

func (a *launcherReferenceAuthority) publish(id string, ticket, store uint64, fingerprint string) (uint64, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.items[id]
	if e.ticket != ticket {
		return 0, false
	}
	if e.revision == 0 || e.store != store || e.fingerprint != fingerprint {
		e.revision = launcherReferenceSequence.Add(1)
		e.store = store
		e.fingerprint = fingerprint
	}
	a.items[id] = e
	return e.revision, true
}

func (a *launcherReferenceAuthority) current(id string, revision, store uint64, fingerprint string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.items[id]
	return ok && e.revision == revision && e.store == store && e.fingerprint == fingerprint
}

func (a *launcherReferenceAuthority) invalidate(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.items, id)
}
