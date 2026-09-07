package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type launcherChildCore interface {
	CaptureChildParent(launcher.Scope, string) (launcher.ChildParent, error)
	ValidateChildParent(launcher.ChildParent) (launcher.Scope, error)
	PlaceChild(launcher.ChildParent, float64, float64) (domain.Bounds, error)
	SetChildBounds(launcher.ChildParent, uint64, domain.Bounds) error
	ClearChildBounds(launcher.ChildParent, uint64)
}
type appLauncherItemPanels struct {
	retained      int // Includes active, pending, and retiring owners until all receipts join.
	core          launcherChildCore
	next          uint64
	owners        map[uint64]*launcherItemPanel
	drains        map[uint64]<-chan struct{}
	folderFactory func() platform.FolderSource
	stopped       bool
}
type launcherItemPanel struct {
	windows       *launcherWindowChild
	parent        launcher.ChildParent
	parentHost    *appLauncherHost
	parentToken   uint64
	childToken    uint64
	manager       *appLauncherItems
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
	wg            sync.WaitGroup
	host          *dockWindow
	source        platform.FolderSource
	state         LauncherItemPanelState
	querySequence uint64
	queryRunning  bool
	actionBusy    bool
	predecessor   <-chan struct{}
}

func (a *App) wireLauncherItemPanels() {
	a.launcherItemPanels = &appLauncherItemPanels{core: a.launcher.core, owners: map[uint64]*launcherItemPanel{}, drains: map[uint64]<-chan struct{}{}, folderFactory: func() platform.FolderSource {
		return platform.NewFolderSource(filepath.Join(filepath.Dir(a.settingsPath), "folder-bookmarks.json"))
	}}
}

func (a *App) launcherChildCurrentLocked(p *launcherItemPanel) bool {
	r := a.launcherItemPanels
	if r == nil || r.stopped || r.owners[p.parent.Scope.Session] != p || p.ctx.Err() != nil || a.launcherItems != p.manager || p.manager.closed || p.manager.ctx.Err() != nil || !a.launcherAllowedLocked() || !a.launcher.ready || a.launcher.hosts[p.parent.Scope.Session] != p.parentHost {
		return false
	}
	if p.windows != nil && !a.launcherWindowSettingsCurrentLocked(p) {
		return false
	}
	_, err := r.core.ValidateChildParent(p.parent)
	return err == nil && launcherChildTokensCurrent(p)
}

func (a *App) launcherChildGuard(p *launcherItemPanel, requireChild bool) error {
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) {
		a.viewMu.Unlock()
		return launcher.ErrRetired
	}
	parentPanel := p.parentHost.window.wheelPanel()
	if panel, ok := parentPanel.(platform.LauncherPanel); !ok || panel.LauncherToken() != p.parentToken {
		a.viewMu.Unlock()
		return launcher.ErrRetired
	}
	var childPanel platform.DockPanel
	if p.host != nil {
		childPanel = p.host.wheelPanel()
		if requireChild {
			if panel, ok := childPanel.(platform.LauncherPanel); !ok || p.childToken == 0 || panel.LauncherToken() != p.childToken {
				a.viewMu.Unlock()
				return launcher.ErrRetired
			}
		}
	}
	a.viewMu.Unlock()
	for i, panel := range []platform.DockPanel{parentPanel, childPanel} {
		if i == 1 && !requireChild {
			continue
		}
		validator, ok := panel.(platform.LauncherPanelValidator)
		if !ok {
			return launcher.ErrUnavailable
		}
		if err := validator.ValidateLauncherPanel(p.ctx, p.parent.Scope.DisplayUUID); err != nil {
			return err
		}
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if !a.launcherChildCurrentLocked(p) {
		return launcher.ErrRetired
	}
	return nil
}

func (a *App) retireLauncherItemPanelLocked(parentSession uint64) <-chan struct{} {
	r := a.launcherItemPanels
	if r == nil {
		return nil
	}
	p := r.owners[parentSession]
	if p == nil {
		return r.drains[parentSession]
	}
	delete(r.owners, parentSession)
	captureDone := a.retireLauncherWindowCaptureLocked(p)
	p.cancel()
	r.core.ClearChildBounds(p.parent, p.state.Session)
	p.state.Open = false
	p.state.Revision++
	a.emit("launcher-item:hide", map[string]uint64{"session": p.state.Session, "revision": p.state.Revision})
	r.drains[parentSession] = p.done
	var hostDone <-chan struct{}
	if p.host != nil {
		hostDone = p.host.closeAndDrain()
	}
	go func() {
		p.wg.Wait()
		if captureDone != nil {
			<-captureDone
		}
		if p.predecessor != nil {
			<-p.predecessor
		}
		if hostDone != nil {
			<-hostDone
		}
		p.manager.wg.Done()
		close(p.done)
		a.viewMu.Lock()
		r.retained--
		if r.drains[parentSession] == p.done {
			delete(r.drains, parentSession)
		}
		a.viewMu.Unlock()
	}()
	return p.done
}

func (a *App) syncLauncherItemPanelsLocked() {
	if a.launcherItemPanels == nil {
		return
	}
	for id, p := range a.launcherItemPanels.owners {
		if !a.launcherChildCurrentLocked(p) {
			a.retireLauncherItemPanelLocked(id)
		}
	}
}

func (a *App) stopLauncherItemPanels() {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItemPanels == nil {
		return
	}
	a.launcherItemPanels.stopped = true
	for id := range a.launcherItemPanels.owners {
		a.retireLauncherItemPanelLocked(id)
	}
}

func (a *App) ShowLauncherItemPanel(epoch uint64, displayUUID string, parentSession, parentRevision uint64, itemID string) (LauncherItemPanelState, error) {
	a.viewMu.Lock()
	r, m := a.launcherItemPanels, a.launcherItems
	if r == nil || r.stopped || m == nil || m.closed || a.launcherItemPanelFactory == nil || !a.launcherAllowedLocked() || !a.launcher.ready {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrUnavailable
	}
	if r.retained >= 16 || (r.owners[parentSession] == nil && len(r.owners) >= 8) {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrBusy
	}
	if current := r.owners[parentSession]; current != nil {
		if !launcherChildReceiptDone(current.predecessor) {
			a.viewMu.Unlock()
			return LauncherItemPanelState{}, launcher.ErrBusy
		}
	} else if !launcherChildReceiptDone(r.drains[parentSession]) {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrBusy
	}
	parent, err := r.core.CaptureChildParent(launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: parentSession, Revision: parentRevision}, itemID)
	if err != nil {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, err
	}
	folder := parent.Configured.Kind == "folder"
	if (folder && (parent.Reference.State != "ready" || m.refs == nil)) || (!folder && (parent.App.Process.PID <= 0 || parent.App.Process.StartSeconds == 0 || parent.App.BundleID == "")) {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrUnavailable
	}
	h := a.launcher.hosts[parentSession]
	if h == nil {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrRetired
	}
	nativeParent, ok := h.window.wheelPanel().(platform.LauncherPanel)
	if !ok || nativeParent.LauncherToken() == 0 {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, launcher.ErrUnavailable
	}
	parentToken := nativeParent.LauncherToken()
	width := float64(420)
	if !folder {
		width = 560
	}
	bounds, err := r.core.PlaceChild(parent, width, 360)
	if err != nil {
		a.viewMu.Unlock()
		return LauncherItemPanelState{}, err
	}
	old := a.retireLauncherItemPanelLocked(parentSession)
	r.next++
	ctx, cancel := context.WithCancel(m.ctx)
	view := parent.Configured.FolderView
	if view != "grid" {
		view = "list"
	}
	order := platform.FolderSort{Field: "name", Direction: "asc", FoldersFirst: true}
	if a.dockState.Folder != nil {
		order = a.dockState.Folder.Sort
	}
	if order.Field == "" {
		order.Field = "name"
	}
	if order.Direction == "" {
		order.Direction = "asc"
	}
	p := &launcherItemPanel{parent: parent, parentHost: h, parentToken: parentToken, manager: m, ctx: ctx, cancel: cancel, done: make(chan struct{}), predecessor: old, state: LauncherItemPanelState{Session: r.next, Revision: 1, ParentEpoch: epoch, ParentSession: parentSession, DisplayUUID: displayUUID, ProfileID: parent.ProfileID, ItemID: itemID, Kind: "folder", Title: parent.Reference.Label, Open: true, Bounds: bounds, Folder: &LauncherItemFolderState{FolderIdentity: fmt.Sprintf("launcher-child:%d", r.next), Status: "loading", View: view, Entries: []platform.FolderEntry{}, Sort: order, Revision: 1}}}
	if !folder {
		a.initLauncherWindowChildLocked(p)
	}
	m.wg.Add(1)
	r.retained++
	r.owners[parentSession] = p
	p.wg.Add(1)
	if folder {
		go a.startLauncherFolderChild(p)
	} else {
		go a.startLauncherWindowChild(p)
	}
	result := cloneLauncherItemPanelState(p.state)
	a.viewMu.Unlock()
	return result, nil
}

func (a *App) childForCommandLocked(session, revision uint64) (*launcherItemPanel, error) {
	if a.launcherItemPanels != nil {
		for _, p := range a.launcherItemPanels.owners {
			if p.state.Session == session && p.state.Revision == revision && a.launcherChildCurrentLocked(p) {
				return p, nil
			}
		}
	}
	return nil, launcher.ErrRetired
}

func (a *App) GetLauncherItemPanelState(session uint64) *LauncherItemPanelState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItemPanels != nil {
		for _, p := range a.launcherItemPanels.owners {
			if p.state.Session == session && a.launcherChildCurrentLocked(p) {
				s := cloneLauncherItemPanelState(p.state)
				if p.windows != nil {
					s.Windows = a.launcherWindowSnapshotLocked(p)
				}
				return &s
			}
		}
	}
	return nil
}

func (a *App) CloseLauncherItemPanel(session, revision uint64) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		return err
	}
	a.retireLauncherItemPanelLocked(p.parent.Scope.Session)
	return nil
}

func (a *App) SetLauncherItemPanelSize(session, revision uint64, width, height int) error {
	if width < 120 || height < 96 || width > 960 || height > 720 {
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		return err
	}
	bounds, err := a.launcherItemPanels.core.PlaceChild(p.parent, float64(width), float64(height))
	if err != nil {
		return err
	}
	if bounds == p.state.Bounds {
		return nil
	}
	if err = a.launcherItemPanels.core.SetChildBounds(p.parent, session, bounds); err != nil {
		return err
	}
	p.state.Bounds = bounds
	if p.host != nil {
		p.host.show(bounds)
	}
	a.publishLauncherChildLocked(p)
	return nil
}

func (a *App) publishLauncherChildLocked(p *launcherItemPanel) {
	p.state.Revision++
	if p.state.Folder != nil {
		p.state.Folder.Revision = p.state.Revision
	}
	if p.windows != nil {
		a.publishLauncherWindowsLocked(p)
	}
	state := cloneLauncherItemPanelState(p.state)
	if p.windows != nil {
		state.Windows = a.launcherWindowSnapshotLocked(p)
	}
	a.emit("launcher-item:update", state)
}

func (a *App) prepareLauncherChildHost(p *launcherItemPanel) bool {
	if p.predecessor != nil {
		select {
		case <-p.predecessor:
		case <-p.ctx.Done():
			return false
		}
	}
	if err := a.launcherChildGuard(p, false); err != nil {
		a.failLauncherChildStart(p)
		return false
	}
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) {
		a.viewMu.Unlock()
		return false
	}
	factory := a.launcherItemPanelFactory
	appearance := p.parentHost.presentation.Appearance
	a.viewMu.Unlock()
	host := factory(p.state.Session, p.parent.Scope.DisplayUUID, platform.LauncherPanelStyle{Material: appearance.Material, Theme: appearance.Theme, CornerRadiusPx: appearance.CornerRadiusPx}, func() { a.failLauncherChildStart(p) })
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) || host == nil {
		if a.launcherItemPanels.owners[p.parent.Scope.Session] == p {
			a.retireLauncherItemPanelLocked(p.parent.Scope.Session)
		}
		a.viewMu.Unlock()
		if host != nil {
			<-host.closeAndDrain()
		}
		return false
	}
	p.host = host
	if err := a.launcherItemPanels.core.SetChildBounds(p.parent, p.state.Session, p.state.Bounds); err != nil {
		a.retireLauncherItemPanelLocked(p.parent.Scope.Session)
		a.viewMu.Unlock()
		return false
	}
	host.show(p.state.Bounds)
	a.publishLauncherChildLocked(p)
	a.viewMu.Unlock()
	ready := time.NewTimer(2 * time.Second)
	defer ready.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		a.viewMu.Lock()
		if !a.launcherChildCurrentLocked(p) {
			a.viewMu.Unlock()
			return false
		}
		panel, ok := host.wheelPanel().(platform.LauncherPanel)
		if ok && panel.LauncherToken() != 0 {
			p.childToken = panel.LauncherToken()
			a.viewMu.Unlock()
			break
		}
		a.viewMu.Unlock()
		select {
		case <-p.ctx.Done():
			return false
		case <-ready.C:
			a.failLauncherChildStart(p)
			return false
		case <-tick.C:
		}
	}
	if err := a.launcherChildGuard(p, true); err != nil {
		a.failLauncherChildStart(p)
		return false
	}
	return true
}

// LauncherToken is immutable Go-side identity, not a native visibility query.
// Recheck this under viewMu after each external validator so host recreation
// cannot be admitted while its asynchronous failure callback is still queued.
func launcherChildTokensCurrent(p *launcherItemPanel) bool {
	parent, ok := p.parentHost.window.wheelPanel().(platform.LauncherPanel)
	if !ok || p.parentToken == 0 || parent.LauncherToken() != p.parentToken {
		return false
	}
	if p.childToken != 0 {
		if p.host == nil {
			return false
		}
		child, ok := p.host.wheelPanel().(platform.LauncherPanel)
		if !ok || child.LauncherToken() != p.childToken {
			return false
		}
	}
	return true
}

func launcherChildReceiptDone(done <-chan struct{}) bool {
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	default:
		return false
	}
}
