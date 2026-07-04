package main

import (
	"net/url"
	"path/filepath"

	"option-tab/internal/config"
	"option-tab/internal/crash"
)

// crashDir is where crash logs live (next to the settings file).
func (a *App) crashDir() string {
	if a.settingsPath == "" {
		return ""
	}
	return filepath.Dir(a.settingsPath)
}

// setupCrashCapture rotates any crash log left by the previous run to pending
// and arms capture for this run — unless the policy is "never", which also
// clears any leftovers. Nothing is ever transmitted automatically.
func (a *App) setupCrashCapture() {
	dir := a.crashDir()
	if dir == "" {
		return
	}
	if a.settings.Behavior.CrashReports == config.CrashNever {
		_ = crash.Dismiss(dir)
		return
	}
	if err := crash.Rotate(dir); err != nil {
		dlog("crash: rotate: %v", err)
	}
	if err := crash.Arm(dir); err != nil {
		dlog("crash: arm: %v", err)
	}
}

// GetCrashReport returns the previous run's crash log, or "" when there is
// none (or the policy is "never").
func (a *App) GetCrashReport() string {
	dir := a.crashDir()
	if dir == "" || a.settings.Behavior.CrashReports == config.CrashNever {
		return ""
	}
	return crash.Pending(dir)
}

// DismissCrashReport discards the pending crash log.
func (a *App) DismissCrashReport() {
	if dir := a.crashDir(); dir != "" {
		_ = crash.Dismiss(dir)
	}
}

// ReportCrash opens a prefilled GitHub issue containing the pending crash log
// (truncated), so the user sees exactly what is shared before submitting.
func (a *App) ReportCrash() {
	log := a.GetCrashReport()
	if log == "" {
		return
	}
	if len(log) > 3000 {
		log = log[:3000] + "\n… (truncated)"
	}
	issueURL := projectURL + "/issues/new?title=" +
		url.QueryEscape("Crash report ("+appVersion+")") +
		"&body=" + url.QueryEscape("```\n"+log+"\n```")
	a.OpenURL(issueURL)
}
