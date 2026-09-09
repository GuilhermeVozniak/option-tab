package main

import (
	"context"
	"errors"
	"regexp"
	"runtime"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/diagnostics"
	"option-tab/internal/platform"
)

type DiagnosticsReview struct {
	Token     string `json:"token"`
	JSON      string `json:"json"`
	ExpiresAt string `json:"expiresAt"`
	Recording bool   `json:"recording"`
	Dropped   uint64 `json:"dropped"`
}

// Called during construction, before any source starts. The service owns no
// native resources until the user explicitly saves a reviewed report.
func (a *App) wireDiagnostics(exporter platform.DiagnosticExportSource) {
	if a.diagnostics != nil {
		a.diagnostics.Close()
	}
	a.diagnostics = diagnostics.New(diagnostics.Deps{Exporter: exporter})
}

func (a *App) diagnosticAdmission() error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.diagnosticAdmissionLocked()
}

func (a *App) diagnosticAdmissionLocked() error {
	if a.diagnostics == nil {
		return errors.New("diagnostics: unavailable")
	}
	select {
	case <-a.captureStop:
		return errors.New("diagnostics: unavailable")
	default:
	}
	if a.sessionInactive {
		return errors.New("diagnostics: inactive")
	}
	return nil
}

func diagnosticError(err error) error {
	if err == nil {
		return nil
	}
	code := "unavailable"
	var native *platform.DiagnosticExportError
	switch {
	case errors.Is(err, diagnostics.ErrBusy):
		code = "busy"
	case errors.Is(err, diagnostics.ErrStale):
		code = "reviewExpired"
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		code = "cancelled"
	case errors.As(err, &native):
		switch native.Code {
		case "destinationExists", "busy", "cancelled", "ioFailure":
			code = native.Code
		}
	}
	return errors.New("diagnostics: " + code)
}

func (a *App) GetDiagnosticsReview() (DiagnosticsReview, error) {
	if err := a.diagnosticAdmission(); err != nil {
		return DiagnosticsReview{}, err
	}
	status := a.diagnosticSnapshot()
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if err := a.diagnosticAdmissionLocked(); err != nil {
		return DiagnosticsReview{}, err
	}
	review, err := a.diagnostics.Preview(status)
	if err != nil {
		return DiagnosticsReview{}, diagnosticError(err)
	}
	return DiagnosticsReview{Token: review.Token, JSON: review.JSON, ExpiresAt: review.ExpiresAt.Format(time.RFC3339), Recording: review.Recording, Dropped: review.Dropped}, nil
}

func (a *App) StartDiagnosticsRecording() error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if err := a.diagnosticAdmissionLocked(); err != nil {
		return err
	}
	return diagnosticError(a.diagnostics.StartRecording())
}

func (a *App) StopDiagnosticsRecording() {
	if a.diagnostics != nil {
		a.diagnostics.StopRecording()
	}
}

func (a *App) ClearDiagnostics() {
	if a.diagnostics != nil {
		a.diagnostics.Clear()
	}
}

func (a *App) SaveDiagnosticsReport(token string) (platform.DiagnosticExportResult, error) {
	if err := a.diagnosticAdmission(); err != nil {
		return platform.DiagnosticExportResult{}, err
	}
	result, err := a.diagnostics.Export(context.Background(), token)
	return result, diagnosticError(err)
}

var diagnosticRuntimeVersion = regexp.MustCompile(`^go[0-9]{1,3}\.[0-9]{1,3}(\.[0-9]{1,3})?$`)

// Construct a fixed allowlist. Never pass settings, permission error messages,
// media samples, raw logs, or window inventories to the report service.
func (a *App) diagnosticSnapshot() diagnostics.StatusSnapshot {
	s := a.settingsSnapshot()
	v := diagnostics.StatusSnapshot{Version: appVersion, OS: runtime.GOOS, Arch: runtime.GOARCH}
	if version := runtime.Version(); diagnosticRuntimeVersion.MatchString(version) {
		v.Runtime = version
	}
	feature := func(component diagnostics.Component, enabled bool) {
		status := diagnostics.Disabled
		if enabled {
			// Configuration does not establish native source health.
			status = diagnostics.Unknown
			if s.Behavior.Paused && component != diagnostics.Updates {
				status = diagnostics.Unavailable
			}
		}
		v.Features = append(v.Features, diagnostics.FeatureStatus{Component: component, Enabled: enabled, Status: status})
	}
	feature(diagnostics.Switcher, true)
	feature(diagnostics.Dock, s.Dock.Enabled)
	feature(diagnostics.Folders, s.Dock.FolderPop.Enabled)
	feature(diagnostics.Media, s.Dock.Media.Enabled && (s.Dock.Media.MusicEnabled || s.Dock.Media.SpotifyEnabled))
	feature(diagnostics.Automation, a.automation != nil)
	feature(diagnostics.Updates, s.Behavior.UpdatePolicy != config.UpdatesOff)
	permission := func(kind diagnostics.Permission, state platform.PermState) {
		status := diagnostics.Unknown
		if state == platform.PermGranted {
			status = diagnostics.Granted
		}
		if state == platform.PermDenied {
			status = diagnostics.Denied
		}
		v.Permissions = append(v.Permissions, diagnostics.PermissionStatus{Permission: kind, Status: status})
	}
	permission(diagnostics.Accessibility, a.platform.Accessibility())
	permission(diagnostics.ScreenRecording, a.platform.ScreenRecording())
	known := a.GetMediaPermissions()
	for _, provider := range []struct {
		key        string
		permission diagnostics.Permission
	}{{"music", diagnostics.MusicAutomation}, {"spotify", diagnostics.SpotifyAutomation}} {
		status := diagnostics.Unknown
		switch known[provider.key].Status {
		case "ready":
			status = diagnostics.Granted
		case "permissionRequired":
			status = diagnostics.Required
		case "denied", "permissionDenied":
			status = diagnostics.Denied
		case "unsupported":
			status = diagnostics.Unsupported
		}
		v.Permissions = append(v.Permissions, diagnostics.PermissionStatus{Permission: provider.permission, Status: status})
	}
	return v
}

func (a *App) recordDiagnostic(component diagnostics.Component, code diagnostics.Code) {
	if a.diagnostics != nil {
		_ = a.diagnostics.Record(diagnostics.Event{Component: component, Code: code, Count: 1})
	}
}

// Names describe application presentation requests, not proof of a physical
// native window. Payloads, frame/key/pointer streams and unknown names are ignored.
func (a *App) recordDiagnosticEvent(name string) {
	switch name {
	case "dock:show":
		a.recordDiagnostic(diagnostics.Dock, diagnostics.PresentationRequested)
	case "dock:hide":
		a.recordDiagnostic(diagnostics.Dock, diagnostics.PresentationRetired)
	case "dock:error":
		a.recordDiagnostic(diagnostics.Dock, diagnostics.ErrorReported)
	case "switcher:show":
		a.recordDiagnostic(diagnostics.Switcher, diagnostics.PresentationRequested)
	case "switcher:hide":
		a.recordDiagnostic(diagnostics.Switcher, diagnostics.PresentationRetired)
	case "switcher:error":
		a.recordDiagnostic(diagnostics.Switcher, diagnostics.ErrorReported)
	case "automation-preview:hide":
		a.recordDiagnostic(diagnostics.Automation, diagnostics.PresentationRetired)
	}
}
