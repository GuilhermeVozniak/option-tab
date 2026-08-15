package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	goruntime "runtime"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/update"
)

// appVersion is shown in the About tab.
const appVersion = "0.4.2"

// projectURL and releasesURL are the About-tab links.
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
	if a.wailsApp != nil {
		_ = a.wailsApp.Browser.OpenURL(url)
	}
}

// updatesDeepLink targets the Updates section of the General tab, where the
// update policy, the manual check and its result live. The preferences UI
// parses the "<tab>#<section>" form.
const updatesDeepLink = "General#updates"

// CheckForUpdates runs a release check now and reveals the Updates section of
// the General tab, where the check's outcome appears (the install banner is
// app-level chrome, shown on every tab). The manual check never auto-installs,
// even under the "auto" policy.
func (a *App) CheckForUpdates() {
	go a.checkForUpdate(false)
	a.openPreferencesTab(updatesDeepLink)
}

// InstallUpdate downloads and installs the release the update banner shows,
// then relaunches the app. Progress rides the "update:progress" event.
func (a *App) InstallUpdate() {
	a.updateMu.Lock()
	rel := a.pendingUpdate
	a.updateMu.Unlock()
	if rel == nil {
		return
	}
	go a.installRelease(*rel)
}

// updateLoop checks for a newer release shortly after launch and then daily,
// honoring Behavior.UpdatePolicy. A hit emits "update:available" so the
// preferences UI can show the install banner; under the "auto" policy the
// install (and relaunch) runs without further confirmation.
func (a *App) updateLoop() {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for range timer.C {
		if a.settings.Behavior.UpdatePolicy != config.UpdatesOff {
			a.checkForUpdate(a.settings.Behavior.UpdatePolicy == config.UpdatesAuto)
		}
		timer.Reset(24 * time.Hour)
	}
}

// checkForUpdate fetches the latest release and always emits "update:checked"
// with the outcome, so a manual check can answer "you're up to date" or
// "could not check" instead of silence. A newer release is also recorded as
// the pending update and emits "update:available" for the install banner;
// autoInstall triggers the full self-install immediately ("auto" policy,
// background loop only).
func (a *App) checkForUpdate(autoInstall bool) {
	rel, err := a.fetchLatestRelease()
	if err != nil {
		dlog("update: check failed: %v", err)
		a.emit("update:checked", map[string]any{"available": false, "error": err.Error()})
		return
	}
	if !update.Newer(appVersion, rel.Version) {
		a.emit("update:checked", map[string]any{"latest": rel.Version, "available": false})
		return
	}
	dlog("update: %s available at %s", rel.Version, rel.URL)
	a.updateMu.Lock()
	a.pendingUpdate = &rel
	a.updateMu.Unlock()
	a.emit("update:checked", map[string]any{"latest": rel.Version, "available": true})
	a.emit("update:available", map[string]string{"version": rel.Version, "url": rel.URL})
	if autoInstall {
		a.installRelease(rel)
	}
}

// fetchLatestRelease fetches and parses the newest published GitHub release.
func (a *App) fetchLatestRelease() (update.Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(updateCheckURL)
	if err != nil {
		return update.Release{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// e.g. no releases yet (404) or rate-limited.
		return update.Release{}, fmt.Errorf("update: check: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return update.Release{}, err
	}
	return update.ParseLatest(body)
}

// installRelease downloads rel and self-installs it, emitting
// "update:progress" stages (downloading → installing → restarting; "error"
// with a message on failure) so the preferences UI can follow along. On
// non-macOS stub builds it falls back to opening the release page.
func (a *App) installRelease(rel update.Release) {
	a.updateMu.Lock()
	if a.updateInstalling {
		a.updateMu.Unlock()
		return
	}
	a.updateInstalling = true
	a.updateMu.Unlock()
	defer func() {
		a.updateMu.Lock()
		a.updateInstalling = false
		a.updateMu.Unlock()
	}()

	if goruntime.GOOS != "darwin" {
		a.OpenURL(rel.URL)
		return
	}

	a.emitUpdateProgress("downloading", "")
	dmg, err := a.downloadUpdate(rel)
	if err != nil {
		dlog("update: download failed: %v", err)
		a.emitUpdateProgress("error", err.Error())
		return
	}
	defer func() { _ = os.Remove(dmg) }()

	a.emitUpdateProgress("installing", "")
	appPath, err := update.InstallDMG(dmg)
	if err != nil {
		dlog("update: install failed: %v", err)
		a.emitUpdateProgress("error", err.Error())
		return
	}

	dlog("update: installed %s over %s; relaunching", rel.Version, appPath)
	a.emitUpdateProgress("restarting", "")
	if err := update.RelaunchSelf(appPath, os.Getpid()); err != nil {
		a.emitUpdateProgress("error", err.Error())
		return
	}
	if a.wailsApp != nil {
		a.wailsApp.Quit()
	}
}

func (a *App) emitUpdateProgress(stage, message string) {
	a.emit("update:progress", map[string]string{"stage": stage, "message": message})
}

// downloadUpdate fetches the release's macOS installer into a temp file and
// returns its path; the caller removes it.
func (a *App) downloadUpdate(rel update.Release) (string, error) {
	url := rel.AssetFor("darwin_" + goruntime.GOARCH)
	if url == "" {
		return "", fmt.Errorf("update: no darwin_%s asset in %s", goruntime.GOARCH, rel.Version)
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update: download: %s", resp.Status)
	}
	f, err := os.CreateTemp("", "option-tab-*.dmg")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), f.Close()
}
