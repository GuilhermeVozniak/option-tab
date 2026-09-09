package main

import (
	"testing"
	"time"

	"option-tab/internal/platform"
)

func dockDrainOpen(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("drain closed before final UI cleanup")
	default:
	}
}

func dockDrainDone(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("drain never closed")
	}
}

func TestDockWindowDrainWaitsForQueuedTerminalReconcile(t *testing.T) {
	d, q, _, ws := dockHarness()
	d.show(dockTestBounds)
	receipt := d.closeAndDrain()
	if receipt != d.closeAndDrain() {
		t.Fatal("unstable receipt")
	}
	dockDrainOpen(t, receipt)
	q.next(t)
	dockDrainDone(t, receipt)
	if len(*ws) != 0 {
		t.Fatal("closed window created resources")
	}
	d.show(dockTestBounds)
}

func TestDockWindowDrainWaitsForBlockedFactoryAndFinalReconcile(t *testing.T) {
	d, q, _, ws := dockHarness()
	factory := d.factory
	entered, release := make(chan struct{}), make(chan struct{})
	d.factory = func() nativeWindow { close(entered); <-release; return factory() }
	d.show(dockTestBounds)
	uiDone := make(chan struct{})
	go func() { q.next(t); close(uiDone) }()
	<-entered
	receipt := d.closeAndDrain()
	dockDrainOpen(t, receipt)
	close(release)
	<-uiDone
	dockDrainOpen(t, receipt)
	if len(*ws) != 1 || (*ws)[0].closes != 1 {
		t.Fatal("factory result not disposed")
	}
	q.next(t)
	dockDrainDone(t, receipt)
}

type drainDockPanel struct {
	platform.DockPanel
	closeFn func() error
}

func (p *drainDockPanel) Close() error { return p.closeFn() }
func TestDockWindowDrainJoinsPanelAndWindowClose(t *testing.T) {
	d, q, h, ws := dockHarness()
	d.show(dockTestBounds)
	q.next(t)
	panelEntered, panelRelease, windowEntered, windowRelease := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	original := h.panels[0]
	d.mu.Lock()
	d.current.panel = &drainDockPanel{DockPanel: original, closeFn: func() error { close(panelEntered); <-panelRelease; return original.Close() }}
	d.mu.Unlock()
	(*ws)[0].onClose = func() { close(windowEntered); <-windowRelease }
	receipt := d.closeAndDrain()
	uiDone := make(chan struct{})
	go func() { q.next(t); close(uiDone) }()
	<-panelEntered
	dockDrainOpen(t, receipt)
	close(panelRelease)
	<-windowEntered
	dockDrainOpen(t, receipt)
	close(windowRelease)
	<-uiDone
	dockDrainDone(t, receipt)
}

func TestDockWindowDrainCloseCallbackRevisionRequiresFinalReconcile(t *testing.T) {
	d, q, h, ws := dockHarness()
	d.show(dockTestBounds)
	q.next(t)
	w := (*ws)[0]
	original := h.panels[0]
	d.mu.Lock()
	d.current.panel = &drainDockPanel{DockPanel: original, closeFn: func() error {
		if !d.markHostClosedIf(w) {
			t.Error("live callback refused")
		}
		return original.Close()
	}}
	d.mu.Unlock()
	receipt := d.closeAndDrain()
	q.next(t)
	dockDrainOpen(t, receipt)
	if d.markHostClosedIf(w) {
		t.Fatal("old callback admitted")
	}
	q.next(t)
	dockDrainDone(t, receipt)
}
