// Package switcher contains the window-switcher controller: the state machine
// that turns hotkey and UI events into a filtered, ordered, searchable list and
// commits the user's selection. It depends only on the platform port and the
// pure logic packages, so it is fully testable with a fake platform.
package switcher

import (
	"errors"
	"maps"
	"sync"
	"sync/atomic"

	"option-tab/internal/appgroup"
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
	Session           uint64                       `json:"session"`
	Revision          uint64                       `json:"revision"`
	Mode              config.SwitcherMode          `json:"mode"`
	Apps              []AppEntry                   `json:"apps"`
	SelectedWindowID  domain.WindowID              `json:"selectedWindowId"`
	ActionBindings    map[string]config.ActionKind `json:"actionBindings"`
	MiddleClickAction config.PointerAction         `json:"middleClickAction"`
	SwipeUpAction     config.PointerAction         `json:"swipeUpAction"`
	SwipeDownAction   config.PointerAction         `json:"swipeDownAction"`
	Open              bool                         `json:"open"`
	Style             config.VisualStyle           `json:"style"`
	Appearance        config.Appearance            `json:"appearance"`
	Placement         config.Placement             `json:"placement"`
	Entries           []Entry                      `json:"entries"`
	Selected          int                          `json:"selected"`
	Search            string                       `json:"search"`
	ShortcutID        int                          `json:"shortcutId"`
	VimKeys           bool                         `json:"vimKeys"`
	ArrowKeys         bool                         `json:"arrowKeys"`
	MouseHover        bool                         `json:"mouseHover"`
	ActiveSpaceID     domain.SpaceID               `json:"activeSpaceId"`
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
	Apps         platform.ApplicationSource
	AppActivator platform.ApplicationActivator
	AppWindows   platform.ApplicationWindowPresenceSource
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

	open                bool
	suspended           bool
	stopped             bool
	presentationSession atomic.Uint64
	shortcut            config.Shortcut
	activeSpace         domain.SpaceID // captured at activate/refresh for the view's badges
	placementScreen     domain.ScreenID
	baseList            []domain.Window // filtered + ordered, before search
	list                []domain.Window // after search
	baseGroups          []appgroup.Group
	groups              []appgroup.Group
	selectedWindow      domain.WindowID
	selected            int
	search              string
	session             uint64
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
			if err := c.Confirm(); err != nil {
				if reporter, ok := c.deps.View.(interface{ ActionFailed(string) }); ok {
					reporter.ActionFailed(err.Error())
				}
			}
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
	prefs := c.settings.Preferences(c.shortcut.Mode)
	return c.open && (!prefs.Behavior.HoldToCycle ||
		c.shortcut.WhenReleased == config.ReleaseDoNothing)
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

// PresentationSession allows a view to reject delayed state without taking
// the controller mutex while it holds its own view/native lifecycle lock.
func (c *Controller) PresentationSession() uint64 { return c.presentationSession.Load() }

// Suspend is a transient desktop-session guard. It cancels the current overlay
// without changing the user's persisted pause setting.
func (c *Controller) Suspend(suspended bool) {
	c.mu.Lock()
	c.suspended = suspended || c.stopped
	hide := suspended && c.open
	retiringSession := c.session
	if c.suspended {
		c.reset()
	}
	c.mu.Unlock()
	if hide {
		c.deliverHide(retiringSession)
	}
}

// Stop permanently closes admission. A queued session resume can no longer
// reactivate this controller. Native/UI retirement is delivered outside mu.
func (c *Controller) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	c.suspended = true
	hide, retiringSession := c.open, c.session
	c.reset()
	c.mu.Unlock()
	if hide {
		c.deliverHide(retiringSession)
	}
}

// deliverHide scopes delayed native/UI delivery to the presentation being
// retired. Simple legacy views retain the original Hide fallback.
func (c *Controller) deliverHide(session uint64) {
	if view, ok := c.deps.View.(interface{ HideSession(uint64) }); ok {
		view.HideSession(session)
	} else if c.deps.View != nil {
		c.deps.View.Hide()
	}
}

// activate opens the switcher for the given shortcut id.
func (c *Controller) activate(shortcutID int) {
	c.mu.Lock()

	if c.settings.Behavior.Paused || c.suspended || c.stopped {
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
	prefs := c.settings.Preferences(sc.Mode)
	c.placementScreen = resolvePlacementScreen(
		prefs.Placement, c.deps.Env.Screens(), ctx.ActiveScreenID, ctx.CursorScreenID,
	)
	if filter.ShortcutIgnoredForApp(wins, ctx.ActiveAppID, c.settings.Filters.AppBlacklist) {
		c.mu.Unlock()
		return
	}
	ordered := c.composeLocked(wins, sc.Scope, ctx, prefs)
	var groups []appgroup.Group
	if sc.Mode == config.ModeApps {
		if c.deps.Apps == nil || c.deps.AppActivator == nil {
			c.mu.Unlock()
			return
		}
		apps, appErr := c.deps.Apps.Apps()
		if appErr != nil {
			c.mu.Unlock()
			return
		}
		groups = c.composeAppsLocked(apps, wins, ordered, sc.Scope, ctx)
	}
	if (sc.Mode == config.ModeApps && len(groups) == 0) || (sc.Mode != config.ModeApps && len(ordered) == 0) {
		c.mu.Unlock()
		return
	}

	c.open = true
	c.session++
	c.presentationSession.Store(c.session)
	c.shortcut = sc
	c.baseList = ordered
	c.list = ordered
	c.baseGroups = groups
	c.groups = groups
	c.search = ""
	c.selected = 0
	c.syncSelectedWindowLocked()
	count := len(ordered)
	if sc.Mode == config.ModeApps {
		count = len(groups)
	}
	if prefs.Behavior.HoldToCycle && count > 1 {
		c.selected = 1 // start on the previous window for instant quick-switch
		c.syncSelectedWindowLocked()
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
	if !c.open || c.selectionCountLocked() == 0 {
		c.mu.Unlock()
		return
	}
	n := c.selectionCountLocked()
	c.selected = ((c.selected+delta)%n + n) % n
	c.syncSelectedWindowLocked()
	st := c.snapshot()
	c.mu.Unlock()
	c.deps.View.Update(st)
}

// Select highlights the entry at index (used by mouse hover). Out-of-range
// indices are ignored.
func (c *Controller) Select(index int) {
	c.mu.Lock()
	if !c.open || index < 0 || index >= c.selectionCountLocked() {
		c.mu.Unlock()
		return
	}
	c.selected = index
	c.syncSelectedWindowLocked()
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
	if c.shortcut.Mode == config.ModeApps {
		c.groups = filterAppGroups(c.baseGroups, query)
		c.selected = 0
		c.syncSelectedWindowLocked()
		st := c.snapshot()
		c.mu.Unlock()
		c.deps.View.Update(st)
		return
	}
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
func (c *Controller) Confirm() error {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return nil
	}
	if c.shortcut.Mode == config.ModeApps {
		if c.selected < 0 || c.selected >= len(c.groups) {
			c.mu.Unlock()
			return errors.New("selected application is no longer available")
		}
		appID := c.groups[c.selected].App.ID
		c.mu.Unlock()
		return c.ConfirmApp(appID)
	}
	if len(c.list) == 0 || c.selected >= len(c.list) {
		c.mu.Unlock()
		return errors.New("selected window is no longer available")
	}
	target := c.list[c.selected]
	session := c.session
	c.mu.Unlock()
	return c.commitWindow(session, target)
}

// ConfirmWindow focuses the requested visible window and closes the overlay.
// The ID is resolved while holding the controller lock so a click does not
// depend on a separate, asynchronous selection update from the frontend.
func (c *Controller) ConfirmWindow(id domain.WindowID) error {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return nil
	}
	visible := c.list
	if c.shortcut.Mode == config.ModeApps {
		visible = nil
		if c.selected >= 0 && c.selected < len(c.groups) {
			visible = c.groups[c.selected].Windows
		}
	}
	var target domain.Window
	for _, window := range visible {
		if window.ID == id {
			target = window
			break
		}
	}
	if target.ID == 0 {
		c.mu.Unlock()
		return errors.New("requested window is not visible in this switcher session")
	}
	session := c.session
	c.mu.Unlock()
	return c.commitWindow(session, target)
}

// NoteFocus records a focus change that happened outside the switcher (a
// click, the Dock, Spotlight, the OS's own ⌘Tab) into the MRU tracker, so
// "recently focused" ordering reflects reality. Events are ignored while the
// overlay is open: mid-cycle reordering would make the visible list jump, and
// Confirm already Touches the window it focuses (the activation echo of that
// focus arrives here a beat later — re-touching the same id is a no-op).
func (c *Controller) NoteFocus(id domain.WindowID) {
	if id == 0 || c.IsOpen() {
		return
	}
	c.deps.MRU.Touch(id)
}

// Cancel closes the overlay without changing focus.
func (c *Controller) Cancel() {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	retiringSession := c.session
	c.reset()
	c.mu.Unlock()
	c.deliverHide(retiringSession)
}

// CloseSelected closes the selected window and refreshes the list.
func (c *Controller) CloseSelected() {
	c.actWindow(func(w domain.Window) { _ = c.deps.Focuser.Close(w.ID) })
}

// MinimizeSelected minimizes the selected window and refreshes the list.
func (c *Controller) MinimizeSelected() {
	c.actWindow(func(w domain.Window) { _ = c.deps.Focuser.Minimize(w.ID) })
}

// FullscreenSelected toggles fullscreen on the selected window and refreshes.
func (c *Controller) FullscreenSelected() {
	c.actWindow(func(w domain.Window) { _ = c.deps.Focuser.Fullscreen(w.ID) })
}

// QuitSelectedApp quits the selected window's application and refreshes.
func (c *Controller) QuitSelectedApp() {
	c.actApp(func(id domain.AppID) { _ = c.deps.Focuser.QuitApp(id) })
}

// HideSelectedApp hides the selected window's application and refreshes.
func (c *Controller) HideSelectedApp() {
	c.actApp(func(id domain.AppID) { _ = c.deps.Focuser.HideApp(id) })
}

// actWindow runs fn on the selected real window (if any) then refreshes.
func (c *Controller) actWindow(fn func(domain.Window)) {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	var w domain.Window
	if c.shortcut.Mode == config.ModeApps {
		if c.selected >= 0 && c.selected < len(c.groups) {
			for _, candidate := range c.groups[c.selected].Windows {
				if candidate.ID == c.selectedWindow {
					w = candidate
					break
				}
			}
		}
	} else if c.selected >= 0 && c.selected < len(c.list) {
		w = c.list[c.selected]
	}
	if w.ID == 0 {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	fn(w)
	c.refresh()
}

func (c *Controller) actApp(fn func(domain.AppID)) {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return
	}
	var id domain.AppID
	if c.shortcut.Mode == config.ModeApps {
		id = c.selectedAppIDLocked()
	} else if c.selected >= 0 && c.selected < len(c.list) {
		id = c.list[c.selected].AppID
	}
	if id == 0 {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	fn(id)
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
	prefs := c.settings.Preferences(c.shortcut.Mode)
	c.baseList = c.composeLocked(wins, c.shortcut.Scope, ctx, prefs)
	if c.shortcut.Mode == config.ModeApps {
		apps, appErr := c.deps.Apps.Apps()
		if appErr != nil {
			c.mu.Unlock()
			return
		}
		oldApp, oldWindow := c.selectedAppIDLocked(), c.selectedWindow
		c.baseGroups = c.composeAppsLocked(apps, wins, c.baseList, c.shortcut.Scope, ctx)
		c.groups = filterAppGroups(c.baseGroups, c.search)
		c.restoreAppSelectionLocked(oldApp, oldWindow)
		if len(c.groups) == 0 {
			retiringSession := c.session
			c.reset()
			c.mu.Unlock()
			c.deliverHide(retiringSession)
			return
		}
		st := c.snapshot()
		c.mu.Unlock()
		c.deps.View.Update(st)
		return
	}
	if c.search == "" {
		c.list = c.baseList
	} else {
		c.list = search.Filter(c.baseList, c.search)
	}
	if len(c.list) == 0 {
		retiringSession := c.session
		c.reset()
		c.mu.Unlock()
		c.deliverHide(retiringSession)
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
func (c *Controller) composeLocked(wins []domain.Window, scope config.ShortcutScope, ctx filter.Context, prefs config.ModePreferences) []domain.Window {
	mode := prefs.Order
	if scope.Order.Valid() {
		mode = scope.Order
	}
	sorted := order.Sort(filter.Apply(wins, c.settings.Filters, scope, ctx), mode)
	return order.SendToBack(sorted, c.settings.Filters)
}

// reset clears the open state. Caller must hold the lock.
func (c *Controller) reset() {
	c.presentationSession.Store(0)
	c.open = false
	c.baseList = nil
	c.list = nil
	c.baseGroups = nil
	c.groups = nil
	c.selectedWindow = 0
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
	prefs := c.settings.Preferences(c.shortcut.Mode)
	style := prefs.Appearance.Style
	if c.open && c.shortcut.StyleOverride != "" {
		style = c.shortcut.StyleOverride
	}
	visible := c.list
	if c.shortcut.Mode == config.ModeApps && c.selected >= 0 && c.selected < len(c.groups) {
		visible = c.groups[c.selected].Windows
	}
	entries := make([]Entry, len(visible))
	for i, w := range visible {
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
		Session:           c.session,
		Mode:              c.shortcut.Mode,
		Apps:              appEntries(c.groups),
		SelectedWindowID:  c.selectedWindow,
		ActionBindings:    maps.Clone(prefs.Behavior.ActionBindings),
		MiddleClickAction: prefs.Behavior.MiddleClickAction,
		SwipeUpAction:     prefs.Behavior.SwipeUpAction,
		SwipeDownAction:   prefs.Behavior.SwipeDownAction,
		Open:              c.open,
		Style:             style,
		Appearance:        prefs.Appearance,
		Placement:         prefs.Placement,
		Entries:           entries,
		Selected:          c.selected,
		Search:            c.search,
		ShortcutID:        c.shortcut.ID,
		VimKeys:           prefs.Behavior.VimKeys,
		ArrowKeys:         prefs.Behavior.ArrowKeys,
		MouseHover:        prefs.Behavior.MouseHoverSelect,
		ActiveSpaceID:     c.activeSpace,
		PlacementScreenID: c.placementScreen,
	}
}
