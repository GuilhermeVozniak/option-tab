// Package preview owns cancellable, bounded window capture sessions.
package preview

import (
	"context"
	"errors"
	"sync"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type StreamSource interface {
	StreamWindow(context.Context, domain.WindowID, int, func(string)) error
}
type SnapshotSource interface {
	ThumbnailDataURL(domain.WindowID, int) string
}
type job struct {
	cancel      context.CancelFunc
	px          int
	live        bool
	unavailable bool
}
type Manager struct {
	mu           sync.Mutex
	closed       bool
	liveLimit    int
	source       any
	emit         func(domain.WindowID, string)
	emitIdentity func(domain.WindowID, string, platform.AutomationWindowIdentity)
	jobs         map[domain.WindowID]*job
	cache        map[domain.WindowID]string
	exportCache  map[domain.WindowID]CachedFrame
	streams      chan struct{}
	snapshots    chan struct{}
}

func New(source any, emit func(domain.WindowID, string)) *Manager {
	return &Manager{source: source, emit: emit, jobs: map[domain.WindowID]*job{}, cache: map[domain.WindowID]string{}, exportCache: map[domain.WindowID]CachedFrame{}, streams: make(chan struct{}, 4), snapshots: make(chan struct{}, 1), liveLimit: 4}
}

// NewPeer creates the second independent presentation owner. Call during App
// construction, before capture starts. Each owner gets two live priorities;
// both share the four-stream and one-snapshot process budget.
func (m *Manager) NewPeer(emit func(domain.WindowID, string)) *Manager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.liveLimit = 2
	peer := New(m.source, emit)
	peer.liveLimit = 2
	peer.streams, peer.snapshots = m.streams, m.snapshots
	peer.closed = m.closed
	return peer
}

func (m *Manager) NewPeerWithIdentity(emit func(domain.WindowID, string, platform.AutomationWindowIdentity)) *Manager {
	peer := m.NewPeer(func(domain.WindowID, string) {})
	peer.emitIdentity = emit
	return peer
}

func (m *Manager) emitFrame(id domain.WindowID, url string, identity platform.AutomationWindowIdentity) {
	if m.emitIdentity != nil {
		m.emitIdentity(id, url, identity)
	} else {
		m.emit(id, url)
	}
}

// Update prioritizes selection and retains unchanged sessions. No native call
// runs on the caller (which may be the keyboard/controller event path).
func (m *Manager) Update(ids []domain.WindowID, selected domain.WindowID, px int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	order := []domain.WindowID{}
	seen := map[domain.WindowID]bool{}
	for _, id := range ids {
		if id == selected && id != 0 {
			order = append(order, id)
			seen[id] = true
			break
		}
	}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			order = append(order, id)
			seen[id] = true
		}
	}
	wanted := map[domain.WindowID]bool{}
	for i, id := range order {
		wanted[id] = i < m.liveLimit
	}
	for id, j := range m.jobs {
		live, ok := wanted[id]
		if !ok || (!j.unavailable && (j.px != px || live != j.live)) {
			j.cancel()
			delete(m.jobs, id)
		}
	}
	for id := range m.cache {
		if _, ok := wanted[id]; !ok {
			delete(m.cache, id)
			delete(m.exportCache, id)
		}
	}
	for _, id := range order {
		if m.jobs[id] != nil {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		j := &job{cancel: cancel, px: px, live: wanted[id]}
		m.jobs[id] = j
		go m.run(ctx, id, j)
	}
}

func (m *Manager) run(ctx context.Context, id domain.WindowID, j *job) {
	sem := m.snapshots
	if j.live {
		sem = m.streams
	}
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return
	}
	defer func() { <-sem }()
	identity := m.captureIdentity(id)
	bound, hasBoundSource := m.source.(platform.IdentityWindowCaptureSource)
	if !hasBoundSource {
		identity = platform.AutomationWindowIdentity{}
	}
	frame := func(url string) {
		if url == "" {
			return
		}
		exported := m.exportFrame(identity, url)
		m.mu.Lock()
		defer m.mu.Unlock()
		if ctx.Err() != nil || m.jobs[id] != j {
			return
		}
		if len(m.cache) < 30 || m.cache[id] != "" {
			m.cache[id] = url
			if exported != nil {
				m.exportCache[id] = *exported
			} else {
				delete(m.exportCache, id)
			}
		}
		m.emitFrame(id, url, identity)
	}
	if ctx.Err() != nil {
		return
	}
	if s, ok := m.source.(StreamSource); ok && j.live {
		var err error
		if identity.ID != 0 {
			err = bound.StreamWindowWithIdentity(ctx, identity, j.px, frame)
		} else {
			err = s.StreamWindow(ctx, id, j.px, frame)
		}
		if err == nil || ctx.Err() != nil {
			return
		}
		var unavailable interface{ WindowUnavailable() bool }
		if errors.As(err, &unavailable) && unavailable.WindowUnavailable() {
			m.retireCapture(ctx, id, j, identity)
			return // retain the completed job until the next target-list change
		}
	}
	if s, ok := m.source.(SnapshotSource); ok && ctx.Err() == nil {
		// A live job holds a stream lease, not the shared snapshot lease. Its
		// fallback must join that budget too, including across independent owners.
		if j.live {
			select {
			case m.snapshots <- struct{}{}:
				defer func() { <-m.snapshots }()
			case <-ctx.Done():
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		if identity.ID != 0 {
			url, err := bound.ThumbnailDataURLWithIdentity(ctx, identity, j.px)
			if err == nil {
				frame(url)
			} else {
				var unavailable interface{ WindowUnavailable() bool }
				if errors.As(err, &unavailable) && unavailable.WindowUnavailable() {
					m.retireCapture(ctx, id, j, identity)
				}
			}
		} else {
			frame(s.ThumbnailDataURL(id, j.px))
		}
	}
}

func (m *Manager) retireCapture(ctx context.Context, id domain.WindowID, j *job, identity platform.AutomationWindowIdentity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() == nil && m.jobs[id] == j {
		j.unavailable = true
		delete(m.cache, id)
		delete(m.exportCache, id)
		m.emitFrame(id, "", identity)
	}
}

func (m *Manager) Hide() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, j := range m.jobs {
		j.cancel()
		delete(m.jobs, id)
	}
}

func (m *Manager) Cached() map[domain.WindowID]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[domain.WindowID]string{}
	for id, url := range m.cache {
		out[id] = url
	}
	return out
}

// Close permanently rejects capture admission, including updates queued before shutdown.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	clear(m.exportCache)
	for id, j := range m.jobs {
		j.cancel()
		delete(m.jobs, id)
	}
}
