package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

type LauncherReferenceView struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	BundleID string `json:"bundleID"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
	Revision uint64 `json:"revision"`
}
type LauncherItemSettings struct {
	IconIDs    []string                `json:"iconIDs"`
	ProfileID  string                  `json:"profileID"`
	Revision   string                  `json:"revision"`
	Items      []config.LauncherItem   `json:"items"`
	References []LauncherReferenceView `json:"references"`
}
type LauncherItemIcon struct {
	ID      string `json:"id"`
	DataURL string `json:"dataURL"`
}
type LauncherItemStatus struct {
	Available bool   `json:"available"`
	Busy      bool   `json:"busy"`
	Reason    string `json:"reason"`
}
type launcherItemJob struct {
	ctx    context.Context
	cancel context.CancelFunc
	epoch  uint64
	kind   string
}

// Admission and catalogs are protected by viewMu. Source calls and persistence
// run outside it. A job retains busy until native cleanup has joined.
type appLauncherItems struct {
	refs                     platform.LauncherReferenceSource
	icons                    platform.LauncherIconSource
	ctx                      context.Context
	cancel                   context.CancelFunc
	done                     chan struct{}
	wg                       sync.WaitGroup
	started, closed, allowed bool
	epoch                    uint64
	job                      *launcherItemJob
	references               map[string]LauncherReferenceView
	images                   map[string][]byte
	reason                   string
}

func launcherItemsError(code string) error { return errors.New("launcher items: " + code) }
func launcherItemsRevision(items []config.LauncherItem) string {
	return config.LauncherItemsRevision(items)
}

// Transferred selections are structural placeholders, never private records.
func launcherSelectionPlaceholder(id string) bool {
	suffix, ok := strings.CutPrefix(id, "selection-")
	if !ok || len(suffix) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(suffix)
	return err == nil && hex.EncodeToString(decoded) == suffix
}

func cloneLauncherItems(items []config.LauncherItem) []config.LauncherItem {
	out := append([]config.LauncherItem{}, items...)
	for i := range out {
		out[i].Members = slices.Clone(out[i].Members)
	}
	return out
}

func launcherReferenceView(r platform.LauncherReference) LauncherReferenceView {
	return LauncherReferenceView{ID: r.ID, Kind: r.Kind, Label: r.Label, BundleID: r.BundleID, State: r.State, Reason: r.Reason, Revision: r.Revision}
}

func (a *App) wireLauncherItems(refs platform.LauncherReferenceSource, icons platform.LauncherIconSource) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &appLauncherItems{refs: refs, icons: icons, ctx: ctx, cancel: cancel, done: make(chan struct{}), references: map[string]LauncherReferenceView{}, images: map[string][]byte{}}
	a.viewMu.Lock()
	a.stopLauncherItemsLocked()
	a.launcherItems = m
	a.syncLauncherItemAdmissionLocked()
	a.viewMu.Unlock()
}

func (a *App) syncLauncherItemAdmissionLocked() {
	m := a.launcherItems
	if m == nil {
		return
	}
	allowed := !m.closed && a.launcherChoicesAllowedLocked()
	if m.allowed != allowed {
		m.allowed = allowed
		m.epoch++
		if !allowed && m.job != nil && m.job.kind != "load" {
			m.job.cancel()
		}
		a.publishLauncherItemsLocked(m)
	}
}

func (a *App) stopLauncherItemsLocked() {
	m := a.launcherItems
	if m == nil || m.closed {
		return
	}
	if a.launcherItemPanels != nil {
		for id := range a.launcherItemPanels.owners {
			a.retireLauncherItemPanelLocked(id)
		}
	}
	m.closed = true
	m.allowed = false
	m.epoch++
	m.cancel()
	a.publishLauncherItemsLocked(m)
	go func() {
		m.wg.Wait()
		if m.refs != nil {
			_ = m.refs.Close()
		}
		if m.icons != nil {
			_ = m.icons.Close()
		}
		close(m.done)
	}()
}

func (a *App) launcherItemStatusLocked() LauncherItemStatus {
	m := a.launcherItems
	if m == nil || m.closed {
		return LauncherItemStatus{Reason: "unavailable"}
	}
	return LauncherItemStatus{Available: m.allowed && (m.refs != nil || m.icons != nil), Busy: m.job != nil, Reason: m.reason}
}

func (a *App) GetLauncherItemStatus() LauncherItemStatus {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherItemAdmissionLocked()
	return a.launcherItemStatusLocked()
}

func (a *App) publishLauncherItemsLocked(m *appLauncherItems) {
	if a.launcherItems == m {
		a.emit("launcher:items", a.launcherItemStatusLocked())
	}
}

func (a *App) beginLauncherItemLocked(m *appLauncherItems, kind string) *launcherItemJob {
	ctx, cancel := context.WithCancel(m.ctx)
	j := &launcherItemJob{ctx: ctx, cancel: cancel, epoch: m.epoch, kind: kind}
	m.job = j
	m.reason = ""
	m.wg.Add(1)
	return j
}

func (a *App) admitLauncherItem(kind string) (*appLauncherItems, *launcherItemJob, error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherItemAdmissionLocked()
	m := a.launcherItems
	if m == nil || m.closed || !m.started || !m.allowed {
		return nil, nil, launcherItemsError("unavailable")
	}
	if m.job != nil {
		return nil, nil, launcherItemsError("busy")
	}
	return m, a.beginLauncherItemLocked(m, kind), nil
}

func (a *App) launcherItemCurrentLocked(m *appLauncherItems, j *launcherItemJob) bool {
	a.syncLauncherItemAdmissionLocked()
	return a.launcherItems == m && !m.closed && m.allowed && m.epoch == j.epoch && m.job == j && j.ctx.Err() == nil
}

func (a *App) finishLauncherItem(m *appLauncherItems, j *launcherItemJob) {
	j.cancel()
	a.viewMu.Lock()
	if m.job == j {
		m.job = nil
	}
	a.publishLauncherItemsLocked(m)
	a.viewMu.Unlock()
	m.wg.Done()
}

func launcherUsed(s config.Settings) (map[string]string, map[string]bool) {
	refs := map[string]string{}
	icons := map[string]bool{}
	for _, p := range s.ReplacementDock.Profiles {
		for _, x := range p.Items {
			if x.ReferenceID != "" {
				refs[x.ReferenceID] = x.Kind
			}
			if x.IconID != "" {
				icons[x.IconID] = true
			}
		}
	}
	return refs, icons
}

func (a *App) startLauncherItems() {
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m.closed || m.started {
		a.viewMu.Unlock()
		return
	}
	m.started = true
	j := a.beginLauncherItemLocked(m, "load")
	s := a.settingsSnapshot()
	a.viewMu.Unlock()
	go func() {
		defer a.finishLauncherItem(m, j)
		refs, icons := launcherUsed(s)
		for id, kind := range refs {
			if j.ctx.Err() != nil {
				return
			}
			r := platform.LauncherReference{ID: id, Kind: kind, State: "needsSelection", Reason: "missing"}
			if m.refs != nil {
				if resolved, err := m.refs.ResolveLauncherReference(j.ctx, id); err == nil && resolved.ID == id {
					r = resolved
				}
			}
			a.viewMu.Lock()
			if a.launcherItems == m && !m.closed {
				m.references[id] = launcherReferenceView(r)
			}
			a.viewMu.Unlock()
		}
		for id := range icons {
			if j.ctx.Err() != nil {
				return
			}
			if m.icons != nil {
				b, err := m.icons.LauncherIcon(j.ctx, id)
				if err == nil {
					a.viewMu.Lock()
					if a.launcherItems == m && !m.closed {
						_ = cacheLauncherIcon(m, id, b)
					}
					a.viewMu.Unlock()
				}
			}
		}
	}()
}

func (a *App) launcherItemSettingsLocked(profileID string) (LauncherItemSettings, error) {
	s := a.settingsSnapshot()
	for _, p := range s.ReplacementDock.Profiles {
		if p.ID == profileID {
			out := LauncherItemSettings{ProfileID: profileID, Revision: launcherItemsRevision(p.Items), Items: cloneLauncherItems(p.Items), References: []LauncherReferenceView{}, IconIDs: []string{}}
			if m := a.launcherItems; m != nil {
				for id := range m.images {
					out.IconIDs = append(out.IconIDs, id)
				}
				sort.Strings(out.IconIDs)
				for _, r := range m.references {
					out.References = append(out.References, r)
				}
			}
			known := map[string]bool{}
			for _, r := range out.References {
				known[r.ID] = true
			}
			for _, item := range p.Items {
				if item.ReferenceID != "" && !known[item.ReferenceID] {
					out.References = append(out.References, LauncherReferenceView{ID: item.ReferenceID, Kind: item.Kind, State: "needsSelection", Reason: "missing"})
					known[item.ReferenceID] = true
				}
			}
			sort.Slice(out.References, func(i, j int) bool { return out.References[i].ID < out.References[j].ID })
			return out, nil
		}
	}
	return LauncherItemSettings{}, launcherItemsError("profileMissing")
}

func (a *App) GetLauncherItemSettings(profileID string) (LauncherItemSettings, error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherItemAdmissionLocked()
	if a.launcherItems == nil || !a.launcherItems.allowed {
		return LauncherItemSettings{}, launcherItemsError("unavailable")
	}
	return a.launcherItemSettingsLocked(profileID)
}

func (a *App) SetLauncherItems(profileID, expectedRevision string, items []config.LauncherItem) (LauncherItemSettings, error) {
	items = cloneLauncherItems(items)
	m, j, err := a.admitLauncherItem("save")
	if err != nil {
		return LauncherItemSettings{}, err
	}
	defer a.finishLauncherItem(m, j)
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.viewMu.Lock()
	if !a.launcherItemCurrentLocked(m, j) {
		a.viewMu.Unlock()
		return LauncherItemSettings{}, launcherItemsError("retired")
	}
	s := a.settingsSnapshot()
	a.viewMu.Unlock()
	index := -1
	for i, p := range s.ReplacementDock.Profiles {
		if p.ID == profileID {
			index = i
			break
		}
	}
	if index < 0 {
		return LauncherItemSettings{}, launcherItemsError("profileMissing")
	}
	if launcherItemsRevision(s.ReplacementDock.Profiles[index].Items) != expectedRevision {
		return LauncherItemSettings{}, launcherItemsError("staleRevision")
	}
	oldRefs, oldIcons := launcherUsed(s)
	s.ReplacementDock.Profiles[index].Items = items
	if err = config.ValidateReplacementDock(s.ReplacementDock); err != nil {
		return LauncherItemSettings{}, err
	}
	if err = a.saveSettingsLocked(s); err != nil {
		return LauncherItemSettings{}, err
	}
	// Settings are committed: private cleanup uses the manager lifetime, not a
	// preference click that can retire while disk work finishes. No user target is deleted.
	refs, icons := launcherUsed(s)
	var cleanupErr error
	for id := range oldRefs {
		if _, used := refs[id]; !used && launcherSelectionPlaceholder(id) {
			a.viewMu.Lock()
			delete(m.references, id)
			a.viewMu.Unlock()
			continue
		}
		if _, used := refs[id]; !used && m.refs != nil {
			if e := m.refs.RemoveLauncherReference(m.ctx, id); e != nil {
				cleanupErr = e
			} else {
				a.viewMu.Lock()
				delete(m.references, id)
				a.viewMu.Unlock()
			}
		}
	}
	for id := range oldIcons {
		if !icons[id] && m.icons != nil {
			if e := m.icons.RemoveLauncherIcon(m.ctx, id); e != nil {
				cleanupErr = e
			} else {
				a.viewMu.Lock()
				delete(m.images, id)
				a.viewMu.Unlock()
			}
		}
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if cleanupErr != nil {
		m.reason = "cleanupFailed"
	}
	if !a.launcherItemCurrentLocked(m, j) {
		return LauncherItemSettings{}, launcherItemsError("retired")
	}
	out, err := a.launcherItemSettingsLocked(profileID)
	if cleanupErr != nil {
		return out, launcherItemsError("cleanupFailed")
	}
	return out, err
}

func (a *App) ChooseLauncherItemReference(kind string) (LauncherReferenceView, error) {
	if kind != "app" && kind != "folder" && kind != "file" {
		return LauncherReferenceView{}, launcherItemsError("invalidKind")
	}
	return a.selectLauncherReference(kind, "", false)
}

func (a *App) RelinkLauncherItemReference(id string) (LauncherReferenceView, error) {
	return a.selectLauncherReference("", id, true)
}

func (a *App) selectLauncherReference(kind, id string, relink bool) (LauncherReferenceView, error) {
	m, j, err := a.admitLauncherItem("reference")
	if err != nil {
		return LauncherReferenceView{}, err
	}
	defer a.finishLauncherItem(m, j)
	a.viewMu.Lock()
	_, known := m.references[id]
	used, _ := launcherUsed(a.settingsSnapshot())
	known = known || used[id] != ""
	full := len(m.references) >= 144
	a.viewMu.Unlock()
	if m.refs == nil {
		return LauncherReferenceView{}, launcherItemsError("unavailable")
	}
	if relink && !known {
		return LauncherReferenceView{}, launcherItemsError("unknownReference")
	}
	if !relink && full {
		return LauncherReferenceView{}, launcherItemsError("catalogFull")
	}
	var r platform.LauncherReference
	if relink {
		r, err = m.refs.RelinkLauncherReference(j.ctx, id)
	} else {
		r, err = m.refs.ChooseLauncherReference(j.ctx, kind)
	}
	if err != nil && r.ID == "" {
		return LauncherReferenceView{}, err
	}
	if r.ID == "" || (relink && r.ID != id) || (!relink && r.Kind != kind) {
		return LauncherReferenceView{}, launcherItemsError("invalidResult")
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	v := launcherReferenceView(r)
	// Native may have committed before cancellation. Keep inert metadata visible
	// to this manager for explicit retry/removal, never publish into a successor.
	if a.launcherItems == m && !m.closed {
		m.references[r.ID] = v
	}
	if err != nil {
		return LauncherReferenceView{}, err
	}
	if !a.launcherItemCurrentLocked(m, j) {
		return LauncherReferenceView{}, launcherItemsError("retired")
	}
	return v, nil
}

func (a *App) CancelLauncherItemSelection() error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherItemAdmissionLocked()
	m := a.launcherItems
	if m == nil || !m.allowed {
		return launcherItemsError("unavailable")
	}
	if m.job != nil && (m.job.kind == "reference" || m.job.kind == "icon") {
		m.epoch++
		m.job.cancel()
	}
	a.publishLauncherItemsLocked(m)
	return nil
}

func (a *App) RemoveUnusedLauncherReference(id string) error {
	return a.removeUnusedLauncherResource(id, false)
}

func (a *App) RemoveUnusedLauncherIcon(id string) error {
	return a.removeUnusedLauncherResource(id, true)
}

func (a *App) removeUnusedLauncherResource(id string, icon bool) error {
	m, j, err := a.admitLauncherItem("remove")
	if err != nil {
		return err
	}
	defer a.finishLauncherItem(m, j)
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	a.viewMu.Lock()
	if !a.launcherItemCurrentLocked(m, j) {
		a.viewMu.Unlock()
		return launcherItemsError("retired")
	}
	refs, icons := launcherUsed(a.settingsSnapshot())
	_, known := m.references[id]
	if icon {
		_, known = m.images[id]
	}
	a.viewMu.Unlock()
	if !known {
		return launcherItemsError("unknownResource")
	}
	if (!icon && refs[id] != "") || (icon && icons[id]) {
		return launcherItemsError("referenced")
	}
	if icon {
		if m.icons == nil {
			return launcherItemsError("unavailable")
		}
		err = m.icons.RemoveLauncherIcon(j.ctx, id)
	} else if !launcherSelectionPlaceholder(id) {
		if m.refs == nil {
			return launcherItemsError("unavailable")
		}
		err = m.refs.RemoveLauncherReference(j.ctx, id)
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if err != nil {
		m.reason = "cleanupFailed"
		return err
	}
	if icon {
		delete(m.images, id)
	} else {
		delete(m.references, id)
	}
	if !a.launcherItemCurrentLocked(m, j) {
		return launcherItemsError("retired")
	}
	return nil
}

func cacheLauncherIcon(m *appLauncherItems, id string, b []byte) error {
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != id || len(b) == 0 || len(b) > 512<<10 {
		return launcherItemsError("invalidIcon")
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != id {
		return launcherItemsError("invalidIcon")
	}
	total := len(b)
	for other, data := range m.images {
		if other != id {
			total += len(data)
		}
	}
	if total > 8<<20 || len(m.images) >= 128 && m.images[id] == nil {
		return launcherItemsError("catalogFull")
	}
	m.images[id] = slices.Clone(b)
	return nil
}

func launcherIconView(id string, b []byte) LauncherItemIcon {
	return LauncherItemIcon{ID: id, DataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)}
}

func (a *App) ChooseLauncherItemIcon() (LauncherItemIcon, error) {
	m, j, err := a.admitLauncherItem("icon")
	if err != nil {
		return LauncherItemIcon{}, err
	}
	defer a.finishLauncherItem(m, j)
	if m.icons == nil {
		return LauncherItemIcon{}, launcherItemsError("unavailable")
	}
	a.viewMu.Lock()
	full := len(m.images) >= 128
	a.viewMu.Unlock()
	if full {
		return LauncherItemIcon{}, launcherItemsError("catalogFull")
	}
	id, selectionErr := m.icons.ChooseLauncherIcon(j.ctx)
	if selectionErr != nil && id == "" {
		return LauncherItemIcon{}, selectionErr
	}
	// Read a committed immutable asset even when its originating chooser retired;
	// it remains an inert catalog entry, with no profile mutation.
	b, err := m.icons.LauncherIcon(m.ctx, id)
	if err != nil {
		return LauncherItemIcon{}, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItems != m || m.closed {
		return LauncherItemIcon{}, launcherItemsError("retired")
	}
	if err = cacheLauncherIcon(m, id, b); err != nil {
		return LauncherItemIcon{}, err
	}
	if selectionErr != nil {
		return LauncherItemIcon{}, selectionErr
	}
	if !a.launcherItemCurrentLocked(m, j) {
		return LauncherItemIcon{}, launcherItemsError("retired")
	}
	return launcherIconView(id, b), nil
}

func (a *App) GetLauncherItemIcon(id string) (LauncherItemIcon, error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherItemAdmissionLocked()
	m := a.launcherItems
	if m == nil || !m.allowed {
		return LauncherItemIcon{}, launcherItemsError("unavailable")
	}
	b, ok := m.images[id]
	if !ok {
		return LauncherItemIcon{}, launcherItemsError("unknownIcon")
	}
	return launcherIconView(id, b), nil
}

// Runtime reads are independent of preferences admission. Their captured source
// is joined by manager shutdown, and results cannot enter a replacement owner.
func (a *App) resolveLauncherItemReference(ctx context.Context, id string) (platform.LauncherReference, error) {
	if ctx == nil {
		return platform.LauncherReference{}, launcherItemsError("cancelled")
	}
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m.closed || m.refs == nil {
		a.viewMu.Unlock()
		return platform.LauncherReference{}, launcherItemsError("unavailable")
	}
	m.wg.Add(1)
	a.viewMu.Unlock()
	defer m.wg.Done()
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	if m.ctx.Err() != nil {
		cancel()
	}
	if err := child.Err(); err != nil {
		return platform.LauncherReference{}, err
	}
	r, err := m.refs.ResolveLauncherReference(child, id)
	if err != nil {
		return r, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItems != m || m.closed || child.Err() != nil {
		return platform.LauncherReference{}, launcherItemsError("retired")
	}
	if r.ID != id {
		return platform.LauncherReference{}, launcherItemsError("invalidResult")
	}
	if len(m.references) < 144 || m.references[id].ID != "" {
		m.references[id] = launcherReferenceView(r)
	}
	return r, nil
}

func (a *App) readLauncherItemIcon(ctx context.Context, id string) ([]byte, error) {
	if ctx == nil {
		return nil, launcherItemsError("cancelled")
	}
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m.closed || m.icons == nil {
		a.viewMu.Unlock()
		return nil, launcherItemsError("unavailable")
	}
	m.wg.Add(1)
	a.viewMu.Unlock()
	defer m.wg.Done()
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	if m.ctx.Err() != nil {
		cancel()
	}
	if err := child.Err(); err != nil {
		return nil, err
	}
	b, err := m.icons.LauncherIcon(child, id)
	if err != nil {
		return nil, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItems != m || m.closed || child.Err() != nil {
		return nil, launcherItemsError("retired")
	}
	if err = cacheLauncherIcon(m, id, b); err != nil {
		return nil, err
	}
	return slices.Clone(b), nil
}
