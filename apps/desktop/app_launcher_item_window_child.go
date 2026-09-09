package main

import (
	"reflect"
	"sync"
	"sync/atomic"

	"option-tab/internal/actions"
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
	"option-tab/internal/preview"
	"option-tab/internal/switcher"
)

// Each window child owns its capture peer, copied identities and immutable frame
// admission. Frame callbacks never take viewMu or consult native APIs.
type launcherWindowChild struct {
	settings   config.Settings
	identities map[domain.WindowID]platform.AutomationWindowIdentity
	peer       *preview.Manager
	capture    bool
	lease      atomic.Pointer[automationPreviewFrameLease]
	frameMu    sync.Mutex
	frames     map[domain.WindowID]string
	sequence   uint64
}

func cloneLauncherWindowState(input *AutomationPreviewViewState) *AutomationPreviewViewState {
	if input == nil {
		return nil
	}
	s := *input
	s.Entries = append([]switcher.Entry{}, input.Entries...)
	s.Frames = make(map[domain.WindowID]string, len(input.Frames))
	for id, url := range input.Frames {
		s.Frames[id] = url
	}
	return &s
}

func (a *App) initLauncherWindowChildLocked(p *launcherItemPanel) {
	s := a.settingsSnapshot()
	p.windows = &launcherWindowChild{settings: s, identities: map[domain.WindowID]platform.AutomationWindowIdentity{}, frames: map[domain.WindowID]string{}}
	p.state.Kind = "windows"
	p.state.Folder = nil
	p.state.Title = p.parent.App.Name
	p.state.Windows = &AutomationPreviewViewState{Session: p.state.Session, Revision: p.state.Revision, Open: true, Title: p.state.Title, Entries: []switcher.Entry{}, Frames: map[domain.WindowID]string{}, Appearance: s.Appearance, CardSpacingPx: 7}
}

func (a *App) launcherWindowSettingsCurrentLocked(p *launcherItemPanel) bool {
	return p.windows != nil && reflect.DeepEqual(p.windows.settings, a.settingsSnapshot())
}

func (a *App) startLauncherWindowChild(p *launcherItemPanel) {
	defer p.wg.Done()
	if !a.prepareLauncherChildHost(p) {
		return
	}
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) {
		a.viewMu.Unlock()
		return
	}
	base := a.captures
	appearance := p.windows.settings.Appearance
	a.viewMu.Unlock()
	capture := (appearance.Style == config.StyleThumbnails || appearance.PreviewSelected) && a.platform.ScreenRecording() == platform.PermGranted
	var peer *preview.Manager
	if base != nil {
		peer = base.NewPeerWithIdentity(func(id domain.WindowID, url string, identity platform.AutomationWindowIdentity) {
			a.emitLauncherWindowFrame(p, id, url, identity)
		})
	}
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) {
		a.viewMu.Unlock()
		if peer != nil {
			<-peer.CloseAndDrain()
		}
		return
	}
	p.windows.peer = peer
	p.windows.capture = capture
	a.queueLauncherWindowsLocked(p)
	a.viewMu.Unlock()
}

func (a *App) retireLauncherWindowCaptureLocked(p *launcherItemPanel) <-chan struct{} {
	if p.windows == nil {
		return nil
	}
	w := p.windows
	w.lease.Store(nil)
	w.frameMu.Lock()
	w.frames = nil
	w.frameMu.Unlock()
	if w.peer != nil {
		return w.peer.CloseAndDrain()
	}
	return nil
}

func launcherWindowCaptureIDs(p *launcherItemPanel) []domain.WindowID {
	out := make([]domain.WindowID, 0, 30)
	state := p.state.Windows
	if state == nil {
		return out
	}
	if state.SelectedWindowID != 0 {
		out = append(out, state.SelectedWindowID)
	}
	for _, entry := range state.Entries {
		if len(out) >= 30 {
			break
		}
		if entry.WindowID != state.SelectedWindowID {
			out = append(out, entry.WindowID)
		}
	}
	return out
}

func (a *App) publishLauncherWindowsLocked(p *launcherItemPanel) {
	w := p.windows
	if w == nil || p.state.Windows == nil {
		return
	}
	p.state.Windows.Session = p.state.Session
	p.state.Windows.Revision = p.state.Revision
	p.state.Windows.Open = p.state.Open
	p.state.Windows.Error = p.state.Error
	if !w.capture || p.ctx.Err() != nil || len(w.identities) == 0 {
		w.lease.Store(nil)
		return
	}
	ids := map[domain.WindowID]platform.AutomationWindowIdentity{}
	for _, id := range launcherWindowCaptureIDs(p) {
		if identity, ok := w.identities[id]; ok {
			ids[id] = identity
		}
	}
	w.lease.Store(&automationPreviewFrameLease{session: p.state.Session, revision: p.state.Revision, identities: ids})
	w.frameMu.Lock()
	for id := range w.frames {
		if _, ok := ids[id]; !ok {
			delete(w.frames, id)
		}
	}
	w.frameMu.Unlock()
}

func (a *App) launcherWindowSnapshotLocked(p *launcherItemPanel) *AutomationPreviewViewState {
	state := cloneLauncherWindowState(p.state.Windows)
	if state == nil || p.windows == nil {
		return state
	}
	w := p.windows
	w.frameMu.Lock()
	defer w.frameMu.Unlock()
	lease := w.lease.Load()
	state.Frames = map[domain.WindowID]string{}
	if lease == nil || lease.session != state.Session || lease.revision != state.Revision {
		return state
	}
	state.FrameSequence = w.sequence
	for id, url := range w.frames {
		if _, ok := lease.identities[id]; ok {
			state.Frames[id] = url
		}
	}
	return state
}

func (a *App) emitLauncherWindowFrame(p *launcherItemPanel, id domain.WindowID, url string, identity platform.AutomationWindowIdentity) {
	w := p.windows
	if w == nil || len(url) > 512*1024 {
		return
	}
	lease := w.lease.Load()
	if lease == nil {
		return
	}
	expected, ok := lease.identities[id]
	if !ok || identity != expected {
		return
	}
	w.frameMu.Lock()
	defer w.frameMu.Unlock()
	if w.lease.Load() != lease {
		return
	}
	total := len(url)
	for other, value := range w.frames {
		if other != id {
			total += len(value)
		}
	}
	if total > 4*1024*1024 {
		return
	}
	if _, cached := w.frames[id]; !cached && len(w.frames) >= 30 {
		return
	}
	if w.frames == nil {
		w.frames = map[domain.WindowID]string{}
	}
	if url == "" {
		delete(w.frames, id)
	} else {
		w.frames[id] = url
	}
	w.sequence++
	a.emit("launcher-item:frames", automationPreviewFrames{Session: lease.session, Revision: lease.revision, Sequence: w.sequence, Frames: map[domain.WindowID]string{id: url}})
}

func (a *App) queueLauncherWindowsLocked(p *launcherItemPanel) {
	p.querySequence++
	if p.queryRunning {
		return
	}
	p.queryRunning = true
	p.wg.Add(1)
	go a.runLauncherWindowQueries(p)
}

func (a *App) runLauncherWindowQueries(p *launcherItemPanel) {
	defer p.wg.Done()
	for {
		a.viewMu.Lock()
		if !a.launcherChildCurrentLocked(p) {
			p.queryRunning = false
			a.viewMu.Unlock()
			return
		}
		seq := p.querySequence
		settings := p.windows.settings
		a.viewMu.Unlock()
		err := a.launcherWindowParentGuard(p)
		var entries []switcher.Entry
		var identities map[domain.WindowID]platform.AutomationWindowIdentity
		if err == nil {
			var windows []domain.Window
			windows, err = a.platform.Windows()
			if err == nil {
				entries, identities, err = a.prepareAutomationPreview(p.ctx, p.parent.App.Process, windows, settings)
			}
		}
		if err == nil {
			err = a.launcherWindowParentGuard(p)
		}
		a.viewMu.Lock()
		if !a.launcherChildCurrentLocked(p) {
			p.queryRunning = false
			a.viewMu.Unlock()
			return
		}
		if seq != p.querySequence {
			a.viewMu.Unlock()
			continue
		}
		if err != nil {
			p.windows.lease.Store(nil)
			p.windows.identities = map[domain.WindowID]platform.AutomationWindowIdentity{}
			p.windows.frameMu.Lock()
			p.windows.frames = map[domain.WindowID]string{}
			p.windows.frameMu.Unlock()
			p.state.Windows.Entries = []switcher.Entry{}
			p.state.Windows.SelectedWindowID = 0
			peer := p.windows.peer
			a.viewMu.Unlock()
			if peer != nil {
				peer.Hide()
			}
			a.viewMu.Lock()
			if !a.launcherChildCurrentLocked(p) {
				p.queryRunning = false
				a.viewMu.Unlock()
				return
			}
			if seq != p.querySequence {
				a.viewMu.Unlock()
				continue
			}
			p.queryRunning = false
			p.state.Error = "Window preview unavailable"
			a.publishLauncherChildLocked(p)
			a.viewMu.Unlock()
			return
		}
		w := p.windows
		w.lease.Store(nil)
		peer := w.peer
		a.viewMu.Unlock()
		// Retire old identity captures before publishing replacement window entries,
		// including WindowServer numeric ID reuse. This owner never hides a sibling.
		if peer != nil {
			peer.Hide()
		}
		a.viewMu.Lock()
		if !a.launcherChildCurrentLocked(p) {
			p.queryRunning = false
			a.viewMu.Unlock()
			return
		}
		if seq != p.querySequence {
			a.viewMu.Unlock()
			continue
		}
		w.identities = identities
		w.frameMu.Lock()
		w.frames = map[domain.WindowID]string{}
		w.frameMu.Unlock()
		state := p.state.Windows
		state.Entries = entries
		if _, ok := identities[state.SelectedWindowID]; !ok {
			state.SelectedWindowID = 0
			if len(entries) > 0 {
				state.SelectedWindowID = entries[0].WindowID
			}
		}
		state.EmptyReason = ""
		if len(entries) == 0 {
			state.EmptyReason = "No windows match the current filters"
		}
		p.state.Error = ""
		p.queryRunning = false
		a.publishLauncherChildLocked(p)
		a.updateLauncherWindowCaptureLocked(p)
		a.viewMu.Unlock()
		return
	}
}

func (a *App) updateLauncherWindowCaptureLocked(p *launcherItemPanel) {
	w := p.windows
	if w.peer != nil && w.capture {
		lease := w.lease.Load()
		if lease == nil {
			return
		}
		w.peer.UpdateBound(launcherWindowCaptureIDs(p), p.state.Windows.SelectedWindowID, p.state.Windows.Appearance.ThumbnailMaxPx, lease.identities)
	}
}

func (a *App) launcherWindowParentGuard(p *launcherItemPanel) error {
	if err := a.launcherChildGuard(p, true); err != nil {
		return err
	}
	source, ok := a.platform.(platform.AutomationIdentitySource)
	if !ok {
		return launcher.ErrUnavailable
	}
	process, err := source.ProcessIdentity(p.parent.App.Process.PID)
	if err != nil || process != p.parent.App.Process {
		return launcher.ErrRetired
	}
	if p.parent.Configured.ID != "" {
		ref, err := p.manager.refs.ResolveLauncherReference(p.ctx, p.parent.Configured.ReferenceID)
		if err != nil {
			return err
		}
		if ref != p.parent.Reference {
			return launcher.ErrRetired
		}
	}
	if err = a.launcherChildGuard(p, true); err != nil {
		return err
	}
	return p.ctx.Err()
}

func (a *App) SelectLauncherWindow(session, revision uint64, id domain.WindowID) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		return err
	}
	if p.windows == nil || p.state.Windows == nil {
		return launcher.ErrUnavailable
	}
	if _, ok := p.windows.identities[id]; !ok {
		return launcher.ErrUnavailable
	}
	p.state.Windows.SelectedWindowID = id
	a.publishLauncherChildLocked(p)
	a.updateLauncherWindowCaptureLocked(p)
	return nil
}

func (a *App) launcherWindowActionGuard(p *launcherItemPanel, revision uint64, id platform.AutomationWindowIdentity) error {
	if err := a.launcherWindowParentGuard(p); err != nil {
		return err
	}
	source, ok := a.platform.(platform.AutomationIdentitySource)
	if !ok || !source.WindowIdentityCurrent(id) {
		return launcher.ErrRetired
	}
	// The exact AX identity read can block while the physical panel/Space retires.
	if err := a.launcherChildGuard(p, true); err != nil {
		return err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	current, err := a.childForCommandLocked(p.state.Session, revision)
	if err != nil || current != p || p.windows.identities[id.ID] != id {
		return launcher.ErrRetired
	}
	return nil
}

func (a *App) PerformLauncherWindowAction(session, revision uint64, kind string, id domain.WindowID, fullscreen bool) error {
	switch kind {
	case "focus", "close", "minimize", "hide", "fullscreen":
	default:
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	if p.windows == nil || p.queryRunning {
		a.viewMu.Unlock()
		return launcher.ErrUnavailable
	}
	identity, ok := p.windows.identities[id]
	if !ok {
		a.viewMu.Unlock()
		return launcher.ErrUnavailable
	}
	if p.actionBusy {
		a.viewMu.Unlock()
		return launcher.ErrBusy
	}
	p.actionBusy = true
	p.wg.Add(1)
	a.viewMu.Unlock()
	defer func() { a.viewMu.Lock(); p.actionBusy = false; a.viewMu.Unlock(); p.wg.Done() }()
	var value *bool
	if kind == "fullscreen" {
		value = &fullscreen
	}
	guard := func() error { return a.launcherWindowActionGuard(p, revision, identity) }
	err = actions.New(a.platform).PerformAutomationWindowAction(p.ctx, kind, identity, value, guard)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	current, scopeErr := a.childForCommandLocked(session, revision)
	if scopeErr != nil || current != p {
		return launcher.ErrRetired
	}
	if err != nil {
		p.state.Error = "Window action failed"
		a.publishLauncherChildLocked(p)
		return err
	}
	a.queueLauncherWindowsLocked(p)
	return nil
}
