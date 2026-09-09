package main

import (
	"strconv"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

// ---- Hotkey wiring ----

func (a *App) registerHotkeys() {
	eng := a.platform.Hotkeys()
	settings := a.settingsSnapshot()
	a.syncHotkeyPolicy(settings)
	for _, sc := range settings.Shortcuts {
		if !sc.Enabled {
			continue
		}
		chord, err := hotkey.Parse(sc.Chord)
		if err != nil {
			continue
		}
		dlog("registerHotkeys: registering shortcut %d chord=%q", sc.ID, sc.Chord)
		if rerr := eng.Register(sc.ID, chord); rerr != nil {
			dlog("registerHotkeys: register error: %v", rerr)
		}
	}
}

func (a *App) syncHotkeyPolicy(settings config.Settings) {
	updater, ok := a.platform.Hotkeys().(platform.HotkeyPolicyUpdater)
	if !ok {
		return
	}
	ignoredApps := make([]string, 0, len(settings.Filters.AppBlacklist))
	for _, entry := range settings.Filters.AppBlacklist {
		if entry.IgnoreShortcuts && entry.Match != "" {
			ignoredApps = append(ignoredApps, entry.Match)
		}
	}
	updater.SetHotkeyPolicy(platform.HotkeyPolicy{
		Enabled:     !settings.Behavior.Paused,
		IgnoredApps: ignoredApps,
	})
}

func (a *App) reRegisterHotkeys() {
	eng := a.platform.Hotkeys()
	for i := 1; i <= config.MaxShortcuts; i++ {
		_ = eng.Unregister(i)
	}
	a.registerHotkeys()
}

func (a *App) hotkeyLoop() {
	for {
		select {
		case <-a.captureStop:
			return
		case ev, ok := <-a.platform.Hotkeys().Events():
			if !ok {
				return
			}
			dlog("hotkeyLoop: received event kind=%d shortcut=%d", ev.Kind, ev.ShortcutID)
			a.controller.HandleHotkey(ev)
		}
	}
}

// focusLoop feeds real focus changes (clicks, Dock, Spotlight, the OS's own
// ⌘Tab) from the platform into the MRU tracker, so "recently focused"
// ordering reflects reality instead of only switches made through the overlay.
func (a *App) focusLoop() {
	src, ok := a.platform.(platform.FocusEventSource)
	if !ok {
		return // backend without focus observation (stub)
	}
	for id := range src.FocusEvents() {
		a.controller.NoteFocus(id)
	}
}

// keyLoop forwards raw key presses the native tap captured while the overlay
// was open to the frontend. The overlay window never becomes key (the app is
// not activated on show), so this is the overlay's only keyboard source.
func (a *App) keyLoop() {
	keys := a.platform.Hotkeys().Keys()
	if keys == nil {
		return // backend without key forwarding (stub/fake)
	}
	for {
		select {
		case <-a.captureStop:
			return
		case ev, ok := <-keys:
			if !ok {
				return
			}
			if session := a.controller.PresentationSession(); session != 0 && ev.Session == session {
				a.emit("switcher:key", ev)
			}
		}
	}
}

func (a *App) setKeySession(session uint64) {
	if source, ok := a.platform.Hotkeys().(platform.KeySessionSetter); ok {
		source.SetKeySession(session)
	}
}

// ---- switcher.View ----

// Show reveals the overlay window and pushes the initial state. If the
// preferences window is open it is dismissed first. The overlay is shown
// WITHOUT activating the app (v3 Show is a bare makeKeyAndOrderFront): the
// previously active app keeps focus, so the active-app scope filter keeps
// seeing the real frontmost app — activating here was the v2 switching bug.
func (a *App) Show(st switcher.State) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.sessionInactive || (st.Session != 0 && a.controller.PresentationSession() != st.Session) {
		return
	}
	select {
	case <-a.captureStop:
		return
	default:
	}
	a.cancelDismissalLocked()
	a.switcherVisible = true
	a.visibleSwitcherSession = st.Session
	a.syncDockSuspensionLocked()
	// Wait out the prior owner's emissions before assigning a new frame token.
	a.captures.Hide()
	a.captureSwitcherSession.Store(st.Session)
	a.fadeOnHide = st.Appearance.FadeOutAnimation
	a.captureActive = true
	dlog("Show: %d entries, selected=%d", len(st.Entries), st.Selected)
	if a.prefsOpen {
		a.closePreferencesWindow()
	}
	// Tell the native tap to consume and forward all keyboard input before the
	// window appears, so a quick follow-up Tab never leaks to the previous app.
	a.setKeySession(st.Session)
	a.platform.Hotkeys().SetOpen(true)
	a.enrichIcons(&st)
	a.switcherRevision++
	st.Revision = a.switcherRevision
	a.emit("switcher:show", st)
	a.lastSelected = st.Selected
	// Size the transparent window to the screen the Placement setting chose
	// (resolved by the controller) so the panel appears there and lays out
	// against the real screen size.
	if a.overlay.alive() {
		if f, ok := a.platform.(platform.OverlayWindowFitter); ok {
			if native := a.overlay.native(); native != nil {
				f.FitOverlayToScreen(native, st.PlacementScreenID)
			}
		}
		// Re-assert the floating level on every show: Wails v3 applies
		// AlwaysOnTop from a WindowDidBecomeKey handler, which never fires for
		// this never-activated window, so without this it stays at normal level
		// and slides behind other apps' windows.
		a.overlay.setAlwaysOnTop(true)
		a.overlay.show()
	}
	a.syncSwitcherMaterialLocked(st)
	a.emitCachedThumbnails(st)
	a.updateCapture(st)
}

// Update pushes a new state to the visible overlay.
func (a *App) Update(st switcher.State) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.sessionInactive || (st.Session != 0 && a.controller.PresentationSession() != st.Session) {
		return
	}
	a.enrichIcons(&st)
	a.switcherRevision++
	st.Revision = a.switcherRevision
	a.emit("switcher:update", st)
	a.syncSwitcherMaterialLocked(st)
	if st.Selected != a.lastSelected {
		a.lastSelected = st.Selected
		if h, ok := a.platform.(platform.HapticFeedback); ok && a.settingsSnapshot().Preferences(st.Mode).Behavior.HapticFeedback {
			h.HapticTick()
		}
	}
	a.updateCapture(st)
}

// Hide pushes the hide event and hides the overlay window.
func (a *App) Hide() {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.hideSwitcherLocked()
}

// HideSession retires only the presentation that requested dismissal. An old
// controller callback must not hide an overlay that has since reopened.
func (a *App) HideSession(session uint64) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if session != a.visibleSwitcherSession {
		return
	}
	a.hideSwitcherLocked()
}

func (a *App) hideSwitcherLocked() {
	select {
	case <-a.captureStop:
		return
	default:
	}
	a.cancelDismissalLocked()
	fade := 0
	if a.fadeOnHide {
		fade = 180
	}
	a.retireSwitcherMaterialLocked(fade)
	a.switcherVisible = false
	retiredSession := a.visibleSwitcherSession
	a.visibleSwitcherSession = 0
	a.syncDockSuspensionLocked()
	a.captureActive = false
	a.captures.Hide() // invalidate callbacks before the overlay disappears
	a.captureSwitcherSession.Store(0)
	a.platform.Hotkeys().SetOpen(false)
	a.setKeySession(0)
	a.emitSwitcherHideLocked(retiredSession)
	if a.fadeOnHide && a.overlay.alive() {
		generation := a.viewGeneration
		// Match Overlay's 180 ms CSS fade; input and capture stop immediately.
		a.dismissal = time.AfterFunc(180*time.Millisecond, func() {
			a.viewMu.Lock()
			defer a.viewMu.Unlock()
			if generation != a.viewGeneration {
				return
			}
			a.dismissal = nil
			a.finishHideLocked()
		})
		return
	}
	a.finishHideLocked()
}

func (a *App) emitSwitcherHideLocked(session uint64) {
	a.switcherRevision++
	a.emit("switcher:hide", dockSessionEvent{Session: session, Revision: a.switcherRevision})
}

// cancelDismissalLocked also invalidates callbacks already waiting on viewMu.
func (a *App) cancelDismissalLocked() {
	a.viewGeneration++
	if a.dismissal != nil {
		a.dismissal.Stop()
		a.dismissal = nil
	}
}

func (a *App) finishHideLocked() {
	a.overlay.hide()
	// Clicking the overlay activates the app (a plain NSWindow can't avoid it);
	// drop that activation so focus returns to the previously active app.
	if act, ok := a.platform.(platform.AppActivator); ok && !a.prefsOpen {
		act.HideAppIfActive()
	}
}

// ---- Capture streaming (thumbnails, previews, icons) ----

// emitCachedThumbnails pushes background-captured thumbnails for the shown
// entries in one event, so the switcher paints with fresh previews instantly
// (the live capture pass then replaces them).
func (a *App) emitCachedThumbnails(st switcher.State) {
	a.thumbCacheMu.Lock()
	defer a.thumbCacheMu.Unlock()
	if len(a.thumbCache) == 0 {
		return
	}
	out := map[string]string{}
	for _, e := range st.Entries {
		if url, ok := a.thumbCache[e.WindowID]; ok {
			out[strconv.Itoa(int(e.WindowID))] = url
		}
	}
	if len(out) > 0 {
		a.emit("switcher:thumbnails", switcherFramePayload(st.Session, out))
	}
}

// backgroundCaptureLoop keeps the thumbnail cache fresh while the switcher is
// hidden, when Behavior.CaptureInBackground is enabled. It shows the macOS
// screen-recording indicator while capturing, which is why it is opt-in.
func (a *App) backgroundCaptureLoop() {
	const maxWindows = 30
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.captureStop:
			return
		case <-ticker.C:
		}
		settings := a.settingsSnapshot()
		if !a.backgroundCaptureAllowed() {
			continue
		}
		epoch := a.backgroundCaptureEpoch()
		src, ok := a.platform.(platform.ThumbnailSource)
		if !ok {
			continue
		}
		wins, err := a.platform.Windows()
		if err != nil {
			continue
		}
		px := settings.Appearance.ThumbnailMaxPx
		if px <= 0 {
			px = 256
		}
		next := map[domain.WindowID]string{}
		for i, w := range wins {
			select {
			case <-a.captureStop:
				return
			default:
			}
			if !a.backgroundCaptureAllowed() {
				break
			}
			if i >= maxWindows {
				break
			}
			url := src.ThumbnailDataURL(w.ID, px)
			if url == "" {
				continue
			}
			next[w.ID] = url
		}
		a.publishBackgroundCache(epoch, next)
	}
}

// updateCapture shares one selected-window stream between thumbnail and preview.
// Caller holds viewMu, making active admission atomic with Hide and shutdown.
func (a *App) updateCapture(st switcher.State) {
	if !a.captureActive {
		return
	}
	select {
	case <-a.captureStop:
		return
	default:
	}
	thumbs := st.Style == config.StyleThumbnails || st.Mode == config.ModeApps
	selected := domain.WindowID(0)
	if st.Mode == config.ModeApps {
		selected = st.SelectedWindowID
	} else if st.Selected >= 0 && st.Selected < len(st.Entries) {
		selected = st.Entries[st.Selected].WindowID
	}
	a.captureSelected.Store(uint64(selected))
	a.capturePreviewEnabled.Store(st.Appearance.PreviewSelected)
	a.captureThumbnailsEnabled.Store(thumbs)
	ids := []domain.WindowID{}
	if thumbs {
		for _, entry := range st.Entries {
			ids = append(ids, entry.WindowID)
		}
	} else if st.Appearance.PreviewSelected && selected != 0 {
		ids = append(ids, selected)
	}
	px := st.Appearance.ThumbnailMaxPx
	if px <= 0 {
		px = 256
	}
	if st.Appearance.PreviewSelected {
		px = 1024
	}
	a.captures.Update(ids, selected, px)
}

// enrichIcons fills each entry's Icon with the owning app's icon (a base64 PNG
// data URL), cached by pid. It is a no-op when the platform provides no icons
// (stub/fake), so the overlay falls back to letter glyphs.
func (a *App) enrichIcons(st *switcher.State) {
	src, ok := a.platform.(platform.IconSource)
	if !ok {
		return
	}
	px := st.Appearance.IconSizePx * 2 // render at 2x for retina crispness
	if px <= 0 {
		px = 64
	}
	a.iconMu.Lock()
	defer a.iconMu.Unlock()
	if a.iconCache == nil {
		a.iconCache = make(map[int]string)
	}
	for i := range st.Entries {
		pid := int(st.Entries[i].AppID)
		img, cached := a.iconCache[pid]
		if !cached {
			img = src.AppIcon(pid, px)
			a.iconCache[pid] = img
		}
		st.Entries[i].Icon = img
	}
	for i := range st.Apps {
		pid := int(st.Apps[i].AppID)
		img, cached := a.iconCache[pid]
		if !cached {
			img = src.AppIcon(pid, px)
			a.iconCache[pid] = img
		}
		st.Apps[i].Icon = img
	}
}

// ---- Bound controller actions (called from the frontend) ----

func (a *App) Advance()       { a.controller.Advance() }
func (a *App) Reverse()       { a.controller.Reverse() }
func (a *App) Confirm() error { return a.controller.Confirm() }

func (a *App) ConfirmWindow(id uint64) error { return a.controller.ConfirmWindow(domain.WindowID(id)) }

func (a *App) SelectApp(id int)            { a.controller.SelectApp(domain.AppID(id)) }
func (a *App) SelectAppWindow(id uint64)   { a.controller.SelectAppWindow(domain.WindowID(id)) }
func (a *App) ConfirmApp(id int) error     { return a.controller.ConfirmApp(domain.AppID(id)) }
func (a *App) ActionFailed(message string) { a.emit("switcher:error", message) }

func (a *App) Cancel()             { a.controller.Cancel() }
func (a *App) Select(index int)    { a.controller.Select(index) }
func (a *App) SetSearch(q string)  { a.controller.SetSearch(q) }
func (a *App) CloseSelected()      { a.controller.CloseSelected() }
func (a *App) MinimizeSelected()   { a.controller.MinimizeSelected() }
func (a *App) FullscreenSelected() { a.controller.FullscreenSelected() }
func (a *App) QuitSelectedApp()    { a.controller.QuitSelectedApp() }
func (a *App) HideSelectedApp()    { a.controller.HideSelectedApp() }
