package main

import (
	"errors"
	"math"
	"strings"

	"option-tab/internal/actions"
	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/platform"
)

func panelWheelEnabled(input config.DockInputSettings) bool {
	return input.SwipeTowardDock != config.PointerNone || input.SwipeAwayFromDock != config.PointerNone || input.SwipePrevious != config.PointerNone || input.SwipeNext != config.PointerNone
}

func (a *App) setDockPreviewRegions(session, frontendRevision uint64, regions []DockPreviewRegion) error {
	if frontendRevision == 0 {
		return errors.New("invalid dock preview region revision")
	}
	a.viewMu.Lock()
	if session == 0 || session != a.dockState.Session || !a.dockItemAllowedLocked(a.dockState.Item) || a.dockState.Item.Kind == "folder" {
		a.viewMu.Unlock()
		return errStaleDockSession
	}
	admission, edge := a.dockState.AdmissionEpoch, a.dockState.Item.Edge
	if a.dockController != nil && admission != a.dockController.AdmissionEpoch() {
		a.viewMu.Unlock()
		return errStaleDockSession
	}
	valid := make(map[domain.WindowID]domain.AppID, len(a.dockState.Windows))
	for _, window := range a.dockState.Windows {
		valid[window.ID] = window.AppID
	}
	a.viewMu.Unlock()

	copied := make([]platform.PreviewRegion, 0, len(regions))
	for _, region := range regions {
		b := region.Bounds
		if region.WindowID == 0 || region.AppID <= 0 || valid[region.WindowID] != region.AppID || b.W <= 0 || b.H <= 0 || nonfinite(b.X, b.Y, b.W, b.H) {
			return errors.New("invalid dock preview region")
		}
		copied = append(copied, platform.PreviewRegion{WindowID: region.WindowID, AppID: region.AppID, Bounds: domain.Bounds{X: b.X, Y: b.Y, W: b.W, H: b.H}})
	}
	a.dockWheelMu.Lock()
	source := a.dockWheelSource
	a.dockWheelMu.Unlock()
	if source == nil && a.dockWindow != nil {
		source = a.dockWindow
	}
	validator, validSource := source.(platform.DockPanelWheelGestureValidator)
	_, ackSource := source.(platform.DockPanelWheelGestureAcknowledger)
	if source == nil || !validSource || !ackSource || validator == nil {
		return errors.New("dock preview gestures are unavailable")
	}
	s := a.settingsSnapshot()
	enabled := panelWheelEnabled(s.Dock.Input)
	a.dockWheelPublishMu.Lock()
	defer a.dockWheelPublishMu.Unlock()
	if !a.currentDockWheelPresentation(session, admission) {
		return errStaleDockSession
	}
	a.dockWheelMu.Lock()
	if session == a.dockWheelSession && frontendRevision <= a.dockWheelFrontendRevision {
		a.dockWheelMu.Unlock()
		return errStaleDockSession
	}
	a.dockWheelRevision++
	if frontendRevision > a.dockWheelRevision {
		a.dockWheelRevision = frontendRevision
	}
	revision := a.dockWheelRevision
	a.dockWheelSource, a.dockWheelSession, a.dockWheelAdmission = source, session, admission
	a.dockWheelFrontendRevision = frontendRevision
	a.dockWheel = dock.NewPanelGestureRecognizer(edge, s.Dock.Input)
	a.dockWheel.SetPresentation(session, revision)
	a.dockWheelMu.Unlock()
	err := source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{Session: session, Revision: revision, Enabled: enabled, Regions: copied}, a.handleDockPanelWheel)
	if err != nil {
		a.setDockWheelError(err)
		return err
	}
	if !a.currentDockWheelPresentation(session, admission) {
		_ = source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{}, nil)
		a.retireDockWheelState()
		return errStaleDockSession
	}
	return nil
}

func (a *App) currentDockWheelPresentation(session, admission uint64) bool {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if session == 0 || session != a.dockState.Session || !a.dockItemAllowedLocked(a.dockState.Item) || a.dockState.Item.Kind == "folder" || admission != a.dockState.AdmissionEpoch {
		return false
	}
	return a.dockController == nil || admission == a.dockController.AdmissionEpoch()
}

func nonfinite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return true
		}
	}
	return false
}

func (a *App) retireDockWheelState() {
	a.dockWheelMu.Lock()
	a.dockWheelSession, a.dockWheelAdmission = 0, 0
	a.dockWheelFrontendRevision = 0
	if a.dockWheel != nil {
		a.dockWheel.Reset()
	}
	a.dockWheelMu.Unlock()
}

func (a *App) disableDockWheel() {
	s := a.settingsSnapshot()
	if !s.Dock.Enabled || !panelWheelEnabled(s.Dock.Input) {
		a.setDockWheelError(nil)
	}
	a.dockWheelPublishMu.Lock()
	defer a.dockWheelPublishMu.Unlock()
	a.dockWheelMu.Lock()
	source := a.dockWheelSource
	a.dockWheelSession, a.dockWheelAdmission = 0, 0
	a.dockWheelFrontendRevision = 0
	if a.dockWheel != nil {
		a.dockWheel.Reset()
	}
	a.dockWheelMu.Unlock()
	if source != nil {
		if err := source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{}, nil); err != nil && !errors.Is(err, platform.ErrDockPanelHostClosed) {
			a.setDockWheelError(err)
		}
	}
}

func wheelTerminal(event platform.DockPanelWheelEvent) bool {
	return event.Reason != "" || event.Phase == "ended" || event.Phase == "cancelled" || event.MomentumPhase == "ended"
}

func (a *App) handleDockPanelWheel(event platform.DockPanelWheelEvent) {
	a.dockWheelMu.Lock()
	recognizer, source := a.dockWheel, a.dockWheelSource
	intent := (*dock.PanelGestureIntent)(nil)
	current := recognizer != nil && event.Session == a.dockWheelSession && event.Revision == a.dockWheelRevision
	if current {
		intent = recognizer.Step(event)
		if intent != nil {
			a.dockWheelPending.session, a.dockWheelPending.revision, a.dockWheelPending.gesture = intent.Session, intent.Revision, intent.GestureID
		}
	}
	pending := a.dockWheelPending.session == event.Session && a.dockWheelPending.revision == event.Revision && a.dockWheelPending.gesture == event.GestureID
	a.dockWheelMu.Unlock()
	ack, _ := source.(platform.DockPanelWheelGestureAcknowledger)
	if current && event.Reason != "" {
		a.setDockWheelError(errors.New(event.Reason))
	}
	if intent == nil {
		if wheelTerminal(event) && !pending && ack != nil {
			ack.CompleteDockPanelWheelGesture(event.Session, event.Revision, event.GestureID)
		}
		return
	}
	go func() {
		defer func() {
			a.dockWheelMu.Lock()
			if a.dockWheelPending.session == intent.Session && a.dockWheelPending.revision == intent.Revision && a.dockWheelPending.gesture == intent.GestureID {
				a.dockWheelPending = struct{ session, revision, gesture uint64 }{}
			}
			a.dockWheelMu.Unlock()
			ack.CompleteDockPanelWheelGesture(intent.Session, intent.Revision, intent.GestureID)
		}()
		if err := a.executeDockPanelGesture(*intent, source); err != nil && !errors.Is(err, dock.ErrInputRetired) {
			a.setDockWheelError(err)
		}
	}()
}

func (a *App) executeDockPanelGesture(intent dock.PanelGestureIntent, source platform.DockPanelWheelSource) error {
	stateCurrent := func() bool {
		a.viewMu.Lock()
		a.dockWheelMu.Lock()
		admission := a.dockWheelAdmission
		wheelCurrent := a.dockWheelSession == intent.Session && a.dockWheelRevision == intent.Revision
		a.dockWheelMu.Unlock()
		current := wheelCurrent && intent.Session != 0 && intent.Session == a.dockState.Session && a.dockAllowedLocked() && a.dockState.AdmissionEpoch == admission
		if current {
			current = a.dockController == nil || a.dockState.AdmissionEpoch == a.dockController.AdmissionEpoch()
		}
		a.viewMu.Unlock()
		return current
	}
	guard := func() error {
		if !stateCurrent() {
			return dock.ErrInputRetired
		}
		validator, ok := source.(platform.DockPanelWheelGestureValidator)
		if !ok || !validator.ValidateDockPanelWheelGesture(intent.Session, intent.Revision, intent.GestureID) {
			return dock.ErrInputRetired
		}
		if !stateCurrent() {
			return dock.ErrInputRetired
		}
		return nil
	}
	if err := guard(); err != nil {
		return err
	}
	windows, err := a.platform.Windows()
	if err != nil {
		return err
	}
	s := a.settingsSnapshot()
	ctx := filter.Context{ActiveAppID: a.platform.ActiveApp(), ActiveSpaceID: a.platform.ActiveSpace(), ActiveScreenID: a.platform.ActiveScreen(), CursorScreenID: a.platform.CursorScreen(), SelfBundleID: selfBundleID}
	eligible := filter.Apply(windows, s.Filters, s.Dock.Scope, ctx)
	found := false
	for _, window := range eligible {
		if window.ID == intent.WindowID && window.AppID == intent.AppID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("dock preview window is no longer eligible")
	}
	kind := string(intent.Action)
	windowID := intent.WindowID
	if strings.HasSuffix(kind, "app") || kind == "hide" || kind == "quit" {
		windowID = 0
	}
	_, err = actions.New(a.platform).PerformGuarded(kind, windowID, intent.AppID, guard)
	if err == nil {
		a.setDockWheelError(nil)
	}
	return err
}
