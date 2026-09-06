package switcher

import (
	"errors"
	"sync"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/mru"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type appNativeFake struct {
	*fake.Fake
	mu            sync.Mutex
	apps          []domain.App
	activated     []domain.AppID
	activateErr   error
	activateHit   chan struct{}
	activateGo    chan struct{}
	presence      map[domain.AppID]platform.WindowPresence
	presenceCalls []domain.AppID
}

func (f *appNativeFake) AppWindowPresence(id domain.AppID) platform.WindowPresence {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.presenceCalls = append(f.presenceCalls, id)
	if p, ok := f.presence[id]; ok {
		return p
	}
	return platform.WindowsUnknown
}

func (f *appNativeFake) Apps() ([]domain.App, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]domain.App(nil), f.apps...), nil
}

func (f *appNativeFake) ActivateApp(id domain.AppID) error {
	if f.activateHit != nil {
		close(f.activateHit)
		<-f.activateGo
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activated = append(f.activated, id)
	return f.activateErr
}

func (f *appNativeFake) QuitApp(id domain.AppID) error {
	f.mu.Lock()
	kept := f.apps[:0:0]
	for _, app := range f.apps {
		if app.ID != id {
			kept = append(kept, app)
		}
	}
	f.apps = kept
	f.mu.Unlock()
	return f.Fake.QuitApp(id)
}

func newAppController(t *testing.T, apps []domain.App, wins []domain.Window, mutate func(*config.Settings)) (*Controller, *appNativeFake, *recordView) {
	t.Helper()
	base := fake.New()
	base.SetWindows(wins)
	native := &appNativeFake{Fake: base, apps: apps}
	view := &recordView{}
	settings := config.Default()
	settings.Shortcuts[0].Mode = config.ModeApps
	if mutate != nil {
		mutate(&settings)
	}
	return New(Deps{Windows: native, Apps: native, Focuser: native, AppActivator: native, Env: native, View: view, MRU: mru.New(), SelfBundleID: "com.option-tab", Cursor: native}, settings), native, view
}

func appFixtures() ([]domain.App, []domain.Window) {
	return []domain.App{{ID: 10, Name: "Same", BundleID: "a"}, {ID: 20, Name: "Same", BundleID: "b"}, {ID: 30, Name: "Windowless", BundleID: "c"}}, []domain.Window{
		{ID: 101, AppID: 10, AppName: "Same", BundleID: "a", Title: "one", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 102, AppID: 10, AppName: "Same", BundleID: "a", Title: "two", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 201, AppID: 20, AppName: "Same", BundleID: "b", Title: "other", OnScreen: true, SpaceID: 1, ScreenID: 1},
	}
}

func TestAppModeGroupsDistinctPIDsAndKeepsWindowlessApps(t *testing.T) {
	apps, wins := appFixtures()
	c, _, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	st := c.State()
	if st.Mode != config.ModeApps || len(st.Apps) != 3 {
		t.Fatalf("state = %+v", st)
	}
	if st.Apps[0].AppID != 10 || st.Apps[1].AppID != 20 || st.Apps[2].AppID != 30 {
		t.Fatalf("apps = %+v", st.Apps)
	}
	if len(st.Entries) != 2 || st.SelectedWindowID != 101 {
		t.Fatalf("selected gallery = %+v id=%d", st.Entries, st.SelectedWindowID)
	}
	c.SelectApp(30)
	st = c.State()
	if len(st.Entries) != 0 || st.SelectedWindowID != 0 {
		t.Fatalf("windowless state = %+v", st)
	}
}

func TestAppModeCyclesAppsAndSelectsExactPreview(t *testing.T) {
	apps, wins := appFixtures()
	c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.Advance()
	if c.State().Apps[c.State().Selected].AppID != 20 {
		t.Fatal("advance did not move by app")
	}
	c.SelectApp(10)
	c.SelectAppWindow(102)
	if err := c.ConfirmApp(10); err != nil {
		t.Fatal(err)
	}
	if native.LastFocused != 102 {
		t.Fatalf("focused %d, want 102", native.LastFocused)
	}
	if c.IsOpen() {
		t.Fatal("successful confirm left overlay open")
	}

	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if err := c.ConfirmApp(30); err != nil {
		t.Fatal(err)
	}
	if len(native.activated) != 1 || native.activated[0] != 30 {
		t.Fatalf("activated = %v", native.activated)
	}
}

func TestAppConfirmRejectsVanishedOrChangedOwnerAndLeavesSessionOpen(t *testing.T) {
	apps, wins := appFixtures()
	for _, tc := range []struct {
		name        string
		replacement []domain.Window
	}{
		{name: "vanished", replacement: wins[1:]},
		{name: "changed owner", replacement: append([]domain.Window{{ID: 101, AppID: 20, AppName: "Same", BundleID: "b", Title: "stolen", OnScreen: true}}, wins[1:]...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
			c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
			native.SetWindows(tc.replacement)
			if err := c.ConfirmApp(10); err == nil {
				t.Fatal("expected stale target error")
			}
			if !c.IsOpen() || len(native.FocusCalls) != 0 {
				t.Fatalf("open=%v focus=%v", c.IsOpen(), native.FocusCalls)
			}
		})
	}
}

func TestAppModeRefreshPreservesPreviewAndClampsAfterQuit(t *testing.T) {
	apps, wins := appFixtures()
	c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SelectAppWindow(102)
	c.CloseSelected()
	if st := c.State(); st.Apps[st.Selected].AppID != 10 || st.SelectedWindowID != 101 || len(st.Entries) != 1 {
		t.Fatalf("close refresh = %+v", st)
	}
	c.SelectApp(20)
	c.QuitSelectedApp()
	st := c.State()
	if !st.Open || len(st.Apps) != 2 || st.Selected < 0 || st.Selected >= len(st.Apps) || st.Apps[st.Selected].AppID == 20 {
		t.Fatalf("quit refresh = %+v", st)
	}
	if len(native.QuitCalls) != 1 || native.QuitCalls[0] != 20 {
		t.Fatalf("quit calls = %v", native.QuitCalls)
	}
}

func TestAppActivationFailureIsVisibleAndLeavesSessionOpen(t *testing.T) {
	apps, wins := appFixtures()
	c, native, view := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = true })
	native.activateErr = errors.New("refused")
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SelectApp(30)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
	if !c.IsOpen() || len(view.failures) != 1 {
		t.Fatalf("open=%v failures=%v", c.IsOpen(), view.failures)
	}
}

func TestDelayedOldAppCommitCannotHideReopenedSession(t *testing.T) {
	apps, wins := appFixtures()
	c, native, view := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
	native.activateHit, native.activateGo = make(chan struct{}), make(chan struct{})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SelectApp(30)
	done := make(chan error, 1)
	go func() { done <- c.ConfirmApp(30) }()
	<-native.activateHit
	c.Cancel()
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	close(native.activateGo)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !c.IsOpen() || view.hides != 1 {
		t.Fatalf("old completion hid replacement: open=%v hides=%d", c.IsOpen(), view.hides)
	}
}

func TestAppModeUsesIndependentPreferencesAndSearchesWindowTitles(t *testing.T) {
	apps, wins := appFixtures()
	c, _, _ := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Behavior.HoldToCycle = true
		s.Appearance.Style = config.StyleTitles
		s.AppSwitcher.Behavior.HoldToCycle = false
		s.AppSwitcher.Appearance.Style = config.StyleAppIcons
		s.AppSwitcher.Placement = config.PlaceCursorScreen
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	st := c.State()
	if st.Selected != 0 || st.Style != config.StyleAppIcons || st.Placement != config.PlaceCursorScreen {
		t.Fatalf("preferences = %+v", st)
	}
	c.SetSearch("other")
	st = c.State()
	if len(st.Apps) != 1 || st.Apps[0].AppID != 20 || len(st.Entries) != 1 || st.Entries[0].WindowID != 201 {
		t.Fatalf("search state = %+v", st)
	}
}

func TestWindowAndAppShortcutsUseIndependentModesAndPreferences(t *testing.T) {
	apps, wins := appFixtures()
	c, _, _ := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Shortcuts[1].Mode = config.ModeWindows
		s.Shortcuts[1].Scope.AppScope = config.AppScopeAll
		s.Behavior.HoldToCycle = true
		s.Appearance.Style = config.StyleTitles
		s.AppSwitcher.Behavior.HoldToCycle = false
		s.AppSwitcher.Appearance.Style = config.StyleAppIcons
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if st := c.State(); st.Mode != config.ModeApps || st.Style != config.StyleAppIcons || st.Selected != 0 || len(st.Apps) != 3 {
		t.Fatalf("app state = %+v", st)
	}
	c.Cancel()
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 2})
	if st := c.State(); st.Mode != config.ModeWindows || st.Style != config.StyleTitles || st.Selected != 1 || len(st.Apps) != 0 || len(st.Entries) != 3 {
		t.Fatalf("window state = %+v", st)
	}
}

func TestAppModeConsumesTriStatePresenceOnlyForAppsWithoutEligibleWindows(t *testing.T) {
	apps, wins := appFixtures()
	c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) {
		s.AppSwitcher.Behavior.HoldToCycle = false
		s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "c", Hide: config.HideWhenNoWindow}}
	})
	native.presence = map[domain.AppID]platform.WindowPresence{20: platform.WindowsPresent, 30: platform.WindowsUnknown}
	c.deps.AppWindows = native
	// Filter app 20's only window, forcing presence resolution without calling
	// the native seam for app 10, which still has eligible previews.
	wins[2].Fullscreen = true
	native.SetWindows(wins)
	c.settings.Filters.ShowFullscreen = config.VisHide
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	st := c.State()
	if len(st.Apps) != 2 || st.Apps[0].AppID != 10 || st.Apps[1].AppID != 30 || st.Apps[1].WindowPresence != platform.WindowsUnknown {
		t.Fatalf("apps = %+v", st.Apps)
	}
	if len(native.presenceCalls) != 2 || native.presenceCalls[0] != 20 || native.presenceCalls[1] != 30 {
		t.Fatalf("presence calls = %v", native.presenceCalls)
	}
}

func TestConfirmedWindowlessAppHonorsBlacklistDespiteCGSurfaces(t *testing.T) {
	apps, wins := appFixtures()
	wins = append(wins, domain.Window{ID: 301, AppID: 30, BundleID: "c"})
	c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) {
		s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "c", Hide: config.HideWhenNoWindow}}
	})
	native.presence = map[domain.AppID]platform.WindowPresence{30: platform.WindowsNone}
	c.deps.AppWindows = native
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	for _, app := range c.State().Apps {
		if app.AppID == 30 {
			t.Fatal("confirmed windowless app bypassed no-window blacklist")
		}
	}
}

type acknowledgedFocusFake struct {
	*fake.Fake
	err    error
	called []domain.WindowID
}

func (f *acknowledgedFocusFake) PerformTargetAction(kind string, id domain.WindowID, app domain.AppID) error {
	f.called = append(f.called, id)
	return f.err
}

func TestCommitRequiresNativeFocusAcknowledgement(t *testing.T) {
	apps, wins := appFixtures()
	c, native, _ := newAppController(t, apps, wins, nil)
	nativeAck := &acknowledgedFocusFake{Fake: native.Fake, err: errors.New("AX raise refused")}
	c.deps.Focuser = nativeAck
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SelectApp(10)
	if err := c.ConfirmApp(10); !errors.Is(err, nativeAck.err) {
		t.Fatalf("native refusal not preserved: %v", err)
	}
	if !c.IsOpen() || len(nativeAck.called) != 1 || nativeAck.called[0] != 101 || len(native.FocusCalls) != 0 {
		t.Fatal("commit used unacknowledged legacy focus path")
	}
}

func TestAppConfirmWindowUsesCurrentGalleryAfterRefresh(t *testing.T) {
	apps, wins := appFixtures()
	for _, kind := range []string{"otherApp", "search", "newWindow"} {
		t.Run(kind, func(t *testing.T) {
			c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
			c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
			id := domain.WindowID(101)
			switch kind {
			case "otherApp":
				c.SelectApp(20)
			case "search":
				c.SetSearch("other")
			case "newWindow":
				native.SetWindows(append(wins, domain.Window{ID: 103, AppID: 10, Title: "new", OnScreen: true, SpaceID: 1, ScreenID: 1}))
				c.Refresh()
				id = 103
			}
			err := c.ConfirmWindow(id)
			if kind == "newWindow" {
				if err != nil || len(native.FocusCalls) != 1 || native.FocusCalls[0] != 103 {
					t.Errorf("new gallery window rejected: %v focus=%v", err, native.FocusCalls)
				}
			} else if err == nil || len(native.FocusCalls) != 0 {
				t.Errorf("undisplayed target committed: err=%v focus=%v", err, native.FocusCalls)
			}
		})
	}
}

type commitReadBarrier struct {
	*appNativeFake
	gateMu       sync.Mutex
	kind         string
	hit, release chan struct{}
}

func (b *commitReadBarrier) wait(kind string) {
	b.gateMu.Lock()
	block := b.kind == kind
	if block {
		b.kind = ""
	}
	b.gateMu.Unlock()
	if block {
		close(b.hit)
		<-b.release
	}
}

func (b *commitReadBarrier) Windows() ([]domain.Window, error) {
	b.wait("windows")
	return b.Fake.Windows()
}

func (b *commitReadBarrier) Apps() ([]domain.App, error) {
	b.wait("apps")
	return b.appNativeFake.Apps()
}

func TestCommitReadCannotDispatchAfterSessionReplacement(t *testing.T) {
	for _, kind := range []string{"windows", "apps"} {
		t.Run(kind, func(t *testing.T) {
			apps, wins := appFixtures()
			c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) { s.AppSwitcher.Behavior.HoldToCycle = false })
			b := &commitReadBarrier{appNativeFake: native, hit: make(chan struct{}), release: make(chan struct{})}
			c.deps.Windows = b
			c.deps.Apps = b
			activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
			c.HandleHotkey(activate)
			b.gateMu.Lock()
			b.kind = kind
			b.gateMu.Unlock()
			result := make(chan error, 1)
			go func() {
				if kind == "windows" {
					result <- c.ConfirmWindow(101)
				} else {
					result <- c.ConfirmApp(30)
				}
			}()
			<-b.hit
			c.Suspend(true)
			c.Suspend(false)
			c.HandleHotkey(activate)
			close(b.release)
			if err := <-result; err == nil {
				t.Error("stale read accepted")
			}
			if len(native.FocusCalls) != 0 || len(native.activated) != 0 {
				t.Errorf("stale native dispatch focus=%v apps=%v", native.FocusCalls, native.activated)
			}
			if !c.IsOpen() {
				t.Fatal("replacement lost")
			}
		})
	}
}

type sessionHideBarrierView struct {
	mu               sync.Mutex
	visible          uint64
	entered, release chan struct{}
}

func (v *sessionHideBarrierView) Show(s State)               { v.mu.Lock(); v.visible = s.Session; v.mu.Unlock() }
func (v *sessionHideBarrierView) Update(s State)             { v.Show(s) }
func (v *sessionHideBarrierView) Hide()                      { v.hide(0) }
func (v *sessionHideBarrierView) HideSession(session uint64) { v.hide(session) }
func (v *sessionHideBarrierView) hide(session uint64) {
	close(v.entered)
	<-v.release
	v.mu.Lock()
	defer v.mu.Unlock()
	if session == 0 || session == v.visible {
		v.visible = 0
	}
}

func TestRetiringHideCannotCloseReplacementPresentation(t *testing.T) {
	c, _, _ := newController(t, threeWins(), nil)
	v := &sessionHideBarrierView{entered: make(chan struct{}), release: make(chan struct{})}
	c.deps.View = v
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	c.HandleHotkey(activate)
	done := make(chan struct{})
	go func() { c.Cancel(); close(done) }()
	<-v.entered
	c.HandleHotkey(activate)
	fresh := c.State().Session
	close(v.release)
	<-done
	v.mu.Lock()
	visible := v.visible
	v.mu.Unlock()
	if visible != fresh || !c.IsOpen() {
		t.Fatalf("retiring Hide removed fresh presentation: visible=%d fresh=%d", visible, fresh)
	}
}

type delayedAcknowledgedFocus struct {
	*appNativeFake
	hit, release chan struct{}
}

func (f *delayedAcknowledgedFocus) PerformTargetAction(string, domain.WindowID, domain.AppID) error {
	close(f.hit)
	<-f.release
	return nil
}

func TestAcceptedOldFocusDoesNotWarpCursorAfterReplacement(t *testing.T) {
	apps, wins := appFixtures()
	c, native, _ := newAppController(t, apps, wins, func(s *config.Settings) {
		s.AppSwitcher.Behavior.HoldToCycle = false
		s.AppSwitcher.Behavior.CursorFollowFocus = true
	})
	focus := &delayedAcknowledgedFocus{native, make(chan struct{}), make(chan struct{})}
	c.deps.Focuser = focus
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	c.HandleHotkey(activate)
	done := make(chan error, 1)
	go func() { done <- c.ConfirmWindow(101) }()
	<-focus.hit
	c.Suspend(true)
	c.Suspend(false)
	c.HandleHotkey(activate)
	close(focus.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(native.WarpCalls) != 0 || !c.IsOpen() {
		t.Fatalf("accepted old focus affected replacement: warps=%v open=%v", native.WarpCalls, c.IsOpen())
	}
}
