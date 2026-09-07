package platform

import (
	"sync"
	"time"
	"unsafe"

	"option-tab/internal/domain"
)

// These capabilities are Go-only: they never dispatch AppKit or invoke callbacks.
type (
	MediaPanelEventValidator interface{ MediaPanelEventCurrent(MediaPanelEvent) bool }
	MediaPanelEventRetirer   interface{ RetireMediaPanelEvents() }
)

type (
	MediaPanelEvent struct {
		VisibilityEpoch     uint64        `json:"-"`
		HostIncarnation     uint64        `json:"-"`
		HostVisibilityEpoch uint64        `json:"-"`
		Session             uint64        `json:"session"`
		Sequence            uint64        `json:"sequence"`
		Bounds              domain.Bounds `json:"bounds"`
		DisplayUUID         string        `json:"displayUUID"`
		Reason              string        `json:"reason"`
	}
	MediaPanelHost interface {
		CreateMediaPanel(unsafe.Pointer, uint64, func(MediaPanelEvent)) (DockPanel, error)
	}
	mediaPanelEvents interface {
		Next(uint64) (MediaPanelEvent, int)
	}
	mediaPanel struct {
		base            DockPanel
		native          mediaPanelEvents
		token, session  uint64
		mu              sync.Mutex
		visible, closed bool
		retired         bool
		sequence        uint64
		revision        uint64
		emit            func(MediaPanelEvent)
		stop            chan struct{}
	}
)

func newMediaPanel(base DockPanel, native mediaPanelEvents, token, session uint64, emit func(MediaPanelEvent)) *mediaPanel {
	p := &mediaPanel{base: base, native: native, token: token, session: session, emit: emit, stop: make(chan struct{})}
	go p.run()
	return p
}

func (p *mediaPanel) Show(bounds domain.Bounds) error {
	p.mu.Lock()
	closed := p.closed
	p.revision++
	revision := p.revision
	retired := p.retired
	if !closed && !retired {
		p.visible = true
	}
	p.mu.Unlock()
	if closed {
		return ErrDockPanelClosed
	}

	if retired {
		if err := p.base.Hide(); err != nil {
			return err
		}
		p.mu.Lock()
		if p.closed || p.revision != revision {
			p.mu.Unlock()
			return nil
		}
		p.revision++
		revision = p.revision
		p.retired = false
		p.visible = true
		p.mu.Unlock()
	}
	if err := p.base.Show(bounds); err != nil {
		p.mu.Lock()
		if p.revision == revision {
			p.visible = false
		}
		p.mu.Unlock()
		return err
	}
	p.mu.Lock()
	if !p.closed && p.revision == revision {
		p.visible = true
	}
	p.mu.Unlock()
	return nil
}

func (p *mediaPanel) Hide() error {
	p.mu.Lock()
	p.visible = false
	p.revision++
	p.mu.Unlock()
	return p.base.Hide()
}

func (p *mediaPanel) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.visible = false
	close(p.stop)
	p.mu.Unlock()
	return p.base.Close()
}

func (p *mediaPanel) accept(event MediaPanelEvent) {
	p.mu.Lock()
	revision := p.revision
	p.mu.Unlock()
	p.acceptRevision(event, revision)
}

func (p *mediaPanel) acceptRevision(event MediaPanelEvent, revision uint64) {
	p.mu.Lock()
	if event.Reason != "hostClosed" && p.revision != revision {
		p.mu.Unlock()
		return
	}
	if p.closed || event.Session != p.session || event.Sequence <= p.sequence || (!p.visible && event.Reason != "hostClosed") {
		p.mu.Unlock()
		return
	}
	p.sequence = event.Sequence
	event.VisibilityEpoch = p.revision
	emit := p.emit
	if event.Reason == "hostClosed" {
		p.closed = true
		p.visible = false
		close(p.stop)
	}
	p.mu.Unlock()
	if emit != nil {
		emit(event)
	}
}

func (p *mediaPanel) run() {
	timer := time.NewTicker(30 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-timer.C:
			p.mu.Lock()
			revision := p.revision
			p.mu.Unlock()
			event, status := p.native.Next(p.token)
			if status > 0 {
				p.acceptRevision(event, revision)
			}
			if status < 0 {
				return
			}
		}
	}
}

func (p *mediaPanel) RetireMediaPanelEvents() {
	p.mu.Lock()
	p.retired = true
	p.revision++
	p.visible = false
	p.mu.Unlock()
}

func (p *mediaPanel) MediaPanelEventCurrent(event MediaPanelEvent) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if event.Session != p.session || event.Sequence == 0 || event.Sequence != p.sequence {
		return false
	}
	if event.Reason == "hostClosed" {
		return p.closed
	}
	return !p.closed && p.visible && event.VisibilityEpoch == p.revision
}
