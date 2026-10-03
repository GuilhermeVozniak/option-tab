package platform

import (
	"testing"

	"option-tab/internal/domain"
)

type fakeMediaPanel struct{ closes int }

func (*fakeMediaPanel) Show(domain.Bounds) error { return nil }
func (*fakeMediaPanel) Hide() error              { return nil }
func (p *fakeMediaPanel) Close() error           { p.closes++; return nil }

type emptyMediaEvents struct{}

func (emptyMediaEvents) Next(uint64) (MediaPanelEvent, int) { return MediaPanelEvent{}, 0 }
func TestMediaPanelAdmissionAndTerminalClose(t *testing.T) {
	base := &fakeMediaPanel{}
	var received []MediaPanelEvent
	p := newMediaPanel(base, emptyMediaEvents{}, 1, 7, func(e MediaPanelEvent) { received = append(received, e) })
	if err := p.Show(domain.Bounds{W: 100, H: 100}); err != nil {
		t.Fatal(err)
	}
	p.accept(MediaPanelEvent{Session: 8, Sequence: 1})
	p.accept(MediaPanelEvent{Session: 7, Sequence: 1})
	p.accept(MediaPanelEvent{Session: 7, Sequence: 1})
	if len(received) != 1 {
		t.Fatal("scope or sequence admission")
	}
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	p.accept(MediaPanelEvent{Session: 7, Sequence: 2})
	if len(received) != 1 {
		t.Fatal("hidden event published")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if base.closes != 1 {
		t.Fatal("close not idempotent")
	}
	if err := p.Show(domain.Bounds{W: 1, H: 1}); err == nil {
		t.Fatal("terminal host reopened")
	}
}

func TestMediaPanelCallbackOutsideLocks(t *testing.T) {
	p := newMediaPanel(&fakeMediaPanel{}, emptyMediaEvents{}, 1, 1, nil)
	p.emit = func(MediaPanelEvent) {
		if err := p.Hide(); err != nil {
			t.Error(err)
		}
	}
	if err := p.Show(domain.Bounds{W: 1, H: 1}); err != nil {
		t.Fatal(err)
	}
	p.accept(MediaPanelEvent{Session: 1, Sequence: 1})
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

type blockedMediaShow struct {
	fakeMediaPanel
	entered, release chan struct{}
}

func (p *blockedMediaShow) Show(domain.Bounds) error { close(p.entered); <-p.release; return nil }
func TestMediaPanelHideRetiresDelayedShow(t *testing.T) {
	base := &blockedMediaShow{entered: make(chan struct{}), release: make(chan struct{})}
	p := newMediaPanel(base, emptyMediaEvents{}, 1, 1, nil)
	done := make(chan error, 1)
	go func() { done <- p.Show(domain.Bounds{W: 1, H: 1}) }()
	<-base.entered
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	close(base.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	visible := p.visible
	p.mu.Unlock()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Fatal("late Show reactivated hidden callback admission")
	}
}

type heldMediaPanelEvent struct {
	entered, release, drained chan struct{}
	calls                     int
}

func (n *heldMediaPanelEvent) Next(uint64) (MediaPanelEvent, int) {
	n.calls++
	if n.calls == 1 {
		close(n.entered)
		<-n.release
		return MediaPanelEvent{Session: 1, Sequence: 1, Reason: "moved"}, 1
	}
	if n.calls == 2 {
		close(n.drained)
	}
	return MediaPanelEvent{}, 0
}

func TestMediaPanelReopenDropsFetchedPriorVisibilityEvent(t *testing.T) {
	native := &heldMediaPanelEvent{entered: make(chan struct{}), release: make(chan struct{}), drained: make(chan struct{})}
	events := make(chan MediaPanelEvent, 1)
	p := newMediaPanel(&fakeMediaPanel{}, native, 1, 1, func(e MediaPanelEvent) { events <- e })
	if err := p.Show(domain.Bounds{W: 1, H: 1}); err != nil {
		t.Fatal(err)
	}
	<-native.entered
	if err := p.Hide(); err != nil {
		t.Fatal(err)
	}
	if err := p.Show(domain.Bounds{X: 20, W: 1, H: 1}); err != nil {
		t.Fatal(err)
	}
	close(native.release)
	<-native.drained
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
		t.Fatal("fetched event from prior visibility published after reopen")
	default:
	}
}

type mediaResetBase struct {
	fakeMediaPanel
	hides int
}

func (p *mediaResetBase) Hide() error { p.hides++; return nil }
func TestMediaPanelRetirementClearsNativeQueueBeforeCoalescedResume(t *testing.T) {
	base := &mediaResetBase{}
	p := newMediaPanel(base, emptyMediaEvents{}, 1, 1, nil)
	if err := p.Show(domain.Bounds{W: 100, H: 100}); err != nil {
		t.Fatal(err)
	}
	p.RetireMediaPanelEvents()
	if err := p.Show(domain.Bounds{W: 100, H: 100}); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if base.hides != 1 {
		t.Fatal("coalesced resume did not clear retired native queue")
	}
}
