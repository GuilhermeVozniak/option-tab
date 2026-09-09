package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"option-tab/internal/dock"
	"option-tab/internal/platform"
)

type dockFolderScope struct {
	session, revision, folderRevision, admission uint64
	ref                                          platform.FolderRef
}

// An accepted chooser belongs to its captured folder independently of hover.
// Keep the owner until the source returns after native cleanup; another chooser
// cannot overlap a cancelled owner that is still draining.
type dockFolderGrantOwner struct {
	scope  dockFolderScope
	cancel context.CancelFunc
}

func cloneDockFolder(folder *dock.FolderState) *dock.FolderState {
	if folder == nil {
		return nil
	}
	copy := *folder
	copy.Entries = slices.Clone(folder.Entries)
	return &copy
}

func (a *App) dockFolderScopeLocked(session, revision uint64) (dockFolderScope, error) {
	st := a.dockState
	if session == 0 || revision == 0 || session != st.Session || revision != a.dockViewState.Revision || !a.dockItemAllowedLocked(st.Item) || st.Item.Kind != "folder" || st.ContentKind != "folder" || st.Folder == nil {
		return dockFolderScope{}, errStaleDockSession
	}
	if st.AdmissionEpoch != 0 && a.dockController != nil && st.AdmissionEpoch != a.dockController.AdmissionEpoch() {
		return dockFolderScope{}, errStaleDockSession
	}
	id, err := platform.FolderIdentity(st.Item.Path)
	if err != nil || id != st.Folder.FolderIdentity {
		return dockFolderScope{}, errors.New("folder identity is no longer available")
	}
	return dockFolderScope{session: session, revision: revision, folderRevision: st.Folder.Revision, admission: st.AdmissionEpoch, ref: platform.FolderRef{Identity: id, Path: st.Item.Path}}, nil
}

func (a *App) currentDockFolderLocked(scope dockFolderScope) bool {
	current, err := a.dockFolderScopeLocked(scope.session, scope.revision)
	return err == nil && current == scope
}

// A chooser was admitted against an exact UI revision. After admission, panel
// sizing may change without changing the folder contents it is granting. Only
// this completion path tolerates geometry revisions; opens remain exact.
func (a *App) currentDockFolderGrantLocked(scope dockFolderScope) bool {
	current, err := a.dockFolderScopeLocked(scope.session, a.dockViewState.Revision)
	current.revision = scope.revision
	return err == nil && current == scope
}

func (a *App) SetDockFolderSort(session, revision uint64, field, direction string, foldersFirst bool) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	scope, err := a.dockFolderScopeLocked(session, revision)
	if err != nil {
		return err
	}
	if a.dockController == nil {
		return errors.New("folder browsing is unavailable")
	}
	return a.dockController.SetFolderSort(session, scope.folderRevision, platform.FolderSort{Field: field, Direction: direction, FoldersFirst: foldersFirst})
}

func (a *App) RequestDockFolderAccess(session, revision uint64) error {
	a.viewMu.Lock()
	scope, err := a.dockFolderScopeLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	source := a.dockFolders
	if source == nil {
		a.viewMu.Unlock()
		return errors.New("folder access is unavailable")
	}
	if a.dockFolderGrant != nil {
		a.viewMu.Unlock()
		return errors.New("a folder access request is already active")
	}
	ctx, cancel := context.WithCancel(context.Background())
	owner := &dockFolderGrantOwner{scope: scope, cancel: cancel}
	a.dockFolderGrant = owner
	a.viewMu.Unlock()
	defer cancel()
	grant, err := source.RequestFolderAccess(ctx, scope.ref)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err == nil && (grant.FolderIdentity != scope.ref.Identity || grant.Status != "ready") {
		err = errors.New("folder access was not granted")
		if grant.Reason != "" {
			err = errors.New(grant.Reason)
		}
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.dockFolderGrant == owner {
		a.dockFolderGrant = nil
	}
	if !a.currentDockFolderGrantLocked(scope) {
		return err
	}
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			a.publishDockFolderErrorLocked(err)
		}
		return err
	}
	if a.dockController != nil {
		a.dockController.RefreshFolder(session, scope.folderRevision)
	}
	return nil
}

func (a *App) CancelDockFolderAccess(session, revision uint64) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if owner := a.dockFolderGrant; owner != nil && owner.scope.session == session && owner.scope.revision == revision {
		owner.cancel()
	}
}

func (a *App) syncDockFolderGrantLocked() {
	owner := a.dockFolderGrant
	if owner == nil {
		return
	}
	s := a.settingsSnapshot()
	stopping := false
	select {
	case <-a.captureStop:
		stopping = true
	default:
	}
	if stopping || !s.Dock.FolderPop.Enabled || s.Behavior.Paused || a.sessionInactive {
		owner.cancel()
	}
}

func (a *App) OpenDockFolderEntry(session, revision uint64, itemID string) error {
	a.viewMu.Lock()
	scope, err := a.dockFolderScopeLocked(session, revision)
	source := a.dockFolders
	if err == nil && !a.dockFolderHasItemLocked(itemID) {
		err = errors.New("item is not available in this folder preview")
	}
	a.viewMu.Unlock()
	if err != nil {
		return err
	}
	if source == nil {
		return errors.New("folder opening is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	guard := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !a.currentDockFolderLocked(scope) || !a.dockFolderHasItemLocked(itemID) {
			return errStaleDockSession
		}
		return nil
	}
	err = source.OpenFolderEntryGuarded(ctx, scope.ref, itemID, guard)
	a.viewMu.Lock()
	if a.currentDockFolderLocked(scope) {
		a.publishDockFolderErrorLocked(err)
	}
	a.viewMu.Unlock()
	return err
}

func (a *App) dockFolderHasItemLocked(itemID string) bool {
	folder := a.dockState.Folder
	if itemID == "" || folder == nil || (folder.Status != "ready" && folder.Status != "partial") {
		return false
	}
	for _, entry := range folder.Entries {
		if entry.ID == itemID && (entry.Kind == "file" || entry.Kind == "folder") {
			return true
		}
	}
	return false
}

func (a *App) publishDockFolderErrorLocked(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if a.dockViewState.Error == message {
		return
	}
	a.dockRevision++
	a.dockViewState.Revision = a.dockRevision
	a.dockViewState.Error = message
	a.emit("dock:error", struct {
		Session  uint64 `json:"session"`
		Revision uint64 `json:"revision"`
		Message  string `json:"message"`
	}{a.dockState.Session, a.dockRevision, message})
}
