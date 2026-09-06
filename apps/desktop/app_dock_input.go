package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"option-tab/internal/actions"
	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/platform"
)

func (a *App) wireDockInput() {
	source, ok := a.platform.(platform.DockInputSource)
	if !ok {
		return
	}
	if _, ok = source.(platform.DockInputGestureValidator); !ok {
		return
	}
	if _, ok = source.(platform.DockInputGestureAcknowledger); !ok {
		return
	}
	if _, ok = a.platform.(platform.ApplicationSource); !ok {
		return
	}
	a.dockInput = dock.NewInputController(dock.InputControllerDeps{
		Source: source, SelfAppID: domain.AppID(os.Getpid()),
		Execute:      a.executeDockInput,
		Failed:       func(_ dock.InputAction, err error) { a.setDockInputError(err) },
		SourceFailed: a.setDockInputError,
	})
	// startup starts the owner loop; configuration is safe before Run.
	a.syncDockInput()
}

func dockInputPolicy(s config.Settings) platform.DockInputPolicy {
	return platform.DockInputPolicy{ClickToHide: s.Dock.Input.ClickToHide, ScrollShowHide: s.Dock.Input.ScrollShowHide, ModifiedRightClick: s.Dock.Input.ModifiedRightClick}
}

func dockInputEnabled(s config.Settings) bool {
	p := dockInputPolicy(s)
	return s.Dock.Enabled && !s.Behavior.Paused && (p.ClickToHide || p.ScrollShowHide || p.ModifiedRightClick)
}

func (a *App) syncDockInput() {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncDockInputLocked()
}

func (a *App) syncDockInputLocked() {
	if a.dockInput == nil {
		return
	}
	s := a.settingsSnapshot()
	enabled := dockInputEnabled(s) && !a.switcherVisible && !a.prefsOpen && !a.sessionInactive
	a.dockInput.Configure(enabled, dockInputPolicy(s))
	if !dockInputEnabled(s) {
		a.setDockInputErrorLocked(nil)
	}
}

func (a *App) setDockInputTarget(epoch uint64, target platform.DockInputTarget) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.dockInput == nil || a.dockController == nil || epoch == 0 || epoch != a.dockController.AdmissionEpoch() || (target.Generation != 0 && !a.dockAllowedLocked()) {
		return
	}
	a.dockInput.Target(target)
}

func (a *App) executeDockInput(action dock.InputAction, nativeGuard func() error) error {
	a.viewMu.Lock()
	controller := a.dockController
	var admission uint64
	if controller != nil {
		admission = controller.AdmissionEpoch()
	}
	a.viewMu.Unlock()
	guard := func() error {
		if err := nativeGuard(); err != nil {
			return err
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !a.dockAllowedLocked() || a.dockController != controller || (controller != nil && controller.AdmissionEpoch() != admission) {
			return dock.ErrInputRetired
		}
		return nil
	}
	if err := guard(); err != nil {
		return err
	}
	if action.Intent.AppID <= 0 || action.Intent.AppID != action.Item.AppID {
		return errors.New("dock input application identity mismatches")
	}
	source := a.platform.(platform.ApplicationSource)
	apps, err := source.Apps()
	if err != nil {
		return fmt.Errorf("refresh application identity: %w", err)
	}
	var current *domain.App
	for i := range apps {
		if apps[i].ID == action.Item.AppID && strings.EqualFold(apps[i].BundleID, action.Item.BundleID) {
			copy := apps[i]
			current = &copy
			break
		}
	}
	if current == nil {
		return errors.New("dock application is no longer running")
	}
	s := a.settingsSnapshot()
	ctx := filter.Context{ActiveAppID: a.platform.ActiveApp(), ActiveSpaceID: a.platform.ActiveSpace(), ActiveScreenID: a.platform.ActiveScreen(), CursorScreenID: a.platform.CursorScreen(), SelfBundleID: selfBundleID}
	if !filter.AppAllowed(*current, -1, s.Filters, s.Dock.Scope, ctx) {
		return errors.New("dock application is excluded by current settings")
	}
	if action.Intent.Kind == "show" {
		activator, ok := a.platform.(platform.ApplicationActivator)
		if !ok {
			return errors.New("show application is unsupported by this platform")
		}
		if err := guard(); err != nil {
			return err
		}
		err = activator.ActivateApp(current.ID)
	} else {
		_, err = actions.New(a.platform).PerformGuarded(action.Intent.Kind, 0, current.ID, guard)
	}
	if err == nil {
		a.setDockInputError(nil)
	}
	return err
}

func (a *App) setDockInputError(err error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.setDockInputErrorLocked(err)
}

func (a *App) setDockInputErrorLocked(err error) {
	a.setDockFeatureErrorLocked(dockErrorIcon, err)
}

type dockErrorSource int

const (
	dockErrorIcon dockErrorSource = iota
	dockErrorWheel
	dockErrorDrag
	dockErrorShake
)

func (a *App) setDockWheelError(err error) { a.setDockFeatureError(dockErrorWheel, err) }
func (a *App) setDockDragError(err error)  { a.setDockFeatureError(dockErrorDrag, err) }
func (a *App) setDockShakeError(err error) { a.setDockFeatureError(dockErrorShake, err) }

func (a *App) setDockFeatureError(source dockErrorSource, err error) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.setDockFeatureErrorLocked(source, err)
}

func (a *App) setDockFeatureErrorLocked(source dockErrorSource, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	a.dockFeatureErrors[source] = message
	var messages []string
	seen := make(map[string]bool, len(a.dockFeatureErrors))
	for _, failure := range a.dockFeatureErrors {
		if failure != "" && !seen[failure] {
			messages = append(messages, failure)
			seen[failure] = true
		}
	}
	message = strings.Join(messages, "; ")
	if message == a.dockInputError {
		return
	}
	a.dockInputError = message
	a.dockRevision++
	a.emit("dock:input-status", struct {
		Revision uint64 `json:"revision"`
		Message  string `json:"message"`
	}{a.dockRevision, message})
	if a.dockState.Session == 0 {
		return
	}
	a.dockViewState.Error = message
	a.dockViewState.Revision = a.dockRevision
	a.emit("dock:error", struct {
		Session  uint64 `json:"session"`
		Revision uint64 `json:"revision"`
		Message  string `json:"message"`
	}{a.dockState.Session, a.dockRevision, message})
}
