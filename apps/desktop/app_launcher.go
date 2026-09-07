package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type LauncherStatus struct {
	Epoch           uint64                  `json:"epoch"`
	Revision        uint64                  `json:"revision"`
	Enabled         bool                    `json:"enabled"`
	Status          string                  `json:"status"`
	Reason          string                  `json:"reason"`
	RecoveryLatched bool                    `json:"recoveryLatched"`
	Displays        []launcher.DisplayState `json:"displays"`
	ClockPackageID  string                  `json:"clockPackageID"`
	ClockDigest     string                  `json:"clockDigest"`
}

type appLauncherHost struct {
	window       *dockWindow
	presentation launcher.Presentation
}

// Mutable fields are protected by App.viewMu; native work never runs under it.
type appLauncherRuntime struct {
	core                      *launcher.Controller
	once                      sync.Once
	started, available, ready bool
	recovery, pointerOwned    bool
	reason                    string
	generation                uint64
	statusRevision            uint64
	configuration             config.ReplacementDockSettings
	drainCancel, cancel       context.CancelFunc
	hosts                     map[uint64]*appLauncherHost
}

type appLauncherView struct{ app *App }

func (v appLauncherView) Publish(state launcher.State) { v.app.publishLauncher(state) }

func (a *App) wireLauncher() {
	r := &appLauncherRuntime{hosts: map[uint64]*appLauncherHost{}}
	environment, envOK := a.platform.(platform.LauncherEnvironmentSource)
	apps, appsOK := a.platform.(platform.ApplicationSource)
	identities, identityOK := a.platform.(launcher.Identities)
	_, activationOK := a.platform.(platform.LauncherAppActivator)
	r.available = envOK && appsOK && identityOK && activationOK
	r.core = launcher.New(launcher.Deps{
		Environment: environment, Applications: apps, Identities: identities,
		View: appLauncherView{a}, Activate: a.activateLauncherTarget,
		SelfAppID: domain.AppID(os.Getpid()), SelfBundleID: selfBundleID, Eligible: a.launcherAppEligible,
	})
	a.launcher = r
	r.core.Suspend(true)
	r.configuration = a.settingsSnapshot().ReplacementDock
	if err := r.core.Configure(r.configuration); err != nil {
		r.reason = "invalidSettings"
	}
	a.viewMu.Lock()
	a.syncDockInputLocked()
	a.syncDockMonitorLockLocked()
	a.viewMu.Unlock()
}

func (a *App) startLauncher() {
	if a.launcher == nil {
		return
	}
	a.launcher.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		a.viewMu.Lock()
		a.launcher.started = true
		a.launcher.cancel = cancel
		a.syncLauncherLocked()
		a.viewMu.Unlock()
		go func() {
			select {
			case <-a.captureStop:
				cancel()
			case <-ctx.Done():
			}
		}()
		go func() { _ = a.launcher.core.Run(ctx) }()
	})
}

func (a *App) launcherWantedLocked() bool {
	return a.launcher != nil && !a.launcher.recovery && a.settingsSnapshot().ReplacementDock.Enabled
}

func (a *App) launcherAllowedLocked() bool {
	select {
	case <-a.captureStop:
		return false
	default:
	}
	s := a.settingsSnapshot()
	return a.launcherWantedLocked() && a.launcher.started && a.launcher.available && a.launcherFactory != nil &&
		reflect.DeepEqual(a.launcher.configuration, s.ReplacementDock) && !s.Behavior.Paused && !a.sessionInactive && !a.prefsOpen && !a.switcherVisible
}

func (a *App) retireLauncherHostsLocked() {
	if a.launcher == nil {
		return
	}
	for session := range a.launcher.hosts {
		a.retireLauncherHostLocked(session)
	}
	a.launcher.pointerOwned = false
}

func (a *App) retireLauncherHostLocked(session uint64) {
	h := a.launcher.hosts[session]
	if h == nil {
		return
	}
	delete(a.launcher.hosts, session)
	p := h.presentation
	p.Visible = false
	p.Revision++
	p.Items, p.Widgets = []launcher.Item{}, []launcher.Widget{}
	h.window.close()
	a.emit("launcher:state", p)
}

func (a *App) stopLauncherAdmissionLocked() {
	r := a.launcher
	if r == nil {
		return
	}
	r.generation++
	r.ready = false
	if r.drainCancel != nil {
		r.drainCancel()
		r.drainCancel = nil
	}
	r.core.Suspend(true)
	a.retireLauncherHostsLocked()
}

// Input owners stop synchronously admitting actions; their native lifetimes
// drain before any launcher host may show. Waiting never holds viewMu/AppKit.
func (a *App) syncLauncherLocked() {
	r := a.launcher
	if r == nil {
		return
	}
	if !a.launcherAllowedLocked() {
		a.stopLauncherAdmissionLocked()
		return
	}
	if r.ready {
		r.core.Suspend(false)
		return
	}
	if r.drainCancel != nil {
		return
	}
	r.core.Suspend(true)
	ctx, cancel := context.WithCancel(context.Background())
	r.drainCancel = cancel
	r.generation++
	generation := r.generation
	var receipts []<-chan struct{}
	if a.dockInput != nil {
		receipts = append(receipts, a.dockInput.RetireSource())
	}
	if a.dockMonitorLock != nil {
		receipts = append(receipts, a.dockMonitorLock.RetireSource())
	}
	go func() {
		for _, receipt := range receipts {
			select {
			case <-receipt:
			case <-ctx.Done():
				return
			case <-a.captureStop:
				return
			}
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if ctx.Err() != nil || a.launcher != r || r.generation != generation || !a.launcherAllowedLocked() {
			return
		}
		r.drainCancel = nil
		cancel()
		r.ready = true
		r.core.Suspend(false)
	}()
}

func (a *App) configureLauncher(previous, next config.Settings) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcher == nil {
		return
	}
	if !previous.ReplacementDock.Enabled && next.ReplacementDock.Enabled {
		a.launcher.recovery = false
		a.launcher.reason = ""
	}
	a.stopLauncherAdmissionLocked()
	if err := a.launcher.core.Configure(next.ReplacementDock); err != nil {
		a.launcher.reason = "invalidSettings"
		return
	}
	a.launcher.configuration = config.CloneReplacementDock(next.ReplacementDock)
	a.syncLauncherLocked()
}

func (a *App) launcherStatusLocked() LauncherStatus {
	status := LauncherStatus{Status: "unavailable", Displays: []launcher.DisplayState{}, ClockPackageID: config.BuiltinClockPackage, ClockDigest: config.BuiltinClockDigest}
	if a.launcher == nil {
		return status
	}
	r := a.launcher
	s := r.core.Snapshot()
	status.Epoch, status.Status = s.Epoch, s.Status
	status.Revision = r.statusRevision
	status.Enabled = a.settingsSnapshot().ReplacementDock.Enabled
	status.Displays = slices.Clone(s.Displays)
	status.RecoveryLatched, status.Reason = r.recovery, r.reason
	if !a.launcherWantedLocked() {
		status.Status = "disabled"
	} else if !r.available || a.launcherFactory == nil {
		status.Status = "unavailable"
	} else if !a.launcherAllowedLocked() {
		status.Status = "suspended"
	} else if !r.ready {
		status.Status = "preparing"
	}
	return status
}

func (a *App) GetLauncherStatus() LauncherStatus {
	a.viewMu.Lock()
	status := a.launcherStatusLocked()
	inactive := a.sessionInactive
	a.viewMu.Unlock()
	// Explicit settings queries can list displays while the launcher is disabled.
	// This uses only the existing static topology reader, never an input owner.
	var topology []launcher.DisplayState
	if len(status.Displays) == 0 && !inactive {
		if displays, err := a.GetDockMonitorLockDisplays(); err == nil {
			for _, d := range displays {
				topology = append(topology, launcher.DisplayState{UUID: d.UUID, Name: d.Name, Main: d.Main, SpaceKind: "unknown", Status: "unbound"})
			}
		}
	}
	a.viewMu.Lock()
	status = a.launcherStatusLocked()
	if len(status.Displays) == 0 && !a.sessionInactive {
		status.Displays = append([]launcher.DisplayState{}, topology...)
	}
	a.viewMu.Unlock()
	return status
}

func (a *App) emitLauncherStatusLocked() {
	if a.launcher != nil {
		a.launcher.statusRevision++
	}
	a.emit("launcher:status", a.launcherStatusLocked())
}

func (a *App) GetLauncherState(session uint64) launcher.Presentation {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if session != 0 && a.launcherAllowedLocked() && a.launcher.ready {
		if h := a.launcher.hosts[session]; h != nil {
			for _, p := range a.launcher.core.Snapshot().Presentations {
				if p.Scope == h.presentation.Scope && p.Visible {
					return p
				}
			}
		}
	}
	return launcher.Presentation{Scope: launcher.Scope{Session: session}, Items: []launcher.Item{}, Widgets: []launcher.Widget{}}
}

func (a *App) publishLauncher(state launcher.State) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	r := a.launcher
	if r == nil || state.Epoch != r.core.Snapshot().Epoch {
		return
	}
	current := map[launcher.Scope]bool{}
	for _, p := range r.core.Snapshot().Presentations {
		current[p.Scope] = p.Visible
	}
	wanted := map[uint64]bool{}
	if a.launcherAllowedLocked() && r.ready {
		for _, p := range state.Presentations {
			if !p.Visible || !current[p.Scope] || p.Session == 0 {
				continue
			}
			wanted[p.Session] = true
			h := r.hosts[p.Session]
			if h == nil {
				var window *dockWindow
				window = a.launcherFactory(p.Session, p.DisplayUUID, func() { a.launcherHostFailed(p.Session, window) })
				if window == nil {
					r.core.FailDisplay(p.Scope, "hostUnavailable")
					continue
				}
				h = &appLauncherHost{window: window}
				r.hosts[p.Session] = h
			}
			if !h.presentation.Visible || h.presentation.Bounds != p.Bounds {
				h.window.show(p.Bounds)
			}
			h.presentation = p
			a.emit("launcher:state", p)
		}
	}
	for session := range r.hosts {
		if !wanted[session] {
			a.retireLauncherHostLocked(session)
		}
	}
	r.pointerOwned = state.PointerOwned && a.launcherAllowedLocked() && r.ready && len(r.hosts) != 0
	a.syncNativeHoverForLauncherLocked()
	a.emitLauncherStatusLocked()
}

func (a *App) syncNativeHoverForLauncherLocked() {
	if a.dockController != nil {
		a.dockController.Suspend(a.switcherVisible || a.prefsOpen || a.sessionInactive || (a.launcher != nil && a.launcher.pointerOwned))
	}
	if !a.dockItemAllowedLocked(a.dockState.Item) {
		a.dismissDockLocked()
	}
}

func (a *App) launcherHostFailed(session uint64, window *dockWindow) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcher == nil {
		return
	}
	h := a.launcher.hosts[session]
	if h == nil || h.window != window {
		return
	}
	a.launcher.core.FailDisplay(h.presentation.Scope, "hostUnavailable")
	a.retireLauncherHostLocked(session)
}

func (a *App) launcherAppEligible(app domain.App) bool {
	if app.ID <= 0 || app.ID == domain.AppID(os.Getpid()) || app.BundleID == selfBundleID {
		return false
	}
	for _, entry := range a.settingsSnapshot().Filters.AppBlacklist {
		if entry.Hide == config.HideAlways && entry.Match != "" && (strings.EqualFold(app.BundleID, entry.Match) || strings.EqualFold(app.Name, entry.Match)) {
			return false
		}
	}
	return true
}

func (a *App) ActivateLauncherItem(epoch uint64, displayUUID string, session, revision uint64, itemID string) error {
	a.viewMu.Lock()
	allowed := a.launcherAllowedLocked() && a.launcher.ready
	r := a.launcher
	a.viewMu.Unlock()
	if !allowed || r == nil {
		return launcher.ErrRetired
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.core.Activate(ctx, launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: revision}, itemID)
}

func (a *App) activateLauncherTarget(ctx context.Context, scope launcher.Scope, target platform.LauncherAppTarget, controllerGuard func() error) error {
	var owner *appLauncherHost
	var token uint64
	guard := func() error {
		if err := controllerGuard(); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if ctx.Err() != nil || !a.launcherAllowedLocked() || !a.launcher.ready || !a.launcherAppEligible(domain.App{ID: target.Process.PID, Name: target.Name, BundleID: target.BundleID}) {
			return launcher.ErrRetired
		}
		h := a.launcher.hosts[scope.Session]
		if h == nil || h.presentation.Scope != scope || !h.presentation.Visible || (owner != nil && owner != h) {
			return launcher.ErrRetired
		}
		panel, ok := h.window.wheelPanel().(platform.LauncherPanel)
		if !ok || panel.LauncherToken() == 0 || (token != 0 && token != panel.LauncherToken()) {
			return launcher.ErrRetired
		}
		owner, token = h, panel.LauncherToken()
		return nil
	}
	if err := guard(); err != nil {
		return err
	}
	source, ok := a.platform.(platform.LauncherAppActivator)
	if !ok {
		return launcher.ErrUnavailable
	}
	target.PanelToken = token
	return source.ActivateLauncherApp(ctx, target, guard)
}

// Recovery changes runtime admission before persistence. A failed save leaves
// the launcher disabled until recovery succeeds and the user enables it again.
func (a *App) UseNativeDock() error {
	a.viewMu.Lock()
	if a.launcher != nil {
		a.launcher.recovery = true
		a.launcher.reason = ""
		a.stopLauncherAdmissionLocked()
		a.syncDockSuspensionLocked()
	}
	a.viewMu.Unlock()
	a.saveMu.Lock()
	s := a.settingsSnapshot()
	s.ReplacementDock.Enabled = false
	err := a.saveSettingsLocked(s)
	a.saveMu.Unlock()
	a.viewMu.Lock()
	if err != nil && a.launcher != nil {
		a.launcher.reason = "saveFailed"
	}
	a.emitLauncherStatusLocked()
	a.viewMu.Unlock()
	if err != nil {
		return errors.New("launcher: could not save native Dock recovery; the launcher remains disabled")
	}
	return nil
}

type launcherPanelHostAdapter struct {
	source platform.LauncherPanelHost
	uuid   string
}

func (h launcherPanelHostAdapter) CreateDockPanel(host unsafe.Pointer) (platform.DockPanel, error) {
	return h.source.CreateLauncherPanel(host, h.uuid)
}
