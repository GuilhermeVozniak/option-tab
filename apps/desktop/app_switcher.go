package main

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

// ---- Hotkey wiring ----

func (a *App) registerHotkeys() {
	eng := a.platform.Hotkeys()
	for _, sc := range a.settings.Shortcuts {
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

func (a *App) reRegisterHotkeys() {
	eng := a.platform.Hotkeys()
	for i := 1; i <= config.MaxShortcuts; i++ {
		_ = eng.Unregister(i)
	}
	a.registerHotkeys()
}

func (a *App) hotkeyLoop() {
	for ev := range a.platform.Hotkeys().Events() {
		dlog("hotkeyLoop: received event kind=%d shortcut=%d", ev.Kind, ev.ShortcutID)
		a.controller.HandleHotkey(ev)
	}
}

// ---- switcher.View ----

// Show reveals the overlay window and pushes the initial state. If the
// preferences window is open it is dismissed first, since both share the
// single Wails window.
func (a *App) Show(st switcher.State) {
	dlog("Show: %d entries, selected=%d", len(st.Entries), st.Selected)
	if a.prefsOpen {
		a.prefsOpen = false
		a.emit("prefs:close", nil)
		if wm, ok := a.platform.(platform.WindowModer); ok {
			wm.SetPrefsWindowMode(false)
		}
	}
	a.enrichIcons(&st)
	a.emit("switcher:show", st)
	a.lastSelected = st.Selected
	// Size the transparent window to the screen the Placement setting chose
	// (resolved by the controller) so the panel appears there and lays out
	// against the real screen size.
	fitted := false
	if f, ok := a.platform.(platform.OverlayWindowPreparer); ok {
		f.FitOverlayToScreen(st.PlacementScreenID)
		fitted = true
	}
	if a.ctx != nil {
		// FitOverlayToScreen already positions the window on the target screen, so
		// no WindowCenter there (it could move it onto the wrong display). On
		// backends without a fit path, center so it isn't shown at a stale spot.
		if !fitted {
			runtime.WindowCenter(a.ctx)
		}
		runtime.WindowShow(a.ctx)
	}
	a.emitCachedThumbnails(st)
	a.captureThumbnails(st)
	a.capturePreview(st)
}

// Update pushes a new state to the visible overlay.
func (a *App) Update(st switcher.State) {
	a.enrichIcons(&st)
	a.emit("switcher:update", st)
	if st.Selected != a.lastSelected {
		a.lastSelected = st.Selected
		if h, ok := a.platform.(platform.HapticFeedback); ok && a.settings.Behavior.HapticFeedback {
			h.HapticTick()
		}
	}
	a.capturePreview(st)
}

// Hide pushes the hide event and hides the overlay window.
func (a *App) Hide() {
	atomic.AddInt64(&a.thumbGen, 1) // invalidate any in-flight thumbnail capture
	a.emit("switcher:hide", nil)
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
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
		a.emit("switcher:thumbnails", out)
	}
}

// backgroundCaptureLoop keeps the thumbnail cache fresh while the switcher is
// hidden, when Behavior.CaptureInBackground is enabled. It shows the macOS
// screen-recording indicator while capturing, which is why it is opt-in.
func (a *App) backgroundCaptureLoop() {
	const maxWindows = 30
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !a.settings.Behavior.CaptureInBackground || a.controller.IsOpen() || a.controller.Paused() {
			continue
		}
		src, ok := a.platform.(platform.ThumbnailSource)
		if !ok {
			continue
		}
		wins, err := a.platform.Windows()
		if err != nil {
			continue
		}
		px := a.settings.Appearance.ThumbnailMaxPx
		if px <= 0 {
			px = 256
		}
		for i, w := range wins {
			if i >= maxWindows || a.controller.IsOpen() {
				break
			}
			url := src.ThumbnailDataURL(w.ID, px)
			if url == "" {
				continue
			}
			a.thumbCacheMu.Lock()
			a.thumbCache[w.ID] = url
			a.thumbCacheMu.Unlock()
		}
	}
}

// captureThumbnails snapshots each window off the hotkey path and streams the
// results to the overlay via "switcher:thumbnails" events, so the switcher
// appears instantly (with icons) and previews fill in as they are captured.
// Each Show/Hide bumps thumbGen; a stale goroutine stops emitting.
func (a *App) captureThumbnails(st switcher.State) {
	if st.Style != config.StyleThumbnails {
		return
	}
	src, ok := a.platform.(platform.ThumbnailSource)
	if !ok {
		return
	}
	px := st.Appearance.ThumbnailMaxPx
	if px <= 0 {
		px = 256
	}
	gen := atomic.AddInt64(&a.thumbGen, 1)
	// Capture the selected window first, and capture in parallel (bounded) so
	// all previews appear together quickly instead of streaming in one by one.
	// Concurrency is modest: each capture is a full ScreenCaptureKit enumeration,
	// so too many at once contend on the window server and risk timing out.
	entries := switcher.OrderSelectedFirst(st.Entries, st.Selected)
	const maxConcurrent = 4
	go func() {
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup
		for _, e := range entries {
			if atomic.LoadInt64(&a.thumbGen) != gen {
				break
			}
			sem <- struct{}{}
			wg.Add(1)
			go func(e switcher.Entry) {
				defer wg.Done()
				defer func() { <-sem }()
				if atomic.LoadInt64(&a.thumbGen) != gen {
					return
				}
				url := src.ThumbnailDataURL(e.WindowID, px)
				if url == "" || atomic.LoadInt64(&a.thumbGen) != gen {
					return
				}
				a.emit("switcher:thumbnails", map[string]string{strconv.Itoa(int(e.WindowID)): url})
			}(e)
		}
		wg.Wait()
	}()
}

// capturePreview captures a high-resolution snapshot of the selected window
// when "preview selected window" is enabled, streamed via "switcher:preview".
// Stale captures are dropped via the same generation counter as thumbnails.
func (a *App) capturePreview(st switcher.State) {
	if !st.Appearance.PreviewSelected || st.Selected < 0 || st.Selected >= len(st.Entries) {
		return
	}
	src, ok := a.platform.(platform.ThumbnailSource)
	if !ok {
		return
	}
	id := st.Entries[st.Selected].WindowID
	gen := atomic.LoadInt64(&a.thumbGen)
	go func() {
		dataURL := src.ThumbnailDataURL(id, 1024)
		if dataURL == "" || atomic.LoadInt64(&a.thumbGen) != gen {
			return
		}
		a.emit("switcher:preview", map[string]string{strconv.Itoa(int(id)): dataURL})
	}()
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
}

// ---- Bound controller actions (called from the frontend) ----

func (a *App) Advance()            { a.controller.Advance() }
func (a *App) Reverse()            { a.controller.Reverse() }
func (a *App) Confirm()            { a.controller.Confirm() }
func (a *App) Cancel()             { a.controller.Cancel() }
func (a *App) Select(index int)    { a.controller.Select(index) }
func (a *App) SetSearch(q string)  { a.controller.SetSearch(q) }
func (a *App) CloseSelected()      { a.controller.CloseSelected() }
func (a *App) MinimizeSelected()   { a.controller.MinimizeSelected() }
func (a *App) FullscreenSelected() { a.controller.FullscreenSelected() }
func (a *App) QuitSelectedApp()    { a.controller.QuitSelectedApp() }
func (a *App) HideSelectedApp()    { a.controller.HideSelectedApp() }
