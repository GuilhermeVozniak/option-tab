package main

import (
	"context"
	"errors"
	"sync"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// dockWindow schedules native work away from its callers, including main-thread
// callbacks: Wails InvokeAsync itself runs inline when invoked on the main thread.
// At most one UI closure is outstanding. Requests coalesce to the latest revision.
type dockWindow struct {
	materialDelivery        *dockMaterialDelivery
	closeReceipt            chan struct{}
	closeReceiptDone        bool
	mu                      sync.Mutex
	dispatch                func(func())
	factory                 func() nativeWindow
	host                    platform.DockPanelHost
	revision                uint64
	visibility, nextHost    uint64
	queued, visible, closed bool
	bounds                  domain.Bounds
	current                 *dockWindowResources
	onFailure               func()
	material                *dockMaterialRequest
	materialCancel          context.CancelFunc
	materialRetire          platform.MaterialScope
}
type dockWindowResources struct {
	incarnation, mediaSequence uint64
	nativeHost                 unsafe.Pointer
	window                     *liveWindow
	panel                      platform.DockPanel
	material                   platform.MaterialSurface
	materialApplied            platform.MaterialScope
}

func newDockWindow(dispatch func(func()), factory func() nativeWindow, host platform.DockPanelHost) *dockWindow {
	return &dockWindow{dispatch: dispatch, factory: factory, host: host}
}

func (d *dockWindow) show(b domain.Bounds) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if !d.visible {
		d.visibility++
	}
	d.visible = true
	d.bounds = b
	d.revision++
	d.scheduleLocked()
}

func (d *dockWindow) hide() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.visible = false
	d.clearMaterialLocked()
	d.visibility++
	d.retireMediaEventsLocked()
	d.revision++
	d.scheduleLocked()
}

func (d *dockWindow) close() { _ = d.closeAndDrain() }

// closeAndDrain returns promptly. Its receipt covers scheduled resource cleanup,
// including Panel.Close and Wails Close returning, not arbitrary OS destruction
// notifications or independent material event delivery.
func (d *dockWindow) closeAndDrain() <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closeReceipt == nil {
		d.closeReceipt = make(chan struct{})
	}
	if !d.closed {
		d.closed = true
		if d.materialDelivery != nil {
			d.materialDelivery.close()
		}
		d.visible = false
		d.clearMaterialLocked()
		d.visibility++
		d.retireMediaEventsLocked()
		d.revision++
		d.scheduleLocked()
	}
	return d.closeReceipt
}

func (d *dockWindow) scheduleLocked() {
	if d.queued {
		return
	}
	d.queued = true
	go d.dispatch(d.reconcile)
}

// The callback identifies the exact Wails host. A delayed notification from an
// already retired host cannot invalidate a replacement or create another panel.
func (d *dockWindow) markHostClosedIf(w nativeWindow) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current == nil || !d.current.window.wraps(w) || !d.current.window.alive() {
		return false
	}
	d.current.window.markClosed()
	d.refreshMaterialLocked()
	d.revision++
	d.scheduleLocked()
	return true
}

func (d *dockWindow) valid(revision uint64) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.revision == revision && !d.closed && d.visible
}

func (d *dockWindow) reconcile() {
	d.mu.Lock()
	revision, visible, closed, b, r := d.revision, d.visible, d.closed, d.bounds, d.current
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.queued = false
		if d.revision != revision {
			d.scheduleLocked()
		} else if closed && d.closed && d.current == nil && d.closeReceipt != nil && !d.closeReceiptDone {
			// Only a completed terminal reconcile can issue the receipt. A
			// close arriving during factory/Show still requires its queued
			// terminal pass, and callback revision changes require another pass.
			d.closeReceiptDone = true
			close(d.closeReceipt)
		}
		d.mu.Unlock()
	}()
	if r != nil && (!r.window.alive() || closed) {
		d.dispose(r)
		r = nil
	}
	if closed {
		return
	}
	if !visible {
		if r != nil {
			d.reconcileMaterial(r, false, revision)
			if err := r.panel.Hide(); err != nil {
				d.failed(r, err)
			}
		}
		return
	}
	if !d.valid(revision) {
		return
	}
	if r == nil {
		w := d.factory()
		if w == nil {
			return
		}
		r = &dockWindowResources{window: newLiveWindow(w)}
		d.mu.Lock()
		d.nextHost++
		r.incarnation = d.nextHost
		d.current = r
		d.mu.Unlock()
		if !d.valid(revision) {
			d.dispose(r)
			return
		}
		// Publish the live wrapper before invoking native code, which can synchronously
		// announce host closure. Neither the factory nor AppKit executes under mu.
		nativeHost := r.window.native()
		d.mu.Lock()
		r.nativeHost = nativeHost
		d.mu.Unlock()
		panel, err := d.host.CreateDockPanel(nativeHost)
		d.mu.Lock()
		if d.current == r {
			r.panel = panel
		}
		d.mu.Unlock()
		if err != nil {
			d.failed(r, err)
			return
		}
		if panel == nil {
			d.dispose(r)
			return
		}
	}
	if !d.valid(revision) || !r.window.alive() {
		return
	}
	if err := r.panel.Show(b); err != nil {
		d.failed(r, err)
		return
	}
	d.reconcileMaterial(r, true, revision)
}

func (d *dockWindow) failed(r *dockWindowResources, err error) {
	if errors.Is(err, platform.ErrDockPanelHostClosed) {
		r.window.markClosed()
	}
	dlog("dock panel: %v", err)
	d.dispose(r)
	if d.onFailure != nil {
		d.onFailure()
	}
}

func (d *dockWindow) dispose(r *dockWindowResources) {
	if r.material != nil {
		_ = r.material.Close()
		r.material = nil
	}
	// Panel Close restores the Wails content view before the host is destroyed.
	// Keep the wrapper published during native callbacks so host loss is recorded.
	if r.panel != nil {
		if err := r.panel.Close(); err != nil {
			if errors.Is(err, platform.ErrDockPanelHostClosed) {
				r.window.markClosed()
			}
			dlog("close dock panel: %v", err)
		}
	}
	w := r.window.get()
	r.window.markClosed()
	d.mu.Lock()
	if d.current == r {
		d.current = nil
	}
	d.mu.Unlock()
	if closer, ok := w.(interface{ Close() }); ok {
		closer.Close()
	}
}

func (d *dockWindow) wheelPanel() platform.DockPanel {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.current == nil || !d.current.window.alive() {
		return nil
	}
	return d.current.panel
}

func (d *dockWindow) SetDockPanelWheelPolicy(policy platform.DockPanelWheelPolicy, emit func(platform.DockPanelWheelEvent)) error {
	panel, ok := d.wheelPanel().(platform.DockPanelWheelSource)
	if !ok {
		return platform.ErrDockPanelHostClosed
	}
	return panel.SetDockPanelWheelPolicy(policy, emit)
}

func (d *dockWindow) ValidateDockPanelWheelGesture(session, revision, gesture uint64) bool {
	panel, ok := d.wheelPanel().(platform.DockPanelWheelGestureValidator)
	return ok && panel.ValidateDockPanelWheelGesture(session, revision, gesture)
}

func (d *dockWindow) CompleteDockPanelWheelGesture(session, revision, gesture uint64) {
	if panel, ok := d.wheelPanel().(platform.DockPanelWheelGestureAcknowledger); ok {
		panel.CompleteDockPanelWheelGesture(session, revision, gesture)
	}
}

// Native media capability methods below are strictly Go-only and never call UI.
func (d *dockWindow) retireMediaEventsLocked() {
	if d.current != nil {
		if source, ok := d.current.panel.(platform.MediaPanelEventRetirer); ok {
			source.RetireMediaPanelEvents()
		}
	}
}

func (d *dockWindow) stampMediaPanelEvent(host unsafe.Pointer, incarnation uint64, event platform.MediaPanelEvent) (platform.MediaPanelEvent, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.current
	if d.closed || r == nil || r.nativeHost != host || r.incarnation != incarnation {
		return event, false
	}
	event.HostIncarnation = r.incarnation
	event.HostVisibilityEpoch = d.visibility
	return event, true
}

func (d *dockWindow) admitMediaPanelEvent(event platform.MediaPanelEvent) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.current
	if d.closed || r == nil || event.HostIncarnation == 0 || r.incarnation != event.HostIncarnation || event.Sequence <= r.mediaSequence {
		return false
	}
	if event.Reason != "hostClosed" && (!d.visible || event.HostVisibilityEpoch != d.visibility) {
		return false
	}
	validator, ok := r.panel.(platform.MediaPanelEventValidator)
	if !ok || !validator.MediaPanelEventCurrent(event) {
		return false
	}
	r.mediaSequence = event.Sequence
	return true
}

func (d *dockWindow) ownsMediaWindow(window nativeWindow) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.closed && d.current != nil && d.current.window.wraps(window)
}

func (d *dockWindow) mediaHostIncarnation(host unsafe.Pointer) uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || d.current == nil || d.current.nativeHost != host {
		return 0
	}
	return d.current.incarnation
}
