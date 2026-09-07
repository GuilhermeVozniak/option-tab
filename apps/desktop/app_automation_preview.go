package main

import (
	"context"
	"math"
	"reflect"
	"strconv"

	"option-tab/internal/actions"
	"option-tab/internal/automation"
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

type automationPreviewOwner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	process    platform.ProcessIdentity
	identities map[domain.WindowID]platform.AutomationWindowIdentity
	state      AutomationPreviewViewState
	settings   config.Settings
	bounds     domain.Bounds
	host       *dockWindow
	capture    bool
}
type automationPreviewFrameLease struct {
	session, revision uint64
	identities        map[domain.WindowID]platform.AutomationWindowIdentity
}
type automationPreviewFrames struct {
	Session  uint64                     `json:"session"`
	Revision uint64                     `json:"revision"`
	Sequence uint64                     `json:"sequence"`
	Frames   map[domain.WindowID]string `json:"frames"`
}

func previewError(code, message string) error { return &automation.Error{Code: code, Message: message} }

func previewRetired() error {
	return previewError("retired", "automation preview is no longer current")
}

// Call only under viewMu. All native queries and caller guards remain outside it.
func (a *App) automationPreviewAllowedLocked() bool {
	r := a.automation
	if r == nil || r.ctx.Err() != nil || a.sessionInactive || a.settingsSnapshot().Behavior.Paused {
		return false
	}
	select {
	case <-a.captureStop:
		return false
	default:
		return true
	}
}

func (a *App) automationPreviewTargetLocked(session, revision uint64) (*automationPreviewOwner, error) {
	r := a.automation
	if !a.automationPreviewAllowedLocked() || r.preview == nil || session == 0 || r.preview.state.Session != session || (revision != 0 && r.preview.state.Revision != revision) || r.preview.ctx.Err() != nil || !reflect.DeepEqual(r.preview.settings, a.settingsSnapshot()) {
		return nil, previewRetired()
	}
	return r.preview, nil
}

func (a *App) prepareAutomationPreview(ctx context.Context, process platform.ProcessIdentity, windows []domain.Window, settings config.Settings) ([]switcher.Entry, map[domain.WindowID]platform.AutomationWindowIdentity, error) {
	source, ok := a.platform.(platform.AutomationIdentitySource)
	if !ok {
		return nil, nil, previewError("unsupported", "window identity source unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	fc := filter.Context{ActiveAppID: a.platform.ActiveApp(), ActiveSpaceID: a.platform.ActiveSpace(), ActiveScreenID: a.platform.ActiveScreen(), CursorScreenID: a.platform.CursorScreen(), SelfBundleID: selfBundleID}
	windows = filter.Apply(windows, settings.Filters, config.ShortcutScope{}, fc)
	entries := make([]switcher.Entry, 0, len(windows))
	identities := make(map[domain.WindowID]platform.AutomationWindowIdentity)
	for _, w := range windows {
		if w.AppID != process.PID {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		id, err := source.WindowIdentity(w.ID)
		if err != nil {
			return nil, nil, err
		}
		if id.ID != w.ID || id.Process != process || !source.WindowIdentityCurrent(id) {
			return nil, nil, previewError("staleIdentity", "preview window identity changed")
		}
		if _, duplicate := identities[w.ID]; duplicate {
			return nil, nil, previewError("unavailable", "duplicate window inventory")
		}
		identities[w.ID] = id
		entries = append(entries, switcher.Entry{WindowID: w.ID, AppID: w.AppID, Title: w.Title, AppName: w.AppName, BundleID: w.BundleID, SpaceID: w.SpaceID, Minimized: w.Minimized, Hidden: w.Hidden, Fullscreen: w.Fullscreen})
	}
	st := switcher.State{Entries: entries}
	a.enrichIcons(&st)
	current, err := source.ProcessIdentity(process.PID)
	if err != nil {
		return nil, nil, err
	}
	if current != process {
		return nil, nil, previewError("staleIdentity", "preview application restarted")
	}
	return st.Entries, identities, ctx.Err()
}

func automationPreviewBounds(screens []domain.Screen, activeScreen domain.ScreenID, point *platform.AutomationPoint, width, height float64) (domain.Bounds, error) {
	if !validMediaPanelSize(width, height) {
		return domain.Bounds{}, previewError("invalidArgument", "invalid preview size")
	}
	if point != nil && (math.IsNaN(point.X) || math.IsNaN(point.Y) || math.IsInf(point.X, 0) || math.IsInf(point.Y, 0)) {
		return domain.Bounds{}, previewError("invalidArgument", "invalid preview position")
	}
	var chosen domain.Bounds
	best := math.Inf(1)
	for _, screen := range screens {
		b := screen.Visible
		if !validMediaPanelSize(b.W, b.H) || math.IsNaN(b.X) || math.IsNaN(b.Y) || math.IsInf(b.X, 0) || math.IsInf(b.Y, 0) {
			continue
		}
		if point == nil {
			if activeScreen != 0 && screen.ID == activeScreen {
				chosen = b
				break
			}
			if chosen.W == 0 || screen.Main {
				chosen = b
			}
			continue
		}
		dx := math.Max(b.X-point.X, math.Max(0, point.X-(b.X+b.W)))
		dy := math.Max(b.Y-point.Y, math.Max(0, point.Y-(b.Y+b.H)))
		distance := math.Hypot(dx, dy)
		if distance < best {
			best = distance
			chosen = b
		}
	}
	if chosen.W == 0 {
		return domain.Bounds{}, previewError("unavailable", "preview display unavailable")
	}
	w, h := math.Min(width, chosen.W), math.Min(height, chosen.H)
	x, y := chosen.X+(chosen.W-w)/2, chosen.Y+(chosen.H-h)/2
	if point != nil {
		x = point.X
		y = point.Y
	}
	return domain.Bounds{X: math.Max(chosen.X, math.Min(x, chosen.X+chosen.W-w)), Y: math.Max(chosen.Y, math.Min(y, chosen.Y+chosen.H-h)), W: w, H: h}, nil
}

func (a *App) showAutomationPreviews(ctx context.Context, process platform.ProcessIdentity, windows []domain.Window, point *platform.AutomationPoint, guard func() error) (automation.Presentation, error) {
	if err := ctx.Err(); err != nil {
		return automation.Presentation{}, err
	}
	if guard == nil {
		return automation.Presentation{}, previewError("invalidArgument", "preview admission guard required")
	}
	if err := guard(); err != nil {
		return automation.Presentation{}, err
	}
	a.viewMu.Lock()
	r := a.automation
	if !a.automationPreviewAllowedLocked() {
		a.viewMu.Unlock()
		return automation.Presentation{}, previewRetired()
	}
	factory := r.previewFactory
	if factory == nil {
		a.viewMu.Unlock()
		return automation.Presentation{}, previewError("unsupported", "automation preview host unavailable")
	}
	r.showSequence++
	sequence := r.showSequence
	r.nextPreview++
	session := r.nextPreview
	settings := a.settingsSnapshot()
	a.viewMu.Unlock()
	entries, identities, err := a.prepareAutomationPreview(ctx, process, windows, settings)
	if err != nil {
		return automation.Presentation{}, err
	}
	bounds, err := automationPreviewBounds(a.platform.Screens(), a.platform.ActiveScreen(), point, 620, 460)
	if err != nil {
		return automation.Presentation{}, err
	}
	capture := (settings.Appearance.Style == config.StyleThumbnails || settings.Appearance.PreviewSelected) && a.platform.ScreenRecording() == platform.PermGranted
	host := factory(session, a.acceptAutomationPreviewEvent)
	if host == nil {
		return automation.Presentation{}, previewError("unsupported", "automation preview host unavailable")
	}
	if err = guard(); err != nil {
		host.close()
		return automation.Presentation{}, err
	}
	if err = ctx.Err(); err != nil {
		host.close()
		return automation.Presentation{}, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if ctx.Err() != nil || !a.automationPreviewAllowedLocked() || a.automation != r || sequence != r.showSequence || !reflect.DeepEqual(settings, a.settingsSnapshot()) {
		host.close()
		return automation.Presentation{}, previewRetired()
	}
	a.retireAutomationPreviewLocked()
	ownerCtx, cancel := context.WithCancel(r.ctx)
	p := &automationPreviewOwner{ctx: ownerCtx, cancel: cancel, process: process, identities: identities, settings: settings, bounds: bounds, host: host, capture: capture, state: AutomationPreviewViewState{Open: true, Session: session, Title: "Application windows", Entries: entries, Appearance: settings.Appearance, CardSpacingPx: 7}}
	if len(entries) > 0 {
		p.state.SelectedWindowID = entries[0].WindowID
		p.state.Title = entries[0].AppName
	} else {
		p.state.EmptyReason = "No windows match the current filters"
	}
	r.preview = p
	a.publishAutomationPreviewLocked(p)
	a.updateAutomationPreviewCaptureLocked(p)
	host.show(bounds)
	return automation.Presentation{Token: strconv.FormatUint(session, 10), Status: "accepted", Bounds: bounds}, nil
}

func (a *App) retireAutomationPreviewLocked() {
	r := a.automation
	if r == nil {
		return
	}
	r.frameLease.Store(nil)
	if r.peer != nil {
		r.peer.Hide()
	}
	r.frameMu.Lock()
	r.frames = nil
	r.frameSequence = 0
	r.frameMu.Unlock()
	p := r.preview
	if p == nil {
		return
	}
	r.preview = nil
	p.cancel()
	if p.host != nil {
		p.host.close()
	}
	r.previewRevision++
	a.emit("automation-preview:hide", map[string]uint64{"session": p.state.Session, "revision": r.previewRevision})
}

func (a *App) syncAutomationPreviewLocked() {
	r := a.automation
	if r != nil && r.preview != nil && (!a.automationPreviewAllowedLocked() || !reflect.DeepEqual(r.preview.settings, a.settingsSnapshot())) {
		r.showSequence++
		a.retireAutomationPreviewLocked()
	}
}

func (a *App) stopAutomationPreview() {
	a.viewMu.Lock()
	r := a.automation
	if r != nil {
		r.showSequence++
		a.retireAutomationPreviewLocked()
	}
	a.viewMu.Unlock()
	if r != nil && r.peer != nil {
		r.peer.Close()
	}
}

func (a *App) publishAutomationPreviewLocked(p *automationPreviewOwner) {
	r := a.automation
	r.previewRevision++
	p.state.Revision = r.previewRevision
	if p.capture {
		identities := make(map[domain.WindowID]platform.AutomationWindowIdentity)
		for _, id := range automationPreviewCaptureIDs(p) {
			identities[id] = p.identities[id]
		}
		r.frameLease.Store(&automationPreviewFrameLease{session: p.state.Session, revision: p.state.Revision, identities: identities})
		r.frameMu.Lock()
		for id := range r.frames {
			if _, ok := identities[id]; !ok {
				delete(r.frames, id)
			}
		}
		r.frameMu.Unlock()
	} else {
		r.frameLease.Store(nil)
	}
	a.emit("automation-preview:update", a.automationPreviewSnapshotLocked(p))
}

func (a *App) updateAutomationPreviewCaptureLocked(p *automationPreviewOwner) {
	r := a.automation
	if r.peer == nil {
		return
	}
	if !p.capture {
		r.peer.Hide()
		return
	}
	r.peer.Update(automationPreviewCaptureIDs(p), p.state.SelectedWindowID, p.state.Appearance.ThumbnailMaxPx)
}

func automationPreviewCaptureIDs(p *automationPreviewOwner) []domain.WindowID {
	ids := make([]domain.WindowID, 0, 30)
	if p.state.SelectedWindowID != 0 {
		ids = append(ids, p.state.SelectedWindowID)
	}
	for _, entry := range p.state.Entries {
		if len(ids) >= 30 {
			break
		}
		if entry.WindowID != p.state.SelectedWindowID {
			ids = append(ids, entry.WindowID)
		}
	}
	return ids
}

// Capture callbacks run under the peer's mutex. They only access immutable
// atomically published leases and the separate frame mutex, never viewMu.
func (a *App) emitAutomationPreviewFrame(id domain.WindowID, url string, identity platform.AutomationWindowIdentity) {
	r := a.automation
	if r == nil || len(url) > 512*1024 {
		return
	}
	lease := r.frameLease.Load()
	if lease == nil {
		return
	}
	expected, ok := lease.identities[id]
	if !ok || (url != "" && identity != expected) {
		return
	}
	r.frameMu.Lock()
	defer r.frameMu.Unlock()
	if r.frameLease.Load() != lease {
		return
	}
	if r.frames == nil {
		r.frames = make(map[domain.WindowID]string)
	}
	total := len(url)
	for other, value := range r.frames {
		if other != id {
			total += len(value)
		}
	}
	_, cached := r.frames[id]
	if total > 4*1024*1024 || (!cached && len(r.frames) >= 30) {
		return
	}
	r.frames[id] = url
	r.frameSequence++
	a.emit("automation-preview:frames", automationPreviewFrames{Session: lease.session, Revision: lease.revision, Sequence: r.frameSequence, Frames: map[domain.WindowID]string{id: url}})
}

// Snapshot includes the current lease's frames so a one-shot capture completed
// before WebKit mounted is not lost through event-before-RPC ordering.
func (a *App) automationPreviewSnapshotLocked(p *automationPreviewOwner) AutomationPreviewViewState {
	state := p.state
	state.Entries = append([]switcher.Entry{}, state.Entries...)
	state.Frames = make(map[domain.WindowID]string)
	r := a.automation
	r.frameMu.Lock()
	defer r.frameMu.Unlock()
	lease := r.frameLease.Load()
	if lease == nil || lease.session != state.Session || lease.revision != state.Revision {
		return state
	}
	state.FrameSequence = r.frameSequence
	for id, url := range r.frames {
		if _, ok := lease.identities[id]; ok {
			state.Frames[id] = url
		}
	}
	return state
}

func (a *App) GetAutomationPreviewState(session uint64) *AutomationPreviewViewState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.automationPreviewTargetLocked(session, 0)
	if err != nil {
		return nil
	}
	state := a.automationPreviewSnapshotLocked(p)
	return &state
}

func (a *App) SelectAutomationPreview(session, revision uint64, id domain.WindowID) error {
	if revision == 0 {
		return previewRetired()
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.automationPreviewTargetLocked(session, revision)
	if err != nil {
		return err
	}
	if _, ok := p.identities[id]; !ok {
		return previewError("notFound", "window is not in this preview")
	}
	p.state.SelectedWindowID = id
	a.publishAutomationPreviewLocked(p)
	a.updateAutomationPreviewCaptureLocked(p)
	return nil
}

func (a *App) CloseAutomationPreview(session, revision uint64) error {
	if revision == 0 {
		return previewRetired()
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if _, err := a.automationPreviewTargetLocked(session, revision); err != nil {
		return err
	}
	a.automation.showSequence++
	a.retireAutomationPreviewLocked()
	return nil
}

func (a *App) hideAutomationPreviews(ctx context.Context, token string, guard func() error) (automation.Presentation, error) {
	session, err := strconv.ParseUint(token, 10, 64)
	if err != nil || session == 0 {
		return automation.Presentation{}, previewError("invalidArgument", "invalid preview token")
	}
	if guard == nil {
		return automation.Presentation{}, previewError("invalidArgument", "preview admission guard required")
	}
	if err = guard(); err != nil {
		return automation.Presentation{}, err
	}
	if err = ctx.Err(); err != nil {
		return automation.Presentation{}, err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if err = ctx.Err(); err != nil {
		return automation.Presentation{}, err
	}
	if _, err = a.automationPreviewTargetLocked(session, 0); err != nil {
		return automation.Presentation{}, err
	}
	a.automation.showSequence++
	a.retireAutomationPreviewLocked()
	return automation.Presentation{Token: token, Status: "accepted"}, nil
}

func (a *App) SetAutomationPreviewSize(session, revision uint64, width, height int) error {
	if revision == 0 {
		return previewRetired()
	}
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return previewError("invalidArgument", "invalid preview size")
	}
	a.viewMu.Lock()
	p, err := a.automationPreviewTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	point := platform.AutomationPoint{X: p.bounds.X, Y: p.bounds.Y}
	a.viewMu.Unlock()
	bounds, err := automationPreviewBounds(a.platform.Screens(), 0, &point, float64(width), float64(height))
	if err != nil {
		return err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	current, err := a.automationPreviewTargetLocked(session, revision)
	if err != nil || current != p {
		return previewRetired()
	}
	if p.bounds == bounds {
		return nil
	}
	p.bounds = bounds
	a.publishAutomationPreviewLocked(p)
	p.host.show(bounds)
	return nil
}

func (a *App) automationPreviewGuard(p *automationPreviewOwner, revision uint64, id platform.AutomationWindowIdentity) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	a.viewMu.Lock()
	current, err := a.automationPreviewTargetLocked(p.state.Session, revision)
	a.viewMu.Unlock()
	if err != nil || current != p {
		return previewRetired()
	}
	source, ok := a.platform.(platform.AutomationIdentitySource)
	if !ok || !source.WindowIdentityCurrent(id) {
		return previewError("staleIdentity", "preview target identity changed")
	}
	a.viewMu.Lock()
	current, err = a.automationPreviewTargetLocked(p.state.Session, revision)
	a.viewMu.Unlock()
	if err != nil || current != p {
		return previewRetired()
	}
	return p.ctx.Err()
}

func (a *App) PerformAutomationPreviewAction(session, revision uint64, kind string, windowID domain.WindowID, fullscreen bool) error {
	if revision == 0 {
		return previewRetired()
	}
	a.viewMu.Lock()
	p, err := a.automationPreviewTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	id, ok := p.identities[windowID]
	a.viewMu.Unlock()
	if !ok {
		return previewError("notFound", "window is not in this preview")
	}
	var value *bool
	if kind == "fullscreen" {
		value = &fullscreen
	}
	guard := func() error { return a.automationPreviewGuard(p, revision, id) }
	err = actions.New(a.platform).PerformAutomationWindowAction(p.ctx, kind, id, value, guard)
	if err == nil {
		var windows []domain.Window
		windows, err = a.platform.Windows()
		if err == nil {
			var entries []switcher.Entry
			var identities map[domain.WindowID]platform.AutomationWindowIdentity
			entries, identities, err = a.prepareAutomationPreview(p.ctx, p.process, windows, p.settings)
			if err == nil {
				a.viewMu.Lock()
				current, targetErr := a.automationPreviewTargetLocked(session, revision)
				if targetErr != nil || current != p {
					a.viewMu.Unlock()
					return previewRetired()
				}
				a.automation.frameLease.Store(nil)
				if a.automation.peer != nil {
					a.automation.peer.Hide()
				}
				a.automation.frameMu.Lock()
				a.automation.frames = nil
				a.automation.frameMu.Unlock()
				p.identities = identities
				p.state.Entries = entries
				if _, exists := identities[p.state.SelectedWindowID]; !exists {
					p.state.SelectedWindowID = 0
					if len(entries) > 0 {
						p.state.SelectedWindowID = entries[0].WindowID
					}
				}
				p.state.Error = ""
				p.state.EmptyReason = ""
				if len(entries) == 0 {
					p.state.EmptyReason = "No windows match the current filters"
				}
				a.publishAutomationPreviewLocked(p)
				a.updateAutomationPreviewCaptureLocked(p)
				a.viewMu.Unlock()
				return nil
			}
		}
	}
	a.viewMu.Lock()
	if current, targetErr := a.automationPreviewTargetLocked(session, revision); targetErr == nil && current == p {
		p.state.Error = err.Error()
		a.publishAutomationPreviewLocked(p)
	}
	a.viewMu.Unlock()
	return err
}

func (a *App) acceptAutomationPreviewEvent(event platform.MediaPanelEvent) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.automationPreviewTargetLocked(event.Session, 0)
	if err != nil || p.host == nil || !p.host.admitMediaPanelEvent(event) {
		return
	}
	if event.Reason == "hostClosed" {
		a.retireAutomationPreviewLocked()
		return
	}
	if !validMediaPanelSize(event.Bounds.W, event.Bounds.H) || math.IsNaN(event.Bounds.X) || math.IsNaN(event.Bounds.Y) || math.IsInf(event.Bounds.X, 0) || math.IsInf(event.Bounds.Y, 0) {
		return
	}
	if p.bounds != event.Bounds || event.Reason == "topology" {
		p.bounds = event.Bounds
		a.publishAutomationPreviewLocked(p)
	}
}

func (a *App) automationPreviewHostClosed(session uint64, scheduler *dockWindow, window nativeWindow) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.automationPreviewTargetLocked(session, 0)
	if err == nil && p.host == scheduler && scheduler.ownsMediaWindow(window) {
		a.retireAutomationPreviewLocked()
	}
}
