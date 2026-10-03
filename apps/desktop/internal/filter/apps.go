package filter

import (
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

// AppAllowed handles filters that do not require a window's geometry. A
// negative rawWindowCount means unknown, and must not be treated as windowless.
func AppAllowed(app domain.App, rawWindowCount int, f config.Filters, scope config.ShortcutScope, ctx Context) bool {
	if app.ID <= 0 || (ctx.SelfBundleID != "" && strings.EqualFold(app.BundleID, ctx.SelfBundleID)) {
		return false
	}
	if scope.AppScope == config.AppScopeActiveApp && ctx.ActiveAppID != app.ID {
		return false
	}
	if app.Hidden && f.ShowHiddenApps == config.VisHide {
		return false
	}
	for _, entry := range f.AppBlacklist {
		if entry.Match == "" || (!strings.EqualFold(entry.Match, app.BundleID) && !strings.EqualFold(entry.Match, app.Name)) {
			continue
		}
		if entry.Hide != config.HideWhenNoWindow || rawWindowCount == 0 {
			return false
		}
	}
	return true
}
