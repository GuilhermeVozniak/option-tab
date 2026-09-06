package main

import (
	"errors"
	"sync"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// dockWindow schedules native work away from its callers, including main-thread
// callbacks: Wails InvokeAsync itself runs inline when invoked on the main thread.
// At most one UI closure is outstanding. Requests coalesce to the latest revision.
type dockWindow struct {
	mu                      sync.Mutex
	dispatch                func(func())
	factory                 func() nativeWindow
	host                    platform.DockPanelHost
	revision                uint64
	queued, visible, closed bool
	bounds                  domain.Bounds
	current                 *dockWindowResources
}
type dockWindowResources struct {
	window *liveWindow
	panel  platform.DockPanel
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
	d.revision++
	d.scheduleLocked()
}

func (d *dockWindow) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	d.closed = true
	d.visible = false
	d.revision++
	d.scheduleLocked()
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
func (d *dockWindow) markHostClosedIf(w nativeWindow) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.current == nil || !d.current.window.wraps(w) || !d.current.window.alive() {
		return
	}
	d.current.window.markClosed()
	d.revision++
	d.scheduleLocked()
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
		d.current = r
		d.mu.Unlock()
		if !d.valid(revision) {
			d.dispose(r)
			return
		}
		// Publish the live wrapper before invoking native code, which can synchronously
		// announce host closure. Neither the factory nor AppKit executes under mu.
		panel, err := d.host.CreateDockPanel(r.window.native())
		r.panel = panel
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
	}
}

func (d *dockWindow) failed(r *dockWindowResources, err error) {
	if errors.Is(err, platform.ErrDockPanelHostClosed) {
		r.window.markClosed()
	}
	dlog("dock panel: %v", err)
	d.dispose(r)
}

func (d *dockWindow) dispose(r *dockWindowResources) {
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
