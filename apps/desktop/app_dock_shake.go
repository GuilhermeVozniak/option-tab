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
	"option-tab/internal/platform"
)

func (a *App) wireDockShake() {
	source, ok := a.platform.(platform.WindowDragObservationSource)
	if !ok {
		return
	}
	if _, ok := source.(platform.WindowDragGestureValidator); !ok {
		return
	}
	if _, ok := a.platform.(platform.WindowRoleSource); !ok {
		return
	}
	if _, ok := a.platform.(platform.OtherWindowPerformer); !ok {
		return
	}
	if _, ok := a.platform.(platform.GuardedOtherWindowPerformer); !ok {
		return
	}
	a.dockShake = dock.NewShakeController(dock.ShakeControllerDeps{
		Source: source, SelfAppID: domain.AppID(os.Getpid()), Execute: a.executeDockShake,
		Failed: func(_ dock.ShakeIntent, err error) { a.setDockShakeError(err) }, SourceFailed: a.setDockShakeError,
	})
	a.viewMu.Lock()
	a.syncDockShakeLocked()
	a.viewMu.Unlock()
}

func dockShakeEnabled(s config.Settings) bool {
	return s.Dock.Enabled && !s.Behavior.Paused && (s.Dock.Input.AeroShakeAction == "closeOthers" || s.Dock.Input.AeroShakeAction == "minimizeOthers")
}

func (a *App) shakeAllowedLocked(kind string) bool {
	select {
	case <-a.captureStop:
		return false
	default:
	}
	s := a.settingsSnapshot()
	return !a.sessionInactive && dockShakeEnabled(s) && (kind == "" || kind == s.Dock.Input.AeroShakeAction)
}

func (a *App) syncDockShakeLocked() {
	if a.dockShake != nil {
		s := a.settingsSnapshot()
		a.dockShake.Configure(a.shakeAllowedLocked(""), s.Dock.Input.AeroShakeAction)
		if !dockShakeEnabled(s) {
			a.setDockFeatureErrorLocked(dockErrorShake, nil)
		}
	}
}

func (a *App) executeDockShake(intent dock.ShakeIntent, nativeGuard func() error) error {
	if nativeGuard == nil {
		return dock.ErrShakeRetired
	}
	current := func() bool {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		return a.shakeAllowedLocked(intent.Kind)
	}
	guard := func() error {
		if !current() {
			return dock.ErrShakeRetired
		}
		if err := nativeGuard(); err != nil {
			return err
		}
		if !current() {
			return dock.ErrShakeRetired
		}
		return nil
	}
	result, err := actions.New(a.platform).PerformOthersGuarded(intent.Kind, intent.WindowID, intent.AppID, guard)
	if len(result.Failures) > 0 {
		messages := make([]string, 0, min(3, len(result.Failures)))
		for _, failure := range result.Failures[:min(3, len(result.Failures))] {
			messages = append(messages, fmt.Sprintf("window %d: %s", failure.WindowID, failure.Error))
		}
		failure := fmt.Errorf("%d windows changed; %d failed: %s", result.Succeeded, len(result.Failures), strings.Join(messages, "; "))
		return errors.Join(err, failure)
	}
	if err == nil {
		a.setDockShakeError(nil)
	}
	return err
}
