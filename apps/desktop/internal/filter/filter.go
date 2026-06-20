// Package filter selects which windows the switcher should display, given the
// global Filters, a per-shortcut Scope override, and the current Context
// (active app/space/screen, cursor screen, and the switcher's own bundle id).
// It is pure and does not mutate its inputs.
package filter

import (
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

// Context carries the runtime facts the filters need.
type Context struct {
	ActiveAppID    domain.AppID
	ActiveSpaceID  domain.SpaceID
	ActiveScreenID domain.ScreenID
	CursorScreenID domain.ScreenID
	SelfBundleID   string // the switcher's own bundle id, always excluded
}

// Apply returns a new slice of the windows that pass all active filters.
// Minimized and hidden windows (when their respective "show" toggles are on)
// bypass the space/screen scoping, matching AltTab: a minimized window has no
// meaningful current space/screen, so it should still appear.
func Apply(wins []domain.Window, f config.Filters, scope config.ShortcutScope, ctx Context) []domain.Window {
	spaceScope := f.Spaces
	if scope.Spaces.Valid() {
		spaceScope = scope.Spaces
	}
	screenScope := f.Screens
	if scope.Screens.Valid() {
		screenScope = scope.Screens
	}

	out := make([]domain.Window, 0, len(wins))
	for _, w := range wins {
		if ctx.SelfBundleID != "" && strings.EqualFold(w.BundleID, ctx.SelfBundleID) {
			continue
		}
		if isBlacklisted(w, f.AppBlacklist) {
			continue
		}
		if !f.ShowWindowsWithoutTitle && strings.TrimSpace(w.Title) == "" {
			continue
		}
		if !f.ShowMinimized && w.Minimized {
			continue
		}
		if !f.ShowHiddenApps && w.Hidden {
			continue
		}
		if !f.ShowFullscreen && w.Fullscreen {
			continue
		}
		if scope.AppScope == config.AppScopeActiveApp && w.AppID != ctx.ActiveAppID {
			continue
		}

		// Minimized/hidden windows bypass space & screen scoping.
		offScreenSpecial := w.Minimized || w.Hidden
		if !offScreenSpecial {
			if !inSpaceScope(w, spaceScope, ctx) || !inScreenScope(w, screenScope, ctx) {
				continue
			}
		}
		out = append(out, w)
	}
	return out
}

func isBlacklisted(w domain.Window, blacklist []string) bool {
	for _, entry := range blacklist {
		if entry == "" {
			continue
		}
		if strings.EqualFold(entry, w.BundleID) || strings.EqualFold(entry, w.AppName) {
			return true
		}
	}
	return false
}

func inSpaceScope(w domain.Window, scope config.SpaceScope, ctx Context) bool {
	if scope == config.SpacesActive {
		return w.SpaceID == ctx.ActiveSpaceID
	}
	return true // SpacesAll (or empty/unknown)
}

func inScreenScope(w domain.Window, scope config.ScreenScope, ctx Context) bool {
	switch scope {
	case config.ScreensActive:
		return w.ScreenID == ctx.ActiveScreenID
	case config.ScreensCursor:
		return w.ScreenID == ctx.CursorScreenID
	default:
		return true // ScreensAll (or empty/unknown)
	}
}
