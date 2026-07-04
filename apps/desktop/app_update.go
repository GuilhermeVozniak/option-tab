package main

import (
	"io"
	"net/http"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"option-tab/internal/config"
	"option-tab/internal/update"
)

// appVersion is shown in the About tab.
const appVersion = "0.1.0"

// projectURL and releasesURL are the About-tab links; the free clone has no
// auto-updater, so "check for updates" opens the releases page.
const (
	projectURL  = "https://github.com/GuilhermeVozniak/option-tab"
	releasesURL = projectURL + "/releases"
)

// updateCheckURL is GitHub's "latest release" endpoint for this repo.
const updateCheckURL = "https://api.github.com/repos/GuilhermeVozniak/option-tab/releases/latest"

// GetVersion returns the app version for the About tab.
func (a *App) GetVersion() string { return appVersion }

// OpenURL opens a link in the user's default browser (About-tab links).
func (a *App) OpenURL(url string) {
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// CheckForUpdates opens the releases page in the browser (manual check).
func (a *App) CheckForUpdates() { a.OpenURL(releasesURL) }

// updateLoop checks for a newer release shortly after launch and then daily,
// honoring Behavior.UpdatePolicy. A hit emits "update:available" so the
// preferences UI can show a download banner. There is no silent auto-install:
// the "auto" policy behaves like "check" until a signed updater exists.
func (a *App) updateLoop() {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for range timer.C {
		if a.settings.Behavior.UpdatePolicy != config.UpdatesOff {
			a.checkForUpdateOnce()
		}
		timer.Reset(24 * time.Hour)
	}
}

func (a *App) checkForUpdateOnce() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateCheckURL)
	if err != nil {
		dlog("update: check failed: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return // e.g. no releases yet (404) or rate-limited
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return
	}
	rel, err := update.ParseLatest(body)
	if err != nil || !update.Newer(appVersion, rel.Version) {
		return
	}
	dlog("update: %s available at %s", rel.Version, rel.URL)
	a.emit("update:available", map[string]string{"version": rel.Version, "url": rel.URL})
}
