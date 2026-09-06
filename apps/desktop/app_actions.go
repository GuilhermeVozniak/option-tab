package main

import (
	"option-tab/internal/actions"
	"option-tab/internal/domain"
)

// PerformAction acts on the supplied identity without changing the selection.
func (a *App) PerformAction(kind string, windowID uint64, appID int) (actions.Result, error) {
	result, err := actions.New(a.platform).Perform(kind, domain.WindowID(windowID), domain.AppID(appID))
	if a.controller != nil {
		a.controller.Refresh()
	}
	return result, err
}
