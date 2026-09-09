package main

import (
	"errors"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type dockQueue chan func()

func (q dockQueue) next(t *testing.T) {
	t.Helper()
	select {
	case f := <-q:
		f()
	case <-time.After(time.Second):
		t.Fatal("UI work was not dispatched")
	}
}

type dockFakeWindow struct {
	fakeWindow
	closes  int
	onClose func()
}

func (w *dockFakeWindow) Close() {
	w.closes++
	if w.onClose != nil {
		w.onClose()
	}
}

type dockFakePanel struct {
	shows, hides, closes int
	err                  error
	onShow               func()
}

func (p *dockFakePanel) Show(domain.Bounds) error {
	p.shows++
	if p.onShow != nil {
		p.onShow()
	}
	return p.err
}
func (p *dockFakePanel) Hide() error  { p.hides++; return p.err }
func (p *dockFakePanel) Close() error { p.closes++; return nil }

type dockFakeHost struct {
	panels []*dockFakePanel
	err    error
}

func (h *dockFakeHost) CreateDockPanel(unsafe.Pointer) (platform.DockPanel, error) {
	if h.err != nil {
		return nil, h.err
	}
	p := &dockFakePanel{}
	h.panels = append(h.panels, p)
	return p, nil
}

func dockHarness() (*dockWindow, dockQueue, *dockFakeHost, *[]*dockFakeWindow) {
	q := make(dockQueue, 10)
	h := &dockFakeHost{}
	ws := &[]*dockFakeWindow{}
	d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow {
		w := &dockFakeWindow{}
		w.native = unsafe.Pointer(new(int))
		*ws = append(*ws, w)
		return w
	}, h)
	return d, q, h, ws
}

var dockTestBounds = domain.Bounds{X: 1, Y: 2, W: 300, H: 160}

func TestDockWindowStaleShowAndTerminalClose(t *testing.T) {
	d, q, h, ws := dockHarness()
	d.show(dockTestBounds)
	d.hide()
	q.next(t)
	if len(h.panels) != 0 {
		t.Fatal("hidden request created panel")
	}
	d.show(dockTestBounds)
	d.close()
	d.close()
	q.next(t)
	d.show(dockTestBounds)
	if len(*ws) != 0 {
		t.Fatal("terminal close created host")
	}
}

func TestDockWindowLifecycleAndHostLoss(t *testing.T) {
	d, q, h, ws := dockHarness()
	d.show(dockTestBounds)
	q.next(t)
	w := (*ws)[0]
	p := h.panels[0]
	if p.shows != 1 {
		t.Fatal("missing show")
	}
	for _, c := range w.calls {
		if c == "show" || c == "focus" {
			t.Fatal("activated Wails host")
		}
	}
	d.hide()
	q.next(t)
	if p.hides != 1 {
		t.Fatal("missing hide")
	}
	d.show(dockTestBounds)
	q.next(t)
	d.markHostClosedIf(w)
	q.next(t)
	if len(h.panels) != 2 || p.closes != 1 || w.closes != 0 {
		t.Fatal("host loss not recreated safely")
	}
	d.markHostClosedIf(w)
	d.hide()
	q.next(t)
	if h.panels[1].hides != 1 {
		t.Fatal("late close retired replacement")
	}
	d.close()
	q.next(t)
	if h.panels[1].closes != 1 || (*ws)[1].closes != 1 {
		t.Fatal("resources not closed")
	}
}

func TestDockWindowNativeErrorCanRetry(t *testing.T) {
	d, q, h, ws := dockHarness()
	h.err = errors.New("create failed")
	d.show(dockTestBounds)
	q.next(t)
	if (*ws)[0].closes != 1 {
		t.Fatal("failed host leaked")
	}
	h.err = nil
	d.show(dockTestBounds)
	q.next(t)
	h.panels[0].err = platform.ErrDockPanelHostClosed
	d.show(dockTestBounds)
	q.next(t)
	if (*ws)[1].closes != 0 {
		t.Fatal("messaged destroyed host")
	}
	d.show(dockTestBounds)
	q.next(t)
	if len(h.panels) != 2 {
		t.Fatal("error prevented recreation")
	}
	d.close()
	q.next(t)
}

func TestDockWindowInlineDispatcherDoesNotBlockCaller(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	d := newDockWindow(func(f func()) { close(entered); <-release; f() }, func() nativeWindow { return nil }, &dockFakeHost{})
	go func() { d.show(dockTestBounds); d.hide(); d.close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("caller blocked in dispatcher")
	}
	<-entered
	close(release)
}

func TestDockWindowReentrantHideDuringShow(t *testing.T) {
	d, q, h, _ := dockHarness()
	d.show(dockTestBounds)
	q.next(t)
	h.panels[0].onShow = d.hide
	d.show(dockTestBounds)
	q.next(t)
	q.next(t)
	if h.panels[0].hides != 1 {
		t.Fatal("reentrant hide lost")
	}
	d.close()
	q.next(t)
}

func TestDockWindowCloseDuringFactoryDiscardsHost(t *testing.T) {
	q := make(dockQueue, 10)
	h := &dockFakeHost{}
	w := &dockFakeWindow{}
	var d *dockWindow
	d = newDockWindow(func(f func()) { q <- f }, func() nativeWindow { d.close(); return w }, h)
	d.show(dockTestBounds)
	q.next(t)
	q.next(t)
	if len(h.panels) != 0 || w.closes != 1 {
		t.Fatal("stale factory result was retained")
	}
}

func TestDockWindowCloseCallbackDuringPanelCleanup(t *testing.T) {
	d, q, h, ws := dockHarness()
	d.show(dockTestBounds)
	q.next(t)
	w := (*ws)[0]
	w.onClose = func() { d.markHostClosedIf(w) }
	d.close()
	q.next(t)
	if w.closes != 1 || h.panels[0].closes != 1 {
		t.Fatal("reentrant close leaked resources")
	}
}
