package filter

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func win(id domain.WindowID, mut func(*domain.Window)) domain.Window {
	w := domain.Window{
		ID: id, AppID: domain.AppID(id), AppName: "App", BundleID: "com.app",
		Title: "Window", OnScreen: true, SpaceID: 1, ScreenID: 1,
	}
	if mut != nil {
		mut(&w)
	}
	return w
}

func ids(ws []domain.Window) []domain.WindowID {
	out := make([]domain.WindowID, len(ws))
	for i, w := range ws {
		out[i] = w.ID
	}
	return out
}

func has(ws []domain.Window, id domain.WindowID) bool {
	for _, w := range ws {
		if w.ID == id {
			return true
		}
	}
	return false
}

func baseCtx() Context {
	return Context{ActiveAppID: 1, ActiveSpaceID: 1, ActiveScreenID: 1, CursorScreenID: 1, SelfBundleID: "com.option-tab"}
}

func TestApply_ExcludesSelf(t *testing.T) {
	f := config.Default().Filters
	ws := []domain.Window{
		win(1, nil),
		win(2, func(w *domain.Window) { w.BundleID = "com.option-tab" }),
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if has(got, 2) {
		t.Error("self window should be excluded")
	}
	if !has(got, 1) {
		t.Error("normal window should be kept")
	}
}

func TestApply_Blacklist(t *testing.T) {
	f := config.Default().Filters
	f.AppBlacklist = []config.BlacklistEntry{
		{Match: "com.banned", Hide: config.HideAlways},
		{Match: "Finder", Hide: config.HideAlways},
	}
	ws := []domain.Window{
		win(1, nil),
		win(2, func(w *domain.Window) { w.BundleID = "com.banned" }),
		win(3, func(w *domain.Window) { w.BundleID = "com.other"; w.AppName = "finder" }), // name match, case-insensitive
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if has(got, 2) || has(got, 3) {
		t.Errorf("blacklisted apps not excluded: %v", ids(got))
	}
}

func TestApply_TitleMinimizedHiddenFullscreenToggles(t *testing.T) {
	f := config.Default().Filters
	f.ShowWindowsWithoutTitle = false
	f.ShowMinimized = config.VisHide
	f.ShowHiddenApps = config.VisHide
	f.ShowFullscreen = config.VisHide
	ws := []domain.Window{
		win(1, nil),
		win(2, func(w *domain.Window) { w.Title = "" }),
		win(3, func(w *domain.Window) { w.Minimized = true; w.OnScreen = false }),
		win(4, func(w *domain.Window) { w.Hidden = true; w.OnScreen = false }),
		win(5, func(w *domain.Window) { w.Fullscreen = true }),
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) {
		t.Error("plain window should be kept")
	}
	for _, id := range []domain.WindowID{2, 3, 4, 5} {
		if has(got, id) {
			t.Errorf("window %d should be filtered out", id)
		}
	}
}

func TestApply_MinimizedKeptWhenEnabled(t *testing.T) {
	f := config.Default().Filters // ShowMinimized true by default
	ws := []domain.Window{win(3, func(w *domain.Window) { w.Minimized = true; w.OnScreen = false; w.SpaceID = 99 })}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 3) {
		t.Error("minimized window should be kept (and bypass space filter)")
	}
}

func TestApply_ActiveSpaceScope(t *testing.T) {
	f := config.Default().Filters
	f.Spaces = config.SpacesActive
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.SpaceID = 1 }),
		win(2, func(w *domain.Window) { w.SpaceID = 2 }),
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) || has(got, 2) {
		t.Errorf("active-space scope wrong: %v", ids(got))
	}
}

func TestApply_ActiveScreenScope(t *testing.T) {
	f := config.Default().Filters
	f.Screens = config.ScreensActive
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.ScreenID = 1 }),
		win(2, func(w *domain.Window) { w.ScreenID = 2 }),
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) || has(got, 2) {
		t.Errorf("active-screen scope wrong: %v", ids(got))
	}
}

func TestApply_CursorScreenScope(t *testing.T) {
	f := config.Default().Filters
	f.Screens = config.ScreensCursor
	ctx := baseCtx()
	ctx.CursorScreenID = 2
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.ScreenID = 1 }),
		win(2, func(w *domain.Window) { w.ScreenID = 2 }),
	}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, ctx)
	if has(got, 1) || !has(got, 2) {
		t.Errorf("cursor-screen scope wrong: %v", ids(got))
	}
}

func TestApply_ShortcutScopeOverridesGlobal(t *testing.T) {
	f := config.Default().Filters
	f.Spaces = config.SpacesAll // global says all spaces
	scope := config.ShortcutScope{AppScope: config.AppScopeAll, Spaces: config.SpacesActive}
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.SpaceID = 1 }),
		win(2, func(w *domain.Window) { w.SpaceID = 2 }),
	}
	got := Apply(ws, f, scope, baseCtx())
	if !has(got, 1) || has(got, 2) {
		t.Errorf("per-shortcut space override should win: %v", ids(got))
	}
}

func TestApply_ActiveAppScope(t *testing.T) {
	f := config.Default().Filters
	scope := config.ShortcutScope{AppScope: config.AppScopeActiveApp}
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.AppID = 1 }), // active app
		win(2, func(w *domain.Window) { w.AppID = 2 }),
	}
	got := Apply(ws, f, scope, baseCtx())
	if !has(got, 1) || has(got, 2) {
		t.Errorf("active-app scope wrong: %v", ids(got))
	}
}

func TestApply_DoesNotMutateInput(t *testing.T) {
	f := config.Default().Filters
	ws := []domain.Window{win(1, nil), win(2, func(w *domain.Window) { w.BundleID = "com.option-tab" })}
	_ = Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if len(ws) != 2 {
		t.Error("Apply must not mutate the input slice length")
	}
}

func TestApply_ShowAtEndKeepsWindowVisible(t *testing.T) {
	f := config.Default().Filters
	f.ShowMinimized = config.VisShowAtEnd
	ws := []domain.Window{win(1, func(w *domain.Window) { w.Minimized = true; w.OnScreen = false })}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) {
		t.Error("showAtEnd windows must pass the filter (ordering happens later)")
	}
}

func TestApply_BlacklistWhenNoWindowDoesNotHideRealWindows(t *testing.T) {
	f := config.Default().Filters
	f.AppBlacklist = []config.BlacklistEntry{{Match: "com.app", Hide: config.HideWhenNoWindow}}
	ws := []domain.Window{win(1, nil)}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) {
		t.Error("whenNoWindow entries must not hide an app's real windows")
	}
}

func TestApply_ShortcutScreenScopeOverridesGlobal(t *testing.T) {
	f := config.Default().Filters
	f.Screens = config.ScreensAll // global says all screens
	scope := config.ShortcutScope{AppScope: config.AppScopeAll, Screens: config.ScreensActive}
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.ScreenID = 1 }),
		win(2, func(w *domain.Window) { w.ScreenID = 2 }),
	}
	got := Apply(ws, f, scope, baseCtx())
	if !has(got, 1) || has(got, 2) {
		t.Errorf("per-shortcut screen override should win: %v", ids(got))
	}
}

func TestApply_HiddenBypassesSpaceScope(t *testing.T) {
	f := config.Default().Filters
	f.Spaces = config.SpacesActive
	f.ShowHiddenApps = config.VisShow
	ws := []domain.Window{win(1, func(w *domain.Window) { w.Hidden = true; w.OnScreen = false; w.SpaceID = 99 })}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) {
		t.Error("hidden window should be kept (and bypass space filter)")
	}
}

func TestApply_UntitledWindows(t *testing.T) {
	f := config.Default().Filters
	f.ShowWindowsWithoutTitle = true
	ws := []domain.Window{win(1, func(w *domain.Window) { w.Title = "" })}
	got := Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if !has(got, 1) {
		t.Error("empty-title window should be kept when ShowWindowsWithoutTitle is true")
	}

	f.ShowWindowsWithoutTitle = false
	ws = []domain.Window{win(2, func(w *domain.Window) { w.Title = "  " })} // whitespace-only trims to empty
	got = Apply(ws, f, config.ShortcutScope{AppScope: config.AppScopeAll}, baseCtx())
	if has(got, 2) {
		t.Error("whitespace-only title should be dropped when ShowWindowsWithoutTitle is false")
	}
}

func TestShortcutIgnoredForApp(t *testing.T) {
	bl := []config.BlacklistEntry{{Match: "com.game", Hide: config.HideWhenNoWindow, IgnoreShortcuts: true}}
	ws := []domain.Window{
		win(1, func(w *domain.Window) { w.AppID = 7; w.BundleID = "com.game" }),
		win(2, nil),
	}
	if !ShortcutIgnoredForApp(ws, 7, bl) {
		t.Error("active blacklisted app with IgnoreShortcuts should suppress activation")
	}
	if ShortcutIgnoredForApp(ws, 2, bl) {
		t.Error("other active apps should not suppress activation")
	}
	if ShortcutIgnoredForApp(ws, 7, []config.BlacklistEntry{{Match: "com.game", Hide: config.HideAlways}}) {
		t.Error("entries without IgnoreShortcuts should not suppress activation")
	}
}
