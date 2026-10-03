package filter

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func TestAppFilterUsesActualWindowPresence(t *testing.T) {
	app := domain.App{ID: 10, Name: "Editor", BundleID: "org.test.editor"}
	f := config.Default().Filters
	f.AppBlacklist = []config.BlacklistEntry{{Match: "ORG.TEST.EDITOR", Hide: config.HideWhenNoWindow}}
	if AppAllowed(app, 0, f, config.ShortcutScope{}, Context{}) {
		t.Fatal("blacklisted windowless app was included")
	}
	if !AppAllowed(app, 1, f, config.ShortcutScope{}, Context{}) {
		t.Fatal("windowless-only blacklist hid an app with a real window")
	}
	if !AppAllowed(app, -1, f, config.ShortcutScope{}, Context{}) {
		t.Fatal("unknown native window count was treated as definitely windowless")
	}
}

func TestAppFilterIdentityScopeAndHiddenRules(t *testing.T) {
	app := domain.App{ID: 10, Name: "Editor", BundleID: "org.test.editor"}
	for _, tc := range []struct {
		name  string
		alter func(*config.Filters, *config.ShortcutScope, *Context, *domain.App)
		want  bool
	}{
		{"regular windowless", func(*config.Filters, *config.ShortcutScope, *Context, *domain.App) {}, true},
		{"self", func(_ *config.Filters, _ *config.ShortcutScope, c *Context, _ *domain.App) {
			c.SelfBundleID = "ORG.TEST.EDITOR"
		}, false},
		{"invalid identity", func(_ *config.Filters, _ *config.ShortcutScope, _ *Context, a *domain.App) { a.ID = 0 }, false},
		{"name blacklist", func(f *config.Filters, _ *config.ShortcutScope, _ *Context, _ *domain.App) {
			f.AppBlacklist = []config.BlacklistEntry{{Match: "EDITOR", Hide: config.HideAlways}}
		}, false},
		{"other active app", func(_ *config.Filters, s *config.ShortcutScope, c *Context, _ *domain.App) {
			s.AppScope = config.AppScopeActiveApp
			c.ActiveAppID = 20
		}, false},
		{"selected active app", func(_ *config.Filters, s *config.ShortcutScope, c *Context, _ *domain.App) {
			s.AppScope = config.AppScopeActiveApp
			c.ActiveAppID = 10
		}, true},
		{"hidden excluded", func(f *config.Filters, _ *config.ShortcutScope, _ *Context, a *domain.App) {
			f.ShowHiddenApps = config.VisHide
			a.Hidden = true
		}, false},
		{"hidden ordered last", func(f *config.Filters, _ *config.ShortcutScope, _ *Context, a *domain.App) {
			f.ShowHiddenApps = config.VisShowAtEnd
			a.Hidden = true
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, s, ctx, a := config.Default().Filters, config.ShortcutScope{}, Context{}, app
			tc.alter(&f, &s, &ctx, &a)
			if got := AppAllowed(a, 0, f, s, ctx); got != tc.want {
				t.Fatalf("AppAllowed = %v, want %v", got, tc.want)
			}
		})
	}
}
