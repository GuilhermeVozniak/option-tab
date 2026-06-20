package switcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/mru"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

// recordView records the states pushed to the view.
type recordView struct {
	shows   []State
	updates []State
	hides   int
}

func (r *recordView) Show(s State)   { r.shows = append(r.shows, s) }
func (r *recordView) Update(s State) { r.updates = append(r.updates, s) }
func (r *recordView) Hide()          { r.hides++ }

func (r *recordView) last() State {
	if len(r.updates) > 0 {
		return r.updates[len(r.updates)-1]
	}
	if len(r.shows) > 0 {
		return r.shows[len(r.shows)-1]
	}
	return State{}
}

func newController(t *testing.T, wins []domain.Window, mut func(*config.Settings)) (*Controller, *fake.Fake, *recordView) {
	t.Helper()
	f := fake.New()
	f.SetWindows(wins)
	v := &recordView{}
	s := config.Default()
	if mut != nil {
		mut(&s)
	}
	c := New(Deps{
		Windows:      f,
		Focuser:      f,
		Env:          f,
		View:         v,
		MRU:          mru.New(),
		SelfBundleID: "com.option-tab",
	}, s)
	return c, f, v
}

func threeWins() []domain.Window {
	return []domain.Window{
		{ID: 1, AppID: 1, AppName: "A", Title: "a", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 2, AppID: 2, AppName: "B", Title: "b", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 3, AppID: 3, AppName: "C", Title: "c", OnScreen: true, SpaceID: 1, ScreenID: 1},
	}
}

func selectedID(s State) domain.WindowID {
	if s.Selected < 0 || s.Selected >= len(s.Entries) {
		return 0
	}
	return s.Entries[s.Selected].WindowID
}

func TestActivate_OpensAndSelectsPreviousWindow(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil) // HoldToCycle default true
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if !c.IsOpen() {
		t.Fatal("controller should be open after activate")
	}
	if len(v.shows) != 1 {
		t.Fatalf("expected 1 Show, got %d", len(v.shows))
	}
	if got := selectedID(v.last()); got != 2 {
		t.Errorf("initial selection = window %d, want 2 (the previous window)", got)
	}
	if len(v.last().Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(v.last().Entries))
	}
}

func TestActivate_EmptyWindowsDoesNotOpen(t *testing.T) {
	c, _, v := newController(t, nil, nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if c.IsOpen() {
		t.Error("should not open with no windows")
	}
	if len(v.shows) != 0 {
		t.Error("Show should not be called with no windows")
	}
}

func TestQuickSwitch_ActivateThenReleaseFocusesPrevious(t *testing.T) {
	c, f, _ := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyRelease})
	if f.LastFocused != 2 {
		t.Errorf("quick switch should focus window 2, got %d", f.LastFocused)
	}
	if c.IsOpen() {
		t.Error("should be closed after release")
	}
}

func TestAdvanceWrapsForward(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // selected=2 (idx1)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyAdvance})                 // idx2 -> window 3
	if got := selectedID(v.last()); got != 3 {
		t.Errorf("after advance selected = %d, want 3", got)
	}
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyAdvance}) // wraps to idx0 -> window 1
	if got := selectedID(v.last()); got != 1 {
		t.Errorf("after wrap selected = %d, want 1", got)
	}
}

func TestReverseWrapsBackward(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // idx1
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyReverse})                 // idx0 -> window 1
	if got := selectedID(v.last()); got != 1 {
		t.Errorf("after reverse selected = %d, want 1", got)
	}
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyReverse}) // wraps to idx2 -> window 3
	if got := selectedID(v.last()); got != 3 {
		t.Errorf("after reverse wrap selected = %d, want 3", got)
	}
}

func TestCancelDoesNotFocus(t *testing.T) {
	c, f, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyCancel})
	if len(f.FocusCalls) != 0 {
		t.Errorf("cancel must not focus, got calls %v", f.FocusCalls)
	}
	if c.IsOpen() {
		t.Error("should be closed after cancel")
	}
	if v.hides != 1 {
		t.Errorf("expected 1 Hide, got %d", v.hides)
	}
}

func TestPressMode_InitialSelectionIsCurrent(t *testing.T) {
	c, _, v := newController(t, threeWins(), func(s *config.Settings) { s.Behavior.HoldToCycle = false })
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if got := selectedID(v.last()); got != 1 {
		t.Errorf("press mode initial selection = %d, want 1 (current)", got)
	}
}

func TestSetSearch_FiltersAndResets(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.SetSearch("b")
	st := v.last()
	if len(st.Entries) != 1 || st.Entries[0].WindowID != 2 {
		t.Errorf("search 'b' should match only window 2, got %+v", st.Entries)
	}
	if st.Search != "b" {
		t.Errorf("state search = %q, want b", st.Search)
	}
	c.SetSearch("")
	if len(v.last().Entries) != 3 {
		t.Errorf("clearing search should restore all entries, got %d", len(v.last().Entries))
	}
}

func TestSelect_Hover(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.Select(0)
	if got := selectedID(v.last()); got != 1 {
		t.Errorf("hover-select index 0 = %d, want 1", got)
	}
	c.Select(99) // out of range ignored
	if got := selectedID(v.last()); got != 1 {
		t.Errorf("out-of-range select should be ignored, got %d", got)
	}
}

func TestConfirm_FocusesAndTracksMRU(t *testing.T) {
	c, f, _ := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.Select(2) // window 3
	c.Confirm()
	if f.LastFocused != 3 {
		t.Errorf("confirm should focus window 3, got %d", f.LastFocused)
	}
}

func TestCloseSelected_RemovesAndClosesWhenEmpty(t *testing.T) {
	c, f, v := newController(t, []domain.Window{{ID: 1, AppID: 1, AppName: "A", Title: "a", OnScreen: true, SpaceID: 1, ScreenID: 1}}, nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c.CloseSelected()
	if len(f.CloseCalls) != 1 || f.CloseCalls[0] != 1 {
		t.Errorf("CloseSelected should close window 1, got %v", f.CloseCalls)
	}
	if c.IsOpen() {
		t.Error("overlay should close when last window is closed")
	}
	if v.hides != 1 {
		t.Errorf("expected Hide on empty, got %d", v.hides)
	}
}

func TestMinimizeAndQuitAndHideSelected(t *testing.T) {
	c, f, _ := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // selected window 2
	c.MinimizeSelected()
	if len(f.MinimizeCalls) != 1 || f.MinimizeCalls[0] != 2 {
		t.Errorf("MinimizeSelected wrong: %v", f.MinimizeCalls)
	}
	c.QuitSelectedApp()
	if len(f.QuitCalls) == 0 {
		t.Error("QuitSelectedApp should call QuitApp")
	}
	c2, f2, _ := newController(t, threeWins(), nil)
	c2.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	c2.HideSelectedApp()
	if len(f2.HideCalls) == 0 {
		t.Error("HideSelectedApp should call HideApp")
	}
}

func TestActivate_PerShortcutActiveAppScope(t *testing.T) {
	wins := []domain.Window{
		{ID: 1, AppID: 1, AppName: "A", Title: "a", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 2, AppID: 2, AppName: "B", Title: "b", OnScreen: true, SpaceID: 1, ScreenID: 1},
		{ID: 3, AppID: 1, AppName: "A", Title: "a2", OnScreen: true, SpaceID: 1, ScreenID: 1},
	}
	c, f, v := newController(t, wins, nil)
	f.ActiveAppID = 1
	// shortcut 2 is scoped to the active app by default.
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 2})
	st := v.last()
	if len(st.Entries) != 2 {
		t.Fatalf("active-app scope should show 2 windows of app 1, got %d", len(st.Entries))
	}
	for _, e := range st.Entries {
		if e.AppID != 1 {
			t.Errorf("entry from wrong app: %+v", e)
		}
	}
}

func TestActivate_DisabledOrUnknownShortcutIgnored(t *testing.T) {
	c, _, v := newController(t, threeWins(), func(s *config.Settings) {
		s.Shortcuts[0].Enabled = false
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // disabled
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 7}) // unknown
	if c.IsOpen() || len(v.shows) != 0 {
		t.Error("disabled/unknown shortcuts should not open the switcher")
	}
}

func TestActivateWhileOpenAdvances(t *testing.T) {
	c, _, v := newController(t, threeWins(), nil)
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // idx1 window2
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}) // advance -> window3
	if got := selectedID(v.last()); got != 3 {
		t.Errorf("activate while open should advance, got %d", got)
	}
}

func TestState_ReflectsStyleAndAppearance(t *testing.T) {
	c, _, _ := newController(t, threeWins(), func(s *config.Settings) {
		s.Appearance.Style = config.StyleTitles
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if c.State().Style != config.StyleTitles {
		t.Errorf("state style = %q, want titles", c.State().Style)
	}
}

func TestStyleOverridePerShortcut(t *testing.T) {
	c, _, v := newController(t, threeWins(), func(s *config.Settings) {
		s.Appearance.Style = config.StyleThumbnails
		s.Shortcuts[0].StyleOverride = config.StyleAppIcons
	})
	c.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	if v.last().Style != config.StyleAppIcons {
		t.Errorf("shortcut style override should win, got %q", v.last().Style)
	}
}
