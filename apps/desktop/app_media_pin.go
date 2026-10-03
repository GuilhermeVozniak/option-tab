package main

import (
	"errors"
	"math"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/media"
	"option-tab/internal/platform"
)

// The Wails factory creates only a scheduler here. Its native webview and panel
// are constructed later on the UI queue, outside App.viewMu.
func (a *App) PinMediaPanel(session, revision uint64) (uint64, error) {
	a.viewMu.Lock()
	source, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return 0, err
	}
	if a.mediaPinFactory == nil {
		a.viewMu.Unlock()
		return 0, errors.New("pinned media panels are unavailable")
	}
	provider := source.state.Provider
	for _, p := range a.media.panels {
		if p.state.Pinned && p.state.Provider == provider {
			if p.host != nil {
				p.host.show(p.bounds)
			}
			a.viewMu.Unlock()
			return p.state.Session, nil
		}
	}
	screenID := a.dockState.Item.ScreenID
	a.viewMu.Unlock()
	// Screens can enter AppKit; never query them while holding presentation locks.
	screens := a.platform.Screens()
	screen := domain.Screen{}
	for _, candidate := range screens {
		if candidate.Main {
			screen = candidate
		}
		if candidate.ID == screenID {
			screen = candidate
			break
		}
	}
	if !validMediaPanelSize(screen.Visible.W, screen.Visible.H) {
		return 0, errors.New("media panel display is unavailable")
	}
	w, h := math.Min(420, screen.Visible.W), math.Min(460, screen.Visible.H)
	bounds := domain.Bounds{X: screen.Visible.X + (screen.Visible.W-w)/2, Y: screen.Visible.Y + (screen.Visible.H-h)/2, W: w, H: h}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.mediaPresentationLocked(session) != source || !a.mediaPanelAllowedLocked(source) {
		return 0, media.ErrRetired
	}
	// Another accepted request may have completed its display query first.
	for _, p := range a.media.panels {
		if p.state.Pinned && p.state.Provider == provider {
			if p.host != nil {
				p.host.show(p.bounds)
			}
			return p.state.Session, nil
		}
	}
	p := a.newMediaPresentationLocked(provider, true)
	p.bounds = bounds
	p.host = a.mediaPinFactory(p.state.Session, provider, a.acceptMediaPanelEvent)
	if p.host == nil {
		a.retireMediaPresentationLocked(p.state.Session)
		return 0, errors.New("media panel host is unavailable")
	}
	p.host.show(bounds)
	return p.state.Session, nil
}

func (a *App) CloseMediaPanel(session, revision uint64) error {
	a.viewMu.Lock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	dockSession := uint64(0)
	if p.state.Pinned {
		a.retireMediaPresentationLocked(session)
	} else {
		dockSession = a.dockState.Session
		a.dismissDockLocked()
	}
	a.viewMu.Unlock()
	if dockSession != 0 && a.dockController != nil {
		a.dockController.Dismiss(dockSession)
	}
	return nil
}

func validMediaPanelSize(w, h float64) bool {
	return w > 0 && h > 0 && !math.IsNaN(w) && !math.IsNaN(h) && !math.IsInf(w, 0) && !math.IsInf(h, 0)
}

func (a *App) SetMediaPanelSize(session, revision uint64, width, height float64) error {
	if !validMediaPanelSize(width, height) || width > 4096 || height > 4096 {
		return platform.ErrDockPanelBounds
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		return err
	}
	// The outer Dock content owns hover sizing, including its border and padding.
	if !p.state.Pinned {
		return nil
	}
	if p.bounds.W == width && p.bounds.H == height {
		return nil
	}
	p.bounds.W = width
	p.bounds.H = height
	if p.host != nil {
		p.host.show(p.bounds)
	}
	return nil
}

func (a *App) acceptMediaPanelEvent(event platform.MediaPanelEvent) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p := a.mediaPresentationLocked(event.Session)
	if p == nil || !p.state.Pinned || event.Sequence == 0 || p.host == nil || !p.host.admitMediaPanelEvent(event) {
		return
	}
	if event.Reason == "hostClosed" {
		a.retireMediaPresentationLocked(event.Session)
		return
	}
	if !p.state.Open || !a.mediaAllowedLocked(p.state.Provider) || !validMediaPanelSize(event.Bounds.W, event.Bounds.H) || math.IsNaN(event.Bounds.X) || math.IsNaN(event.Bounds.Y) || math.IsInf(event.Bounds.X, 0) || math.IsInf(event.Bounds.Y, 0) {
		return
	}
	p.nativeSequence = event.Sequence
	p.bounds = event.Bounds
	p.displayUUID = event.DisplayUUID
	if event.Reason == "topology" {
		p.state.InteractionEpoch++
		a.publishMediaLocked(p)
	}
}

func (a *App) mediaPinHostClosed(session uint64, scheduler *dockWindow, window nativeWindow) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p := a.mediaPresentationLocked(session)
	if p != nil && p.state.Pinned && p.host == scheduler && scheduler.ownsMediaWindow(window) {
		a.retireMediaPresentationLocked(session)
	}
}

type mediaPinHostAdapter struct {
	owner   *dockWindow
	source  platform.MediaPanelHost
	session uint64
	emit    func(platform.MediaPanelEvent)
}

func (h *mediaPinHostAdapter) CreateDockPanel(host unsafe.Pointer) (platform.DockPanel, error) {
	incarnation := h.owner.mediaHostIncarnation(host)
	return h.source.CreateMediaPanel(host, h.session, func(event platform.MediaPanelEvent) {
		if h.owner == nil {
			return
		}
		stamped, ok := h.owner.stampMediaPanelEvent(host, incarnation, event)
		if ok {
			h.emit(stamped)
		}
	})
}
