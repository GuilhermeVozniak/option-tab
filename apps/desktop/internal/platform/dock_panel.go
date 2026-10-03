package platform

import (
	"errors"
	"math"
	"sync"
	"unsafe"

	"option-tab/internal/domain"
)

var (
	ErrDockPanelHostClosed = errors.New("dock panel host is unavailable")
	ErrDockPanelClosed     = errors.New("dock panel is closed")
	ErrDockPanelBounds     = errors.New("dock panel bounds must be finite and positive")
)

// DockPanel hosts an independently retained, dedicated hidden Wails content view.
// Bounds are global top-left points; Close is terminal and idempotent.
type DockPanel interface {
	Show(domain.Bounds) error
	Hide() error
	Close() error
}
type DockPanelHost interface {
	CreateDockPanel(unsafe.Pointer) (DockPanel, error)
}
type dockPanelNative interface {
	Show(uint64, domain.Bounds) error
	Hide(uint64) error
	Close(uint64) error
}
type dockPanel struct {
	mu     sync.Mutex
	token  uint64
	native dockPanelNative
	closed bool
}

func newDockPanel(token uint64, native dockPanelNative) *dockPanel {
	return &dockPanel{token: token, native: native}
}

// Never hold a Go lock across synchronous AppKit dispatch: a close notification
// or another call on the UI thread must be able to retire the token immediately.
// Concurrent native operations linearize in the native main-thread registry.
func (p *dockPanel) Show(b domain.Bounds) error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return ErrDockPanelClosed
	}
	for _, v := range []float64{b.X, b.Y, b.W, b.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return ErrDockPanelBounds
		}
	}
	if b.W <= 0 || b.H <= 0 {
		return ErrDockPanelBounds
	}
	return p.recordHostLoss(p.native.Show(p.token, b))
}

func (p *dockPanel) Hide() error {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil
	}
	return p.recordHostLoss(p.native.Hide(p.token))
}

func (p *dockPanel) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	return p.native.Close(p.token)
}

func (p *dockPanel) recordHostLoss(err error) error {
	if errors.Is(err, ErrDockPanelHostClosed) {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
	}
	return err
}
