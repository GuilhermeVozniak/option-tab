package main

import (
	"errors"

	"option-tab/internal/actions"
	"option-tab/internal/domain"
)

// PerformAction acts on the supplied identity without changing the selection.
func (a *App) PerformAction(kind string, windowID uint64, appID int) (actions.Result, error) {
	guard := func() error {
		select {
		case <-a.captureStop:
			return errors.New("application is shutting down")
		default:
			return nil
		}
	}
	if err := guard(); err != nil {
		return actions.Result{}, err
	}
	result, err := actions.New(a.platform).PerformGuarded(kind, domain.WindowID(windowID), domain.AppID(appID), guard)
	if a.controller != nil {
		a.controller.Refresh()
	}
	return result, err
}
