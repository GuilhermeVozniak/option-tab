package dock

import (
	"context"
	"errors"
	"slices"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type folderJob struct {
	ctx                                      context.Context
	ref                                      platform.FolderRef
	sort                                     platform.FolderSort
	session, revision, generation, admission uint64
}
type folderResult struct {
	job     folderJob
	listing platform.FolderListing
	err     error
}

func copyFolderState(state *FolderState) *FolderState {
	if state == nil {
		return nil
	}
	copy := *state
	copy.Entries = slices.Clone(state.Entries)
	return &copy
}

func validFolderSort(sort platform.FolderSort) bool {
	return (sort.Field == "name" || sort.Field == "size" || sort.Field == "modified" || sort.Field == "kind") && (sort.Direction == "asc" || sort.Direction == "desc")
}

// SetFolderSort validates shape immediately; stale presentation/revision
// commands are dropped by the owner without starting filesystem work.
func (c *Controller) SetFolderSort(session, folderRevision uint64, sort platform.FolderSort) error {
	if !validFolderSort(sort) {
		return errors.New("invalid folder sort")
	}
	c.send(command{kind: "folderSort", session: session, folderRevision: folderRevision, folderSort: sort})
	return nil
}

func (c *Controller) RefreshFolder(session, folderRevision uint64) {
	c.send(command{kind: "folderRefresh", session: session, folderRevision: folderRevision})
}

func (l *controllerLoop) cancelFolder() {
	if l.folderCancel != nil {
		l.folderCancel()
		l.folderCancel = nil
	}
	l.folderPending = nil
}

func (l *controllerLoop) beginFolder() {
	if l.candidate == nil || l.candidate.Kind != "folder" || !l.settings.Dock.FolderPop.Enabled {
		return
	}
	identity, err := platform.FolderIdentity(l.candidate.Path)
	l.screen = domain.Bounds{}
	if l.controller.deps.Env != nil {
		for _, screen := range l.controller.deps.Env.Screens() {
			if screen.ID == l.candidate.ScreenID {
				l.screen = screen.Visible
				if l.screen.Area() == 0 {
					l.screen = screen.Bounds
				}
				break
			}
		}
	}
	l.state = State{ContentKind: "folder", AdmissionEpoch: l.admission, Session: l.session, Item: *l.candidate, Appearance: l.settings.Dock.Appearance, CardSpacingPx: l.settings.Dock.CardSpacingPx, Folder: &FolderState{FolderIdentity: identity, Sort: platform.FolderSort{Field: "name", Direction: "asc", FoldersFirst: true}, Entries: []platform.FolderEntry{}}}
	if err != nil || l.screen.Area() == 0 {
		l.state.Folder.Status = "unavailable"
		l.state.Folder.Reason = "folder identity or display unavailable"
		l.state.Folder.Revision = 1
		l.place()
		l.shown = true
		l.publish(true)
		return
	}
	l.requestFolder(l.state.Folder.Sort)
}

func (l *controllerLoop) requestFolder(sort platform.FolderSort) {
	if l.state.Folder == nil || l.candidate == nil {
		return
	}
	l.cancelFolder()
	ctx, cancel := context.WithCancel(l.ctx)
	l.folderCancel = cancel
	folder := l.state.Folder
	folder.Revision++
	folder.Status = "loading"
	folder.Reason = ""
	folder.Entries = []platform.FolderEntry{}
	folder.Partial = false
	folder.Sort = sort
	l.folderPending = &folderJob{ctx: ctx, ref: platform.FolderRef{Identity: folder.FolderIdentity, Path: l.candidate.Path}, sort: sort, session: l.session, revision: folder.Revision, generation: l.generation, admission: l.admission}
	first := !l.shown
	l.shown = true
	l.place()
	l.publish(first)
	l.queryFolder()
}

// Only one folder read is in flight. New requests cancel the previous context
// and replace pending work, without blocking the owner loop on filesystem I/O.
func (l *controllerLoop) queryFolder() {
	if l.folderQuerying || l.folderPending == nil {
		return
	}
	job := *l.folderPending
	l.folderPending = nil
	if job.ctx.Err() != nil {
		return
	}
	l.folderQuerying = true
	c := l.controller
	c.folderWork.Add(1)
	go func() {
		defer c.folderWork.Done()
		result := folderResult{job: job}
		if c.deps.Folders == nil {
			result.err = errors.New("folder access unavailable")
		} else {
			result.listing, result.err = c.deps.Folders.ListFolder(job.ctx, job.ref, job.sort)
			result.listing.Entries = slices.Clone(result.listing.Entries)
		}
		select {
		case c.folderResults <- result:
		case <-l.ctx.Done():
		}
	}()
}

func (l *controllerLoop) acceptFolder(result folderResult) {
	job := result.job
	folder := l.state.Folder
	if job.ctx.Err() != nil || !l.enabled() || !l.settings.Dock.FolderPop.Enabled || l.admission != l.controller.AdmissionEpoch() || job.admission != l.admission || l.candidate == nil || l.candidate.Kind != "folder" || folder == nil || job.session != l.session || job.revision != folder.Revision || job.generation != l.generation || job.ref.Identity != folder.FolderIdentity {
		return
	}
	listing := result.listing
	folder.Revision++
	folder.Entries = []platform.FolderEntry{}
	folder.Reason = listing.Reason
	folder.Status = listing.Status
	folder.Partial = listing.Status == "partial"
	if result.err != nil {
		if folder.Status == "" || folder.Status == "ready" || folder.Status == "partial" {
			folder.Status = "unavailable"
		}
		folder.Reason = result.err.Error()
	}
	if result.err == nil && listing.FolderIdentity != folder.FolderIdentity {
		folder.Status = "unavailable"
		folder.Reason = "folder listing identity changed"
	} else if result.err == nil && (folder.Status == "ready" || folder.Status == "partial") {
		folder.Entries = slices.Clone(listing.Entries[:min(500, len(listing.Entries))])
		if len(listing.Entries) > 500 {
			folder.Status = "partial"
			folder.Partial = true
			folder.Reason = "folder entry limit reached"
		}
	}
	switch folder.Status {
	case "ready", "permissionRequired", "missing", "revoked", "partial", "unavailable":
	default:
		folder.Status = "unavailable"
		folder.Reason = "folder source returned an unsupported state"
	}
	l.place()
	l.publish(false)
}
