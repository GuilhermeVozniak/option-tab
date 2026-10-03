package main

import (
	"testing"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// This fake models only the strictly Go-only validity/retirement capability.
type leaseMediaPanel struct {
	dockFakePanel
	epoch uint64
}

func (p *leaseMediaPanel) RetireMediaPanelEvents() { p.epoch++ }
func (p *leaseMediaPanel) MediaPanelEventCurrent(e platform.MediaPanelEvent) bool {
	return e.VisibilityEpoch == p.epoch
}

func TestAppMediaPinOldHostCannotRetireReplacement(t *testing.T) {
	a, _ := newAppMediaFixture(t)
	d, _, _, _ := dockHarness()
	old, newWindow := &dockFakeWindow{}, &dockFakeWindow{}
	old.native = unsafe.Pointer(new(int))
	newWindow.native = unsafe.Pointer(new(int))
	native := &leaseMediaPanel{epoch: 1}
	d.visible = true
	d.visibility = 1
	d.current = &dockWindowResources{window: newLiveWindow(newWindow), panel: native, nativeHost: newWindow.native, incarnation: 2}
	a.viewMu.Lock()
	p := a.newMediaPresentationLocked(platform.MediaMusic, true)
	p.host = d
	p.bounds = domain.Bounds{W: 100, H: 100}
	session := p.state.Session
	a.viewMu.Unlock()
	if _, ok := d.stampMediaPanelEvent(newWindow.native, 1, platform.MediaPanelEvent{}); ok {
		t.Fatal("reused pointer relabelled old incarnation")
	}
	if d.markHostClosedIf(old) {
		t.Fatal("retired host notification matched replacement")
	}
	a.mediaPinHostClosed(session, d, old)
	if a.GetMediaState(session) == nil {
		t.Fatal("old host close retired replacement")
	}
	a.acceptMediaPanelEvent(platform.MediaPanelEvent{Session: session, Sequence: 999, HostIncarnation: 1, HostVisibilityEpoch: 1, VisibilityEpoch: 1, Reason: "hostClosed"})
	if a.GetMediaState(session) == nil {
		t.Fatal("old native close retired replacement")
	}
	fresh := platform.MediaPanelEvent{Session: session, Sequence: 1, HostIncarnation: 2, HostVisibilityEpoch: 1, VisibilityEpoch: 1, Reason: "moved", Bounds: domain.Bounds{X: 40, W: 100, H: 100}}
	p.nativeSequence = 999 // stale high-water mark from the old native host.
	a.acceptMediaPanelEvent(fresh)
	a.viewMu.Lock()
	x := p.bounds.X
	a.viewMu.Unlock()
	if x != 40 {
		t.Fatal("replacement's first native sequence rejected")
	}
}

func TestAppMediaPinAdmittedEventRefusedAfterCoalescedSuspendResume(t *testing.T) {
	a, _ := newAppMediaFixture(t)
	d, _, _, _ := dockHarness()
	w := &dockFakeWindow{}
	w.native = unsafe.Pointer(new(int))
	native := &leaseMediaPanel{epoch: 1}
	d.visible = true
	d.visibility = 1
	d.current = &dockWindowResources{window: newLiveWindow(w), panel: native, nativeHost: w.native, incarnation: 1}
	a.viewMu.Lock()
	p := a.newMediaPresentationLocked(platform.MediaMusic, true)
	p.host = d
	p.bounds = domain.Bounds{X: 5, W: 100, H: 100}
	a.viewMu.Unlock()
	event := platform.MediaPanelEvent{Session: p.state.Session, Sequence: 1, VisibilityEpoch: 1, Reason: "moved", Bounds: domain.Bounds{X: 999, W: 100, H: 100}}
	// Callback was admitted by source before pause, but final App lock happens later.
	event, ok := d.stampMediaPanelEvent(w.native, 1, event)
	if !ok {
		t.Fatal("fixture lease")
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() { close(entered); <-release; a.acceptMediaPanelEvent(event); close(done) }()
	<-entered
	d.hide()
	d.show(p.bounds) // Leave native UI queue unprocessed: hide/resume coalesces.
	if native.epoch != 2 {
		t.Fatal("hide failed to synchronously retire source admission")
	}
	close(release)
	<-done
	a.viewMu.Lock()
	x := p.bounds.X
	a.viewMu.Unlock()
	if x != 5 {
		t.Fatal("old callback replaced resumed geometry")
	}
	// Even stamping an already-admitted old callback after resume cannot resurrect it.
	event, _ = d.stampMediaPanelEvent(w.native, 1, event)
	a.acceptMediaPanelEvent(event)
	a.viewMu.Lock()
	x = p.bounds.X
	a.viewMu.Unlock()
	if x != 5 {
		t.Fatal("restamped stale native visibility admitted")
	}
}
