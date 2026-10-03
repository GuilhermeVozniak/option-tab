package platform

import (
	"errors"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type panelRecorder struct {
	calls []uint64
	fail  error
}

func (n *panelRecorder) Show(id uint64, b domain.Bounds) error {
	n.calls = append(n.calls, id)
	return n.fail
}
func (n *panelRecorder) Hide(id uint64) error  { n.calls = append(n.calls, id); return n.fail }
func (n *panelRecorder) Close(id uint64) error { n.calls = append(n.calls, id); return n.fail }
func TestDockPanelTerminalLifecycle(t *testing.T) {
	n := &panelRecorder{}
	p := newDockPanel(1, n)
	if err := p.Show(domain.Bounds{W: 240, H: 120}); err != nil {
		t.Fatal(err)
	}
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	if err := p.Show(domain.Bounds{W: 240, H: 120}); err == nil {
		t.Fatal("closed panel shown")
	}
	if len(n.calls) != 4 {
		t.Fatalf("retired/repeated operations reached native: %v", n.calls)
	}
}

func TestDockPanelOldCloseCannotRetireNewHost(t *testing.T) {
	n := &panelRecorder{}
	old := newDockPanel(1, n)
	next := newDockPanel(2, n)
	_ = old.Close()
	_ = old.Close()
	if err := next.Show(domain.Bounds{W: 120, H: 100}); err != nil {
		t.Fatal(err)
	}
	if len(n.calls) != 2 || n.calls[0] != 1 || n.calls[1] != 2 {
		t.Fatalf("wrong native generation: %v", n.calls)
	}
}

func TestDockPanelLostHostIsTerminal(t *testing.T) {
	n := &panelRecorder{fail: ErrDockPanelHostClosed}
	p := newDockPanel(1, n)
	if !errors.Is(p.Show(domain.Bounds{W: 240, H: 120}), ErrDockPanelHostClosed) {
		t.Fatal("missing host-loss error")
	}
	n.fail = nil
	if err := p.Show(domain.Bounds{W: 240, H: 120}); err == nil {
		t.Fatal("lost host revived")
	}
	_ = p.Close()
	if len(n.calls) != 1 {
		t.Fatalf("lost host messaged: %v", n.calls)
	}
}

type reentrantPanelNative struct {
	panel  *dockPanel
	closed bool
}

func (n *reentrantPanelNative) Show(uint64, domain.Bounds) error { return n.panel.Close() }
func (n *reentrantPanelNative) Hide(uint64) error                { return nil }
func (n *reentrantPanelNative) Close(uint64) error               { n.closed = true; return nil }
func TestDockPanelDoesNotHoldGoLockAcrossNativeDispatch(t *testing.T) {
	n := &reentrantPanelNative{}
	p := newDockPanel(1, n)
	n.panel = p
	done := make(chan struct{})
	go func() { _ = p.Show(domain.Bounds{W: 100, H: 100}); close(done) }()
	select {
	case <-done:
		if !n.closed {
			t.Fatal("reentrant retirement failed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Go lifecycle lock held across native main-thread dispatch")
	}
}
