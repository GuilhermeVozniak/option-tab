package main

import (
	"cmp"
	"slices"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

type LauncherAppChoice struct {
	Name     string `json:"name"`
	BundleID string `json:"bundleID"`
}

func (a *App) launcherChoicesAllowedLocked() bool {
	select {
	case <-a.captureStop:
		return false
	default:
		return a.prefsOpen && !a.sessionInactive
	}
}

// GetLauncherAppChoices is an explicit settings inventory. It performs no
// window query, permission prompt or app activation and exposes no process IDs.
func (a *App) GetLauncherAppChoices() []LauncherAppChoice {
	out := []LauncherAppChoice{}
	a.viewMu.Lock()
	allowed := a.launcherChoicesAllowedLocked()
	a.viewMu.Unlock()
	source, ok := a.platform.(platform.ApplicationSource)
	if !allowed || !ok {
		return out
	}
	apps, err := source.Apps()
	if err != nil {
		return out
	}
	seen := map[string]bool{}
	for _, app := range apps[:min(len(apps), 1024)] {
		bundle := app.BundleID
		if app.ID <= 0 || bundle == selfBundleID || seen[bundle] || !config.ValidLauncherBundleID(bundle) {
			continue
		}
		name := strings.TrimSpace(app.Name)
		if name == "" {
			name = bundle
		}
		runes := []rune(name)
		name = string(runes[:min(len(runes), 80)])
		seen[bundle] = true
		out = append(out, LauncherAppChoice{Name: name, BundleID: bundle})
	}
	slices.SortFunc(out, func(a, b LauncherAppChoice) int {
		if order := cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return cmp.Compare(a.BundleID, b.BundleID)
	})
	a.viewMu.Lock()
	allowed = a.launcherChoicesAllowedLocked()
	a.viewMu.Unlock()
	if !allowed {
		return []LauncherAppChoice{}
	}
	return out[:min(len(out), 128)]
}
