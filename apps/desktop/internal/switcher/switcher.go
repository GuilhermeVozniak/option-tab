// Package switcher contains the window-switcher controller: the state machine
// that turns hotkey and UI events into a filtered, ordered, searchable list and
// commits the user's selection. It depends only on the platform port and the
// pure logic packages, so it is fully testable with a fake platform.
package switcher

import (
	"sync"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/mru"
	"option-tab/internal/order"
	"option-tab/internal/platform"
	"option-tab/internal/search"
)

// View receives switcher state changes for rendering. The Wails layer
// implements it by emitting events to the frontend; tests use a recorder.
type View interface {
	Show(State)
	Update(State)
	Hide()
}

// Entry is one window as presented to the view (JSON-serializable).
type Entry struct {
	WindowID   domain.WindowID `json:"windowId"`
	AppID      domain.AppID    `json:"appId"`
	Title      string          `json:"title"`
	AppName    string          `json:"appName"`
	BundleID   string          `json:"bundleId"`
	SpaceID    domain.SpaceID  `json:"spaceId"`
	Minimized  bool            `json:"minimized"`
	Hidden     bool            `json:"hidden"`
	Fullscreen bool            `json:"fullscreen"`
	// Icon is a base64 PNG data URL of the owning app's icon. The controller
	// leaves it empty; the view layer fills it (platform-specific).
	Icon string `json:"icon,omitempty"`
}

// State is the full switcher snapshot handed to the view.
type State struct {
	Open          bool               `json:"open"`
	Style         config.VisualStyle `json:"style"`
	Appearance    config.Appearance  `json:"appearance"`
	Placement     config.Placement   `json:"placement"`
	Entries       []Entry            `json:"entries"`
	Selected      int                `json:"selected"`
	Search        string             `json:"search"`
	ShortcutID    int                `json:"shortcutId"`
	VimKeys       bool               `json:"vimKeys"`
	ArrowKeys     bool               `json:"arrowKeys"`
	MouseHover    bool               `json:"mouseHover"`
	ActiveSpaceID domain.SpaceID     `json:"activeSpaceId"`
	// PlacementScreenID is the display the overlay window is sized to appear on.
	PlacementScreenID domain.ScreenID `json:"placementScreenId"`
}

// resolvePlacementScreen picks the display id the overlay should appear on for
// the given Placement. It falls back to the main screen, then the first
// screen, then zero ("keep the window's current screen").
func resolvePlacementScreen(p config.Placement, screens []domain.Screen, active, cursor domain.ScreenID) domain.ScreenID {
	target := active
	if p == config.PlaceCursorScreen {
		target = cursor
	}
	for _, s := range screens {
		if s.ID == target {
			return s.ID
		}
	}
	for _, s := range screens {
		if s.Main {
			return s.ID
		}
	}
	if len(screens) > 0 {
		return screens[0].ID
	}
	return 0
}

// OrderSelectedFirst returns entries with the selected one moved to the front,
// preserving the relative order of the rest. Used so thumbnail capture snaps
// the most likely pick first. A selected index out of range returns the input.
func OrderSelectedFirst(entries []Entry, selected int) []Entry {
	if selected <= 0 || selected >= len(entries) {
		return entries
	}
	out := make([]Entry, 0, len(entries))
	out = append(out, entries[selected])
	out = append(out, entries[:selected]...)
	out = append(out, entries[selected+1:]...)
	return out
}

// Deps are the controller's collaborators.
type Deps struct {
	Windows      platform.WindowSource
	Focuser      platform.Focuser
	Env          platform.Environment
	View         View
	MRU          *mru.Tracker
	SelfBundleID string
	// Cursor is optional; when present and CursorFollowFocus is enabled, the
	// mouse is warped to the window focused on commit.
	Cursor platform.CursorWarper
}

// Controller is the switcher state machine. All public methods are safe for
// concurrent use: hotkey events arrive on a platform goroutine while UI calls
// arrive on the Wails goroutine.
type Controller struct {
	mu       sync.Mutex
	deps     Deps
	settings config.Settings

	open            bool
	shortcut        config.Shortcut
	activeSpace     domain.SpaceID // captured at activate/refresh for the view's badges
	placementScreen domain.ScreenID
	baseList        []domain.Window // filtered + ordered, before search
	list            []domain.Window // after search
	selected        int
	search          string
}

// New creates a Controller with the given dependencies and initial settings.
func New(deps Deps, settings config.Settings) *Controller {
	if deps.MRU == nil {
		deps.MRU = mru.New()
	}
	return &Controller{deps: deps, settings: settings}
}

// SetSettings replaces the active settings (e.g. after the user edits prefs).
func (c *Controller) SetSettings(s config.Settings) {
	c.mu.Lock()
	c.settings = s
	c.mu.Unlock()
}

// IsOpen reports whether the switcher overlay is currently shown.
func (c *Controller) IsOpen() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open
}

// State returns the current snapshot.
func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot()
}

// HandleHotkey routes a platform hotkey event to the right transition.
func (c *Controller) HandleHotkey(ev platform.HotkeyEvent) {
	switch ev.Kind {
	case platform.HotkeyActivate:
		if c.IsOpen() {
			c.Advance()
		} else {
			c.activate(ev.ShortcutID)
		}
	case platform.HotkeyAdvance:
		c.Advance()
	case platform.HotkeyReverse:
		c.Reverse()
	case platform.HotkeyRelease:
		// Per-shortcut "when released: do nothing" keeps the switcher open
		// until Enter/Escape/click (AltTab parity).
		if !c.releaseDoesNothing() {
			c.Confirm()
		}
	case platform.HotkeyCancel:
		c.Cancel()
	}
}

// releaseDoesNothing reports whether the active shortcut ignores modifier
// release while the switcher is open.
func (c *Controller) releaseDoesNothing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.open && c.shortcut.WhenReleased == config.ReleaseDoNothing
}

// SetPaused enables or disables activation. While paused, hotkeys do not open
// the switcher; an already-open overlay is unaffected. Pausing is exposed in the
// menubar so the user can temporarily disable the switcher.
func (c *Controller) SetPaused(paused bool) {
	c.mu.Lock()
	c.settings.Behavior.Paused = paused
	c.mu.Unlock()
}

// Paused reports whether activation is currently suspended.
func (c *Controller) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.settings.Behavior.Paused
}

// activate opens the switcher for the given shortcut id.
func (c *Controller) activate(shortcutID int) {
	c.mu.Lock()

	if c.settings.Behavior.Paused {
		c.mu.Unlock()
		return
	}

	sc, ok := c.findShortcut(shortcutID)
	if !ok || !sc.Enabled {
		c.mu.Unlock()
		return
	}
	wins, err := c.deps.Windows.Windows()
	if err != nil {
		c.mu.Unlock()
		return
	}
	wins = c.deps.MRU.Stamp(wins)

	ctx := filter.Context{
		ActiveAppID:    c.deps.Env.ActiveApp(),
		ActiveSpaceID:  c.deps.Env.ActiveSpace(),
		ActiveScreenID: c.deps.Env.ActiveScreen(),
		CursorScreenID: c.deps.Env.CursorScreen(),
		SelfBundleID:   c.deps.SelfBundleID,
	}
	c.activeSpace = ctx.ActiveSpaceID
	c.placementScreen = resolvePlacementScreen(
		c.settings.Placement, c.deps.Env.Screens(), ctx.ActiveScreenID, ctx.CursorScreenID,
	)
	if filter.ShortcutIgnoredForApp(wins, ctx.ActiveAppID, c.settings.Filters.AppBlacklist) {
		c.mu.Unlock()
		return
	}
	ordered := c.composeLocked(wins, sc.Scope, ctx)
	if len(ordered) == 0 {
		c.mu.Unlock()
		return
	}

	c.open = true
	c.shortcut = sc
	c.baseList = ordered
	c.list = ordered
	c.search = ""
	c.selected = 0
	if c.settings.Behavior.HoldToCycle && len(ordered) > 1 {
		c.selected = 1 // start on the previous window for instant quick-switch
	}
	st := c.snapshot()
	c.mu.Unlock()

	c.deps.View.Show(st)
}

// Advance moves the selection to the next window, wrapping around.
func (c *Controller) Advance() { c.move(1) }

// Reverse moves the selection to the previous window, wrapping around.
func (c *Controller) Reverse() { c.move(-1) }

// Navigate moves the selection by delta (used by arrow keys), wrapping.
func (c *Controller) Navigate(delta int) { c.move(delta) }

func (c *Controller) move(delta int) {
	c.mu.Lock()
	if !c.open || len(c.list) == 0 {
		c.mu.Unlock()
		return
	}
	n := len(c.list)
	c.selected = ((c.selected+delta)%n + n) % n
	st := c.snapshot()
	c.mu.Unlock()
	c.deps.View.Update(st)
}

// Select highlights the entry at index (used by mouse hover). Out-of-range
// indices are ignored.
func (c *Controller) Select(index int) {
	c.mu.Lock()
	if !c.open || index < 0 || index >= len(c.list) {
		c.mu.Unlock()
		return
	}
	c.selected = index
	st := c.snapshot()
	c.mu.Unlock()
	c.deps.View.Update(st)
}

// SetSearch updates the type-to-filter query, recomputing the visible list and
// resetting the selection to the best match.
func (c *Controller) SetSearch(query string) {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	c.search = query
	if query == "" {
		c.list = c.baseList
	} else {
		c.list = search.Filter(c.baseList, query)
	}
	c.selected = 0
	st := c.snapshot()
	c.mu.Unlock()
	c.deps.View.Update(st)
}

// Confirm focuses the selected window and closes the overlay.
func (c *Controller) Confirm() {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	var focusID domain.WindowID
	if len(c.list) > 0 && c.selected < len(c.list) {
		focusID = c.list[c.selected].ID
	}
	follow := c.settings.Behavior.CursorFollowFocus
	c.reset()
	c.mu.Unlock()

	if focusID != 0 {
		// Focus navigates to the window's Space natively (SkyLight window
		// fronting) so a window on another desktop comes forward there.
		_ = c.deps.Focuser.Focus(focusID)
		c.deps.MRU.Touch(focusID)
		if follow && c.deps.Cursor != nil {
			_ = c.deps.Cursor.WarpCursorToWindow(focusID)
		}
	}
	c.deps.View.Hide()
}

// Cancel closes the overlay without changing focus.
func (c *Controller) Cancel() {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	c.reset()
	c.mu.Unlock()
	c.deps.View.Hide()
}

// CloseSelected closes the selected window and refreshes the list.
func (c *Controller) CloseSelected() {
	c.act(func(w domain.Window) { _ = c.deps.Focuser.Close(w.ID) })
}

// MinimizeSelected minimizes the selected window and refreshes the list.
func (c *Controller) MinimizeSelected() {
	c.act(func(w domain.Window) { _ = c.deps.Focuser.Minimize(w.ID) })
}

// FullscreenSelected toggles fullscreen on the selected window and refreshes.
func (c *Controller) FullscreenSelected() {
	c.act(func(w domain.Window) { _ = c.deps.Focuser.Fullscreen(w.ID) })
}

// QuitSelectedApp quits the selected window's application and refreshes.
func (c *Controller) QuitSelectedApp() {
	c.act(func(w domain.Window) { _ = c.deps.Focuser.QuitApp(w.AppID) })
}

// HideSelectedApp hides the selected window's application and refreshes.
func (c *Controller) HideSelectedApp() {
	c.act(func(w domain.Window) { _ = c.deps.Focuser.HideApp(w.AppID) })
}

// act runs fn on the selected window (if any) then refreshes the list.
func (c *Controller) act(fn func(domain.Window)) {
	c.mu.Lock()
	if !c.open || len(c.list) == 0 || c.selected >= len(c.list) {
		c.mu.Unlock()
		return
	}
	w := c.list[c.selected]
	c.mu.Unlock()

	fn(w)
	c.refresh()
}

// refresh re-queries windows and rebuilds the list with the active shortcut and
// current search, keeping the overlay open. If nothing remains, it closes.
func (c *Controller) refresh() {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	wins, err := c.deps.Windows.Windows()
	if err != nil {
		c.mu.Unlock()
		return
	}
	wins = c.deps.MRU.Stamp(wins)
	ctx := filter.Context{
		ActiveAppID:    c.deps.Env.ActiveApp(),
		ActiveSpaceID:  c.deps.Env.ActiveSpace(),
		ActiveScreenID: c.deps.Env.ActiveScreen(),
		CursorScreenID: c.deps.Env.CursorScreen(),
		SelfBundleID:   c.deps.SelfBundleID,
	}
	c.activeSpace = ctx.ActiveSpaceID
	c.baseList = c.composeLocked(wins, c.shortcut.Scope, ctx)
	if c.search == "" {
		c.list = c.baseList
	} else {
		c.list = search.Filter(c.baseList, c.search)
	}
	if len(c.list) == 0 {
		c.reset()
		c.mu.Unlock()
		c.deps.View.Hide()
		return
	}
	if c.selected >= len(c.list) {
		c.selected = len(c.list) - 1
	}
	st := c.snapshot()
	c.mu.Unlock()
	c.deps.View.Update(st)
}

// composeLocked filters, orders (honoring a per-shortcut order override), and
// applies the "show at the end" tristates. Caller must hold the lock.
func (c *Controller) composeLocked(wins []domain.Window, scope config.ShortcutScope, ctx filter.Context) []domain.Window {
	mode := c.settings.Order
	if scope.Order.Valid() {
		mode = scope.Order
	}
	sorted := order.Sort(filter.Apply(wins, c.settings.Filters, scope, ctx), mode)
	return order.SendToBack(sorted, c.settings.Filters)
}

// reset clears the open state. Caller must hold the lock.
func (c *Controller) reset() {
	c.open = false
	c.baseList = nil
	c.list = nil
	c.selected = 0
	c.search = ""
}

// findShortcut returns the configured shortcut with the given id.
func (c *Controller) findShortcut(id int) (config.Shortcut, bool) {
	for _, sc := range c.settings.Shortcuts {
		if sc.ID == id {
			return sc, true
		}
	}
	return config.Shortcut{}, false
}

// snapshot builds the view State. Caller must hold the lock.
func (c *Controller) snapshot() State {
	style := c.settings.Appearance.Style
	if c.open && c.shortcut.StyleOverride != "" {
		style = c.shortcut.StyleOverride
	}
	entries := make([]Entry, len(c.list))
	for i, w := range c.list {
		entries[i] = Entry{
			WindowID:   w.ID,
			AppID:      w.AppID,
			Title:      w.Title,
			AppName:    w.AppName,
			BundleID:   w.BundleID,
			SpaceID:    w.SpaceID,
			Minimized:  w.Minimized,
			Hidden:     w.Hidden,
			Fullscreen: w.Fullscreen,
		}
	}
	return State{
		Open:              c.open,
		Style:             style,
		Appearance:        c.settings.Appearance,
		Placement:         c.settings.Placement,
		Entries:           entries,
		Selected:          c.selected,
		Search:            c.search,
		ShortcutID:        c.shortcut.ID,
		VimKeys:           c.settings.Behavior.VimKeys,
		ArrowKeys:         c.settings.Behavior.ArrowKeys,
		MouseHover:        c.settings.Behavior.MouseHoverSelect,
		ActiveSpaceID:     c.activeSpace,
		PlacementScreenID: c.placementScreen,
	}
}
