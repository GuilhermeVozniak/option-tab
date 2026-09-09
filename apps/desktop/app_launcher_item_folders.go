package main

import (
	"context"
	"errors"

	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func (a *App) startLauncherFolderChild(p *launcherItemPanel) {
	defer p.wg.Done()
	if !a.prepareLauncherChildHost(p) {
		return
	}
	a.viewMu.Lock()
	if !a.launcherChildCurrentLocked(p) {
		a.viewMu.Unlock()
		return
	}
	factory := a.launcherItemPanels.folderFactory
	a.viewMu.Unlock()
	source := factory()
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if !a.launcherChildCurrentLocked(p) {
		return
	}
	if source == nil {
		a.retireLauncherItemPanelLocked(p.parent.Scope.Session)
		return
	}
	p.source = source
	a.queueLauncherFolderLocked(p)
}

func (a *App) failLauncherChildStart(p *launcherItemPanel) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItemPanels.owners[p.parent.Scope.Session] == p {
		a.retireLauncherItemPanelLocked(p.parent.Scope.Session)
	}
}

func (a *App) queueLauncherFolderLocked(p *launcherItemPanel) {
	p.querySequence++
	p.state.Folder.Status = "loading"
	p.state.Folder.Reason = ""
	p.state.Error = ""
	a.publishLauncherChildLocked(p)
	if p.queryRunning || p.source == nil {
		return
	}
	p.queryRunning = true
	p.wg.Add(1)
	go a.runLauncherFolderQueries(p)
}

func (a *App) runLauncherFolderQueries(p *launcherItemPanel) {
	defer p.wg.Done()
	for {
		a.viewMu.Lock()
		if !a.launcherChildCurrentLocked(p) {
			p.queryRunning = false
			a.viewMu.Unlock()
			return
		}
		seq, order := p.querySequence, p.state.Folder.Sort
		a.viewMu.Unlock()
		var listing platform.FolderListing
		err := a.launcherChildGuard(p, false)
		if err == nil {
			err = p.manager.refs.WithLauncherFolder(p.ctx, p.parent.Reference.ID, p.parent.Reference.Revision, func(ref platform.FolderRef) error {
				if err := a.launcherChildGuard(p, false); err != nil {
					return err
				}
				var listErr error
				listing, listErr = p.source.ListFolder(p.ctx, ref, order)
				return listErr
			})
		}
		if err == nil {
			err = a.launcherFolderReferenceGuard(p)
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
		f := p.state.Folder
		f.Entries = []platform.FolderEntry{}
		f.Partial = false
		f.Reason = ""
		if err != nil {
			f.Status = "unavailable"
			f.Reason = "folderUnavailable"
		} else {
			switch listing.Status {
			case "ready", "partial", "missing", "revoked", "permissionRequired", "unavailable":
				f.Status = listing.Status
			default:
				f.Status = "unavailable"
			}
			if len(listing.Entries) > 500 {
				listing.Entries = listing.Entries[:500]
				f.Status = "partial"
			}
			f.Entries = append([]platform.FolderEntry{}, listing.Entries...)
			f.Partial = f.Status == "partial"
			if f.Status != "ready" {
				f.Reason = "folderUnavailable"
			}
		}
		p.queryRunning = false
		a.publishLauncherChildLocked(p)
		a.viewMu.Unlock()
		return
	}
}

func (a *App) SetLauncherFolderSort(session, revision uint64, field, direction string, foldersFirst bool) error {
	if (field != "name" && field != "modified" && field != "size" && field != "kind") || (direction != "asc" && direction != "desc") {
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		return err
	}
	if p.state.Folder == nil {
		return launcher.ErrUnavailable
	}
	if p.actionBusy {
		return launcher.ErrBusy
	}
	p.state.Folder.Sort = platform.FolderSort{Field: field, Direction: direction, FoldersFirst: foldersFirst}
	a.queueLauncherFolderLocked(p)
	return nil
}

func (a *App) SetLauncherFolderView(session, revision uint64, view string) error {
	if view != "list" && view != "grid" {
		return launcher.ErrUnavailable
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		return err
	}
	if p.state.Folder == nil {
		return launcher.ErrUnavailable
	}
	if p.state.Folder.View != view {
		p.state.Folder.View = view
		a.publishLauncherChildLocked(p)
	}
	return nil
}

func (a *App) OpenLauncherFolderEntry(session, revision uint64, entryID string) error {
	a.viewMu.Lock()
	p, err := a.childForCommandLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	if p.state.Folder == nil {
		a.viewMu.Unlock()
		return launcher.ErrUnavailable
	}
	if p.actionBusy || p.queryRunning || p.source == nil {
		a.viewMu.Unlock()
		return launcher.ErrBusy
	}
	found := false
	for _, e := range p.state.Folder.Entries {
		if e.ID == entryID && entryID != "" {
			found = true
			break
		}
	}
	if !found {
		a.viewMu.Unlock()
		return launcher.ErrRetired
	}
	p.actionBusy = true
	p.wg.Add(1)
	a.viewMu.Unlock()
	defer p.wg.Done()
	guard := func() error {
		if err := a.launcherChildGuard(p, true); err != nil {
			return err
		}
		current, err := p.manager.refs.ResolveLauncherReference(p.ctx, p.parent.Reference.ID)
		if err != nil {
			return err
		}
		if current.ID != p.parent.Reference.ID || current.Revision != p.parent.Reference.Revision || current.Kind != "folder" || current.State != "ready" {
			return launcher.ErrRetired
		}
		if err = a.launcherChildGuard(p, true); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if p.state.Revision != revision {
			return launcher.ErrRetired
		}
		return nil
	}
	err = guard()
	if err == nil {
		err = p.manager.refs.WithLauncherFolder(p.ctx, p.parent.Reference.ID, p.parent.Reference.Revision, func(ref platform.FolderRef) error { return p.source.OpenFolderEntryGuarded(p.ctx, ref, entryID, guard) })
	}
	a.viewMu.Lock()
	p.actionBusy = false
	if a.launcherChildCurrentLocked(p) && err != nil && !errors.Is(err, context.Canceled) {
		p.state.Error = "folderOpenRefused"
		a.publishLauncherChildLocked(p)
	}
	a.viewMu.Unlock()
	return err
}

func (a *App) launcherFolderReferenceGuard(p *launcherItemPanel) error {
	if err := a.launcherChildGuard(p, true); err != nil {
		return err
	}
	current, err := p.manager.refs.ResolveLauncherReference(p.ctx, p.parent.Reference.ID)
	if err != nil {
		return err
	}
	if current.ID != p.parent.Reference.ID || current.Revision != p.parent.Reference.Revision || current.Kind != "folder" || current.State != "ready" {
		return launcher.ErrRetired
	}
	return a.launcherChildGuard(p, true)
}
