package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"option-tab/internal/actions"
	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

type DockBounds struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type DockItemView struct {
	Kind     string          `json:"kind"`
	AppID    domain.AppID    `json:"appId"`
	BundleID string          `json:"bundleId"`
	Path     string          `json:"path"`
	Title    string          `json:"title"`
	Bounds   DockBounds      `json:"bounds"`
	ScreenID domain.ScreenID `json:"screenId"`
	Edge     string          `json:"edge"`
}

type DockViewState struct {
	ContentKind        string            `json:"contentKind"`
	Folder             *dock.FolderState `json:"folder,omitempty"`
	Open               bool              `json:"open"`
	Revision           uint64            `json:"revision"`
	Session            uint64            `json:"session"`
	Item               DockItemView      `json:"item"`
	Entries            []switcher.Entry  `json:"entries"`
	SelectedWindowID   domain.WindowID   `json:"selectedWindowId"`
	Appearance         config.Appearance `json:"appearance"`
	CardSpacingPx      int               `json:"cardSpacingPx"`
	EmptyReason        string            `json:"emptyReason"`
	Pointer            *DockPointer      `json:"pointer,omitempty"`
	Error              string            `json:"error,omitempty"`
	PreviewDragEnabled bool              `json:"previewDragEnabled"`
	DragGestureFloor   uint64            `json:"dragGestureFloor"`
}

type DockPointer struct {
	Session  uint64  `json:"session"`
	Sequence uint64  `json:"sequence"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Inside   bool    `json:"inside"`
}

type DockPreviewRegion struct {
	WindowID domain.WindowID `json:"windowId"`
	AppID    domain.AppID    `json:"appId"`
	Bounds   DockBounds      `json:"bounds"`
}

type dockFrames struct {
	Session uint64            `json:"session"`
	Frames  map[string]string `json:"frames"`
}

type dockSessionEvent struct {
	Session  uint64 `json:"session"`
	Revision uint64 `json:"revision"`
}

type appDockView struct{ app *App }

func (v appDockView) Show(st dock.State)           { v.app.showDock(st, true) }
func (v appDockView) Update(st dock.State)         { v.app.showDock(st, false) }
func (v appDockView) Hide(session uint64)          { v.app.hideDock(session) }
func (v appDockView) Pointer(st dock.PointerState) { v.app.moveDockPointer(st) }
func (v appDockView) InputTarget(epoch uint64, target platform.DockInputTarget) {
	v.app.setDockInputTarget(epoch, target)
}

func (a *App) moveDockPointer(st dock.PointerState) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if st.Session == 0 || st.Session != a.dockState.Session || !a.dockAllowedLocked() || st.AdmissionEpoch != a.dockState.AdmissionEpoch {
		return
	}
	if a.dockController != nil && st.AdmissionEpoch != a.dockController.AdmissionEpoch() {
		return
	}
	if st.Sequence == 0 || math.IsNaN(st.X) || math.IsNaN(st.Y) || math.IsInf(st.X, 0) || math.IsInf(st.Y, 0) {
		return
	}
	if previous := a.dockViewState.Pointer; previous != nil && st.Sequence <= previous.Sequence {
		return
	}
	pointer := &DockPointer{Session: st.Session, Sequence: st.Sequence, X: st.X, Y: st.Y, Inside: st.Inside}
	a.dockViewState.Pointer = pointer
	a.emit("dock:pointer", *pointer)
}

func (a *App) startDock() {
	if a.dockController == nil && a.dockInput == nil && a.dockShake == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-a.captureStop; cancel() }()
	if a.dockController != nil {
		go a.dockController.Run(ctx)
	}
	if a.dockInput != nil {
		go a.dockInput.Run(ctx)
	}
	if a.dockShake != nil {
		go a.dockShake.Run(ctx)
	}
}

func (a *App) dockAllowedLocked() bool {
	select {
	case <-a.captureStop:
		return false
	default:
	}
	s := a.settingsSnapshot()
	return (s.Dock.Enabled || s.Dock.FolderPop.Enabled) && !s.Behavior.Paused && !a.switcherVisible && !a.prefsOpen && !a.sessionInactive
}

func (a *App) dockItemAllowedLocked(item dock.Item) bool {
	if !a.dockAllowedLocked() {
		return false
	}
	s := a.settingsSnapshot()
	switch item.Kind {
	case "folder":
		return s.Dock.FolderPop.Enabled
	case "", "app":
		return s.Dock.Enabled
	default:
		return false
	}
}

func (a *App) showDock(st dock.State, first bool) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if st.AdmissionEpoch != 0 && a.dockController != nil && st.AdmissionEpoch != a.dockController.AdmissionEpoch() {
		return
	}
	if st.Session == 0 || !a.dockItemAllowedLocked(st.Item) || (!first && st.Session != a.dockState.Session) || (st.Session <= a.dockLastSession && st.Session != a.dockState.Session) {
		if a.dockController != nil {
			a.dockController.Dismiss(st.Session)
		}
		return
	}
	if st.Session != a.dockState.Session {
		a.dismissDockLocked()
		// Hide waits for any old emission before the owner changes; the
		// existing manager's four-slot semaphore also bounds draining streams.
		a.captures.Hide()
		a.captureDockSession.Store(st.Session)
	}
	st.Windows = slices.Clone(st.Windows)
	st.Folder = cloneDockFolder(st.Folder)
	if st.Item.Kind == "folder" {
		st.ContentKind = "folder"
		st.Windows = nil
		st.SelectedWindowID = 0
		a.captures.Hide()
		a.captureDockSession.Store(0)
	}
	a.dockState = st
	a.dockLastSession = st.Session
	a.dockRevision++
	dto := dockStateView(st)
	dto.Open = true
	dto.Revision = a.dockRevision
	dto.Pointer = a.dockViewState.Pointer
	dto.Error = a.dockInputError
	if st.Item.Kind == "folder" {
		dto.Error = ""
		if previous := a.dockViewState.Folder; previous != nil && st.Folder != nil && previous.Revision == st.Folder.Revision && previous.FolderIdentity == st.Folder.FolderIdentity {
			dto.Error = a.dockViewState.Error
		}
	}
	dto.PreviewDragEnabled = st.Item.Kind != "folder" && a.settingsSnapshot().Dock.Input.PreviewDrag
	dto.DragGestureFloor = a.dockDragFloor(st.Session)
	icons := switcher.State{Entries: dto.Entries, Appearance: st.Appearance}
	a.enrichIcons(&icons)
	dto.Entries = icons.Entries
	a.dockViewState = dto
	name := "dock:update"
	if first {
		name = "dock:show"
	}
	a.emit(name, dto)
	if a.dockWindow != nil {
		a.dockWindow.show(st.Bounds)
	}
	if st.Item.Kind == "folder" {
		return
	}
	ids := make([]domain.WindowID, 0, len(st.Windows))
	for _, window := range st.Windows {
		if window.ID != 0 {
			ids = append(ids, window.ID)
		}
	}
	px := st.Appearance.ThumbnailMaxPx
	if px <= 0 {
		px = 256
	}
	if st.Appearance.PreviewSelected {
		px = 1024
	}
	a.captures.Update(ids, st.SelectedWindowID, px)
}

func dockStateView(st dock.State) DockViewState {
	item := st.Item
	contentKind := "windows"
	if item.Kind == "folder" {
		contentKind = "folder"
		item.Path = ""
	}
	entries := make([]switcher.Entry, 0, len(st.Windows))
	for _, w := range st.Windows {
		entries = append(entries, switcher.Entry{WindowID: w.ID, AppID: w.AppID, Title: w.Title, AppName: w.AppName, BundleID: w.BundleID, SpaceID: w.SpaceID, Minimized: w.Minimized, Hidden: w.Hidden, Fullscreen: w.Fullscreen})
	}
	return DockViewState{ContentKind: contentKind, Folder: cloneDockFolder(st.Folder), Session: st.Session, Item: DockItemView{Kind: item.Kind, AppID: item.AppID, BundleID: item.BundleID, Path: item.Path, Title: item.Title, Bounds: DockBounds{X: item.Bounds.X, Y: item.Bounds.Y, W: item.Bounds.W, H: item.Bounds.H}, ScreenID: item.ScreenID, Edge: item.Edge}, Entries: entries, SelectedWindowID: st.SelectedWindowID, Appearance: st.Appearance, CardSpacingPx: st.CardSpacingPx, EmptyReason: st.EmptyReason}
}

// GetDockState lets a newly loaded hidden webview catch up with a hover that
// arrived before its event subscriptions were installed.
func (a *App) GetDockState() *DockViewState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	state := a.dockViewState
	if a.dockState.Session == 0 {
		state = DockViewState{Session: a.dockLastSession, Revision: a.dockRevision, Entries: []switcher.Entry{}, Error: a.dockInputError}
	}
	state.Entries = slices.Clone(state.Entries)
	state.Folder = cloneDockFolder(state.Folder)
	state.DragGestureFloor = a.dockDragFloor(state.Session)
	if state.Pointer != nil {
		pointer := *state.Pointer
		state.Pointer = &pointer
	}
	return &state
}

func (a *App) hideDock(session uint64) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.dockState.Session == session {
		a.dismissDockLocked()
	}
}

// SetDockPreviewRegions publishes clipped panel-local card geometry. Native
// code copies this policy and captures one immutable target at gesture begin.
func (a *App) SetDockPreviewRegions(session, revision uint64, regions []DockPreviewRegion) error {
	return a.setDockPreviewRegions(session, revision, regions)
}

func (a *App) dismissDockLocked() {
	session := a.dockState.Session
	if session == 0 {
		return
	}
	a.captures.Hide()
	a.captureDockSession.Store(0)
	a.dockState = dock.State{}
	a.retireDockWheelState()
	a.dockViewState = DockViewState{}
	a.dockRevision++
	if a.dockWindow != nil {
		a.dockWindow.hide()
	}
	a.emit("dock:hide", dockSessionEvent{Session: session, Revision: a.dockRevision})
}

func (a *App) syncDockSuspensionLocked() {
	if a.dockController != nil {
		a.dockController.Suspend(a.switcherVisible || a.prefsOpen || a.sessionInactive)
	}
	if !a.dockItemAllowedLocked(a.dockState.Item) {
		a.dismissDockLocked()
	}
	a.syncDockInputLocked()
	a.syncDockShakeLocked()
	a.syncDockFolderGrantLocked()
}

func (a *App) setSessionInactive(inactive bool) {
	a.transitionSession(0, inactive)
	if inactive {
		a.cancelDockPreviewDrag()
	}
}

func (a *App) backgroundCaptureAllowed() bool {
	s := a.settingsSnapshot()
	if !s.Behavior.CaptureInBackground || s.Behavior.Paused {
		return false
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	select {
	case <-a.captureStop:
		return false
	default:
	}
	return !a.switcherVisible && a.dockState.Session == 0 && !a.prefsOpen && !a.sessionInactive
}

type backgroundEpoch struct{ view, dock, session uint64 }

func (a *App) backgroundCaptureEpoch() backgroundEpoch {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return backgroundEpoch{a.viewGeneration, a.dockRevision, a.sessionGeneration}
}

func (a *App) publishBackgroundCache(epoch backgroundEpoch, next map[domain.WindowID]string) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	current := backgroundEpoch{a.viewGeneration, a.dockRevision, a.sessionGeneration}
	s := a.settingsSnapshot()
	select {
	case <-a.captureStop:
		return
	default:
	}
	if current != epoch || !s.Behavior.CaptureInBackground || s.Behavior.Paused || a.switcherVisible || a.dockState.Session != 0 || a.prefsOpen || a.sessionInactive {
		return
	}
	a.thumbCacheMu.Lock()
	defer a.thumbCacheMu.Unlock()
	a.thumbCache = next
}

var errStaleDockSession = errors.New("dock preview session is no longer active")

func (a *App) validateDockTarget(session uint64, windowID domain.WindowID, appID domain.AppID, windowRequired bool) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if session == 0 || session != a.dockState.Session || !a.dockItemAllowedLocked(a.dockState.Item) || a.dockState.Item.Kind == "folder" {
		return errStaleDockSession
	}
	if epoch := a.dockState.AdmissionEpoch; epoch != 0 && a.dockController != nil && epoch != a.dockController.AdmissionEpoch() {
		return errStaleDockSession
	}
	if appID <= 0 || appID != a.dockState.Item.AppID || a.dockState.EmptyReason == "notRunning" {
		return errors.New("application is not available in this Dock preview")
	}
	if windowID == 0 {
		if windowRequired {
			return errors.New("this action requires a visible window target")
		}
		return nil
	}
	for _, window := range a.dockState.Windows {
		if window.ID == windowID && window.AppID == appID {
			return nil
		}
	}
	return errors.New("window is not available in this Dock preview")
}

func (a *App) SelectDockWindow(session, id uint64) {
	a.viewMu.Lock()
	app := a.dockState.Item.AppID
	a.viewMu.Unlock()
	if a.validateDockTarget(session, domain.WindowID(id), app, true) == nil && a.dockController != nil {
		a.dockController.SelectWindow(session, domain.WindowID(id))
	}
}

func (a *App) SetDockPanelSize(session uint64, width, height float64) {
	a.viewMu.Lock()
	current := session != 0 && session == a.dockState.Session && a.dockAllowedLocked()
	a.viewMu.Unlock()
	if current && a.dockController != nil {
		a.dockController.SetPanelBounds(session, domain.Bounds{W: width, H: height})
	}
}

func (a *App) PerformDockAction(session uint64, kind string, windowID uint64, appID int) (actions.Result, error) {
	windowRequired := kind == "focus" || kind == "close" || kind == "minimize" || kind == "fullscreen"
	guard := func() error {
		return a.validateDockTarget(session, domain.WindowID(windowID), domain.AppID(appID), windowRequired)
	}
	if err := guard(); err != nil {
		return actions.Result{}, err
	}
	result, err := actions.New(a.platform).PerformGuarded(kind, domain.WindowID(windowID), domain.AppID(appID), guard)
	if a.dockController != nil {
		a.dockController.Refresh(session)
	}
	return result, err
}

func (a *App) FocusDockWindow(session, windowID uint64, appID int) (actions.Result, error) {
	result, err := a.PerformDockAction(session, "focus", windowID, appID)
	if err == nil && result.Succeeded > 0 && len(result.Failures) == 0 {
		a.controller.NoteFocus(domain.WindowID(windowID))
		a.hideDock(session)
		if a.dockController != nil {
			a.dockController.Dismiss(session)
		}
	}
	return result, err
}

func (a *App) emitCaptureFrame(id domain.WindowID, url string) {
	payload := map[string]string{fmt.Sprint(id): url}
	if session := a.captureDockSession.Load(); session != 0 {
		a.emit("dock:frames", dockFrames{Session: session, Frames: payload})
		return
	}
	frame := switcherFramePayload(a.captureSwitcherSession.Load(), payload)
	if url == "" || a.captureThumbnailsEnabled.Load() {
		a.emit("switcher:thumbnails", frame)
	}
	if url == "" || (a.capturePreviewEnabled.Load() && a.captureSelected.Load() == uint64(id)) {
		a.emit("switcher:preview", frame)
	}
}

func switcherFramePayload(session uint64, frames map[string]string) any {
	if session == 0 {
		return frames // compatibility with direct, unscoped test/demo views
	}
	return dockFrames{Session: session, Frames: frames}
}

func (a *App) configureDock(settings config.Settings) {
	a.viewMu.Lock()
	// Eligibility may change independently of the gesture feature switches.
	// Retire the admitted owner before publishing the next Dock configuration.
	a.cancelDockPreviewDrag()
	if a.dockController != nil {
		a.dockController.Configure(settings)
	}
	a.syncDockSuspensionLocked()
	a.viewMu.Unlock()
	a.disableDockWheel()
	a.syncDockPreviewDrag(settings)
}

func (a *App) wireDockController() {
	observer, ok := a.platform.(platform.DockObservationSource)
	if !ok {
		return
	}
	deps := dock.Deps{Observations: observer, Windows: a.platform, Env: a.platform, Folders: a.dockFolders, View: appDockView{a}, SelfBundleID: selfBundleID}
	deps.Apps, _ = a.platform.(platform.ApplicationSource)
	deps.AppWindows, _ = a.platform.(platform.ApplicationWindowPresenceSource)
	a.dockController = dock.NewController(deps, a.settingsSnapshot())
}
