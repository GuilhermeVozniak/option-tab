package main

import (
	"bytes"
	"context"
	"errors"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

// SaveLauncherProfileExport keeps sanitization and file writing in the backend;
// WKWebView does not provide a browser download destination in pinned Wails.
func (a *App) SaveLauncherProfileExport(profileID string) (platform.DiagnosticExportResult, error) {
	document, err := a.GetLauncherProfileExport(profileID)
	if err != nil {
		return platform.DiagnosticExportResult{}, err
	}
	return a.saveJSONExport("option-tab-launcher-profile.json", []byte(document))
}

func (a *App) SaveSettingsExport() (platform.DiagnosticExportResult, error) {
	var document bytes.Buffer
	if err := config.Save(&document, a.settingsSnapshot()); err != nil {
		return platform.DiagnosticExportResult{}, errors.New("json export: ioFailure")
	}
	return a.saveJSONExport("option-tab-settings.json", document.Bytes())
}

func (a *App) syncJSONExportAdmissionLocked() {
	if !a.launcherChoicesAllowedLocked() && a.jsonExportCancel != nil {
		a.jsonExportCancel()
	}
}

func (a *App) saveJSONExport(name string, data []byte) (platform.DiagnosticExportResult, error) {
	a.viewMu.Lock()
	if a.jsonExporter == nil || !a.launcherChoicesAllowedLocked() {
		a.viewMu.Unlock()
		return platform.DiagnosticExportResult{}, errors.New("json export: unavailable")
	}
	if a.jsonExportCancel != nil {
		a.viewMu.Unlock()
		return platform.DiagnosticExportResult{}, errors.New("json export: busy")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.jsonExportCancel = cancel
	exporter := a.jsonExporter
	a.viewMu.Unlock()
	defer func() {
		cancel()
		a.viewMu.Lock()
		a.jsonExportCancel = nil
		a.viewMu.Unlock()
	}()
	result, err := exporter.SaveJSONExport(ctx, name, data)
	if errors.Is(err, context.Canceled) || (err == nil && result.Status == "cancelled") {
		return platform.DiagnosticExportResult{Status: "cancelled"}, nil
	}
	if err == nil && result.Status == "saved" {
		return result, nil
	}
	code := "ioFailure"
	var native *platform.DiagnosticExportError
	if errors.As(err, &native) {
		switch native.Code {
		case "destinationExists", "busy":
			code = native.Code
		case "unsupported", "unavailable":
			code = "unavailable"
		}
	}
	return platform.DiagnosticExportResult{}, errors.New("json export: " + code)
}
