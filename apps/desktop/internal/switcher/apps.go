package switcher

import (
	"errors"
	"sort"
	"strings"

	"option-tab/internal/appgroup"
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/platform"
	"option-tab/internal/search"
)

// AppEntry represents one exact running process. Windows is the count of
// eligible preview entries, not a claim about all native windows.
type AppEntry struct {
	AppID          domain.AppID            `json:"appId"`
	AppName        string                  `json:"appName"`
	BundleID       string                  `json:"bundleId"`
	Hidden         bool                    `json:"hidden"`
	WindowCount    int                     `json:"windowCount"`
	WindowPresence platform.WindowPresence `json:"windowPresence,omitempty"`
	Icon           string                  `json:"icon,omitempty"`
}

func (c *Controller) composeAppsLocked(apps []domain.App, raw, eligible []domain.Window, scope config.ShortcutScope, ctx filter.Context) []appgroup.Group {
	var groups []appgroup.Group
	if c.deps.AppWindows == nil {
		groups = appgroup.Compose(apps, raw, eligible)
	} else {
		eligibleApps := make(map[domain.AppID]bool)
		for _, window := range eligible {
			if window.ID != 0 {
				eligibleApps[window.AppID] = true
			}
		}
		presence := make(map[domain.AppID]platform.WindowPresence)
		for _, app := range apps {
			if app.ID > 0 && !eligibleApps[app.ID] {
				presence[app.ID] = c.deps.AppWindows.AppWindowPresence(app.ID)
			}
		}
		groups = appgroup.ComposeWithPresence(apps, raw, eligible, presence)
	}
	counts := make(map[domain.AppID]int)
	for _, window := range raw {
		if window.ID != 0 {
			counts[window.AppID]++
		}
	}
	out := groups[:0]
	for _, group := range groups {
		rawCount := counts[group.App.ID]
		switch group.Presence {
		case platform.WindowsUnknown:
			rawCount = -1
		case platform.WindowsNone:
			rawCount = 0
		}
		if filter.AppAllowed(group.App, rawCount, c.settings.Filters, scope, ctx) {
			out = append(out, group)
		}
	}
	return out
}

func appEntries(groups []appgroup.Group) []AppEntry {
	out := make([]AppEntry, len(groups))
	for i, group := range groups {
		out[i] = AppEntry{AppID: group.App.ID, AppName: group.App.Name, BundleID: group.App.BundleID, Hidden: group.App.Hidden, WindowCount: len(group.Windows), WindowPresence: group.Presence}
	}
	return out
}

func filterAppGroups(groups []appgroup.Group, query string) []appgroup.Group {
	if strings.TrimSpace(query) == "" {
		return append([]appgroup.Group(nil), groups...)
	}
	type ranked struct {
		group        appgroup.Group
		score, order int
	}
	matches := make([]ranked, 0, len(groups))
	for i, group := range groups {
		best, ok := search.Match(group.App.Name, query)
		for _, window := range group.Windows {
			if score, match := search.Match(window.Title, query); match && (!ok || score > best) {
				best, ok = score, true
			}
		}
		if ok {
			matches = append(matches, ranked{group: group, score: best, order: i})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].order < matches[j].order
	})
	out := make([]appgroup.Group, len(matches))
	for i := range matches {
		out[i] = matches[i].group
	}
	return out
}

func (c *Controller) selectionCountLocked() int {
	if c.shortcut.Mode == config.ModeApps {
		return len(c.groups)
	}
	return len(c.list)
}

func (c *Controller) selectedAppIDLocked() domain.AppID {
	if c.shortcut.Mode == config.ModeApps && c.selected >= 0 && c.selected < len(c.groups) {
		return c.groups[c.selected].App.ID
	}
	return 0
}

func (c *Controller) syncSelectedWindowLocked() {
	if c.shortcut.Mode != config.ModeApps || c.selected < 0 || c.selected >= len(c.groups) {
		c.selectedWindow = 0
		return
	}
	windows := c.groups[c.selected].Windows
	for _, window := range windows {
		if window.ID == c.selectedWindow {
			return
		}
	}
	if len(windows) > 0 {
		c.selectedWindow = windows[0].ID
	} else {
		c.selectedWindow = 0
	}
}

func (c *Controller) restoreAppSelectionLocked(appID domain.AppID, windowID domain.WindowID) {
	c.selected = 0
	for i, group := range c.groups {
		if group.App.ID == appID {
			c.selected = i
			break
		}
	}
	c.selectedWindow = windowID
	c.syncSelectedWindowLocked()
}

func (c *Controller) SelectApp(id domain.AppID) {
	c.mu.Lock()
	if !c.open || c.shortcut.Mode != config.ModeApps {
		c.mu.Unlock()
		return
	}
	for i, group := range c.groups {
		if group.App.ID == id {
			c.selected = i
			c.selectedWindow = 0
			c.syncSelectedWindowLocked()
			st := c.snapshot()
			c.mu.Unlock()
			c.deps.View.Update(st)
			return
		}
	}
	c.mu.Unlock()
}

func (c *Controller) SelectAppWindow(id domain.WindowID) {
	c.mu.Lock()
	if !c.open || c.shortcut.Mode != config.ModeApps || c.selected < 0 || c.selected >= len(c.groups) {
		c.mu.Unlock()
		return
	}
	for _, window := range c.groups[c.selected].Windows {
		if window.ID == id {
			c.selectedWindow = id
			st := c.snapshot()
			c.mu.Unlock()
			c.deps.View.Update(st)
			return
		}
	}
	c.mu.Unlock()
}

func (c *Controller) ConfirmApp(id domain.AppID) error {
	c.mu.Lock()
	if !c.open {
		c.mu.Unlock()
		return nil
	}
	if c.shortcut.Mode != config.ModeApps {
		c.mu.Unlock()
		return errors.New("application confirmation requires app mode")
	}
	var target appgroup.Group
	for _, group := range c.groups {
		if group.App.ID == id {
			target = group
			break
		}
	}
	if target.App.ID == 0 {
		c.mu.Unlock()
		return errors.New("requested application is not visible in this switcher session")
	}
	windowID := domain.WindowID(0)
	if id == c.selectedAppIDLocked() {
		windowID = c.selectedWindow
	}
	if windowID == 0 && len(target.Windows) > 0 {
		windowID = target.Windows[0].ID
	}
	var window domain.Window
	for _, candidate := range target.Windows {
		if candidate.ID == windowID {
			window = candidate
			break
		}
	}
	session := c.session
	c.mu.Unlock()
	if window.ID != 0 {
		return c.commitWindow(session, window)
	}
	apps, err := c.deps.Apps.Apps()
	if err != nil {
		return err
	}
	found := false
	for _, app := range apps {
		if app.ID == id && app.BundleID == target.App.BundleID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("application is no longer running")
	}
	if c.PresentationSession() != session {
		return errors.New("switcher session is no longer current")
	}
	if err := c.deps.AppActivator.ActivateApp(id); err != nil {
		return err
	}
	c.finishCommit(session, 0)
	return nil
}

func (c *Controller) commitWindow(session uint64, target domain.Window) error {
	c.mu.Lock()
	if !c.open || c.session != session {
		c.mu.Unlock()
		return errors.New("switcher session is no longer current")
	}
	follow := c.settings.Preferences(c.shortcut.Mode).Behavior.CursorFollowFocus
	c.mu.Unlock()
	windows, err := c.deps.Windows.Windows()
	if err != nil {
		return err
	}
	found := false
	for _, window := range windows {
		if window.ID == target.ID && window.AppID == target.AppID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("window no longer exists or application identity changed")
	}
	if c.PresentationSession() != session {
		return errors.New("switcher session is no longer current")
	}
	// The optional native action path acknowledges AX raise failures. The
	// legacy Focuser may only submit a best-effort activation request.
	if exact, ok := c.deps.Focuser.(interface {
		PerformTargetAction(string, domain.WindowID, domain.AppID) error
	}); ok {
		err = exact.PerformTargetAction("focus", target.ID, target.AppID)
	} else {
		err = c.deps.Focuser.Focus(target.ID)
	}
	if err != nil {
		return err
	}
	c.deps.MRU.Touch(target.ID)
	if follow && c.deps.Cursor != nil && c.PresentationSession() == session {
		_ = c.deps.Cursor.WarpCursorToWindow(target.ID)
	}
	c.finishCommit(session, target.ID)
	return nil
}

func (c *Controller) finishCommit(session uint64, _ domain.WindowID) {
	c.mu.Lock()
	if !c.open || c.session != session {
		c.mu.Unlock()
		return
	}
	c.reset()
	c.mu.Unlock()
	c.deliverHide(session)
}
