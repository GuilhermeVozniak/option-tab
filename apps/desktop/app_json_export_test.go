package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

type jsonExportFake struct {
	save func(context.Context, string, []byte) (platform.DiagnosticExportResult, error)
}

func (f jsonExportFake) SaveJSONExport(ctx context.Context, name string, data []byte) (platform.DiagnosticExportResult, error) {
	return f.save(ctx, name, data)
}

func TestJSONExportUsesCanonicalSanitizedProfileAndSettings(t *testing.T) {
	a := transferApp(t)
	// Keep private fields in canonical settings to prove export sanitizes its
	// own snapshot, not caller-provided bytes.
	var source struct {
		Profile config.LauncherProfile `json:"profile"`
	}
	if err := json.Unmarshal([]byte(transferDocument(t)), &source); err != nil {
		t.Fatal(err)
	}
	a.settingsMu.Lock()
	a.settings.ReplacementDock.Profiles[0] = source.Profile
	a.settingsMu.Unlock()
	before := a.GetSettings()
	var gotName string
	var gotData []byte
	a.jsonExporter = jsonExportFake{save: func(_ context.Context, name string, data []byte) (platform.DiagnosticExportResult, error) {
		gotName, gotData = name, bytes.Clone(data)
		return platform.DiagnosticExportResult{Status: "saved"}, nil
	}}
	result, err := a.SaveLauncherProfileExport("default")
	if err != nil || result.Status != "saved" {
		t.Fatalf("export: %v %v", result, err)
	}
	want, err := config.ExportLauncherProfile(source.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if gotName != "option-tab-launcher-profile.json" || !bytes.Equal(gotData, want) || strings.Contains(string(gotData), strings.Repeat("a", 32)) || strings.Contains(string(gotData), strings.Repeat("b", 64)) {
		t.Fatal("profile export did not save sanitized canonical bytes")
	}
	if a.GetSettings() != before {
		t.Fatal("export changed settings")
	}
	result, err = a.SaveSettingsExport()
	if err != nil || result.Status != "saved" {
		t.Fatalf("settings export: %v %v", result, err)
	}
	var expected bytes.Buffer
	if err := config.Save(&expected, a.settingsSnapshot()); err != nil {
		t.Fatal(err)
	}
	if gotName != "option-tab-settings.json" || !bytes.Equal(gotData, expected.Bytes()) {
		t.Fatal("settings export did not save canonical settings")
	}
	if a.GetSettings() != before {
		t.Fatal("settings export changed settings")
	}
}

func TestJSONExportCancellationErrorsAndAdmission(t *testing.T) {
	a := transferApp(t)
	calls := 0
	a.jsonExporter = jsonExportFake{save: func(_ context.Context, _ string, _ []byte) (platform.DiagnosticExportResult, error) {
		calls++
		return platform.DiagnosticExportResult{Status: "cancelled"}, nil
	}}
	got, err := a.SaveSettingsExport()
	if err != nil || got.Status != "cancelled" {
		t.Fatalf("cancel must be neutral: %v %v", got, err)
	}
	if _, err := a.SaveLauncherProfileExport("missing"); err == nil || calls != 1 {
		t.Fatal("missing profile opened save dialog")
	}
	a.ClosePreferences()
	if _, err := a.SaveSettingsExport(); err == nil || calls != 1 {
		t.Fatal("closed preferences opened save dialog")
	}
	a.viewMu.Lock()
	a.prefsOpen = true
	a.viewMu.Unlock()
	for _, code := range []string{"destinationExists", "ioFailure", "unsupported"} {
		a.jsonExporter = jsonExportFake{save: func(_ context.Context, _ string, _ []byte) (platform.DiagnosticExportResult, error) {
			return platform.DiagnosticExportResult{}, &platform.DiagnosticExportError{Code: code}
		}}
		_, err := a.SaveSettingsExport()
		want := code
		if code == "unsupported" {
			want = "unavailable"
		}
		if err == nil || err.Error() != "json export: "+want {
			t.Fatalf("coarse error: %v", err)
		}
	}
	a.jsonExporter = jsonExportFake{save: func(_ context.Context, _ string, _ []byte) (platform.DiagnosticExportResult, error) {
		return platform.DiagnosticExportResult{}, errors.New("private path/content")
	}}
	if _, err := a.SaveSettingsExport(); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("native error leaked")
	}
}

func TestJSONExportRetainsBusyUntilCancelledNativeOwnerJoins(t *testing.T) {
	for _, retire := range []string{"prefs", "session", "shutdown"} {
		t.Run(retire, func(t *testing.T) {
			a := transferApp(t)
			entered := make(chan context.Context, 1)
			release := make(chan struct{})
			a.jsonExporter = jsonExportFake{save: func(ctx context.Context, _ string, _ []byte) (platform.DiagnosticExportResult, error) {
				entered <- ctx
				<-release
				return platform.DiagnosticExportResult{Status: "cancelled"}, ctx.Err()
			}}
			done := make(chan error, 1)
			go func() { _, err := a.SaveSettingsExport(); done <- err }()
			ctx := <-entered
			if _, err := a.SaveSettingsExport(); err == nil || err.Error() != "json export: busy" {
				t.Fatal("parallel save admitted")
			}
			switch retire {
			case "prefs":
				a.ClosePreferences()
			case "session":
				a.transitionSession(1, true)
			case "shutdown":
				a.stopCapture()
			}
			if ctx.Err() == nil {
				t.Fatal("retired preferences/session retained export admission")
			}
			a.viewMu.Lock()
			busy := a.jsonExportCancel != nil
			a.viewMu.Unlock()
			if !busy {
				t.Fatal("busy released before native cleanup")
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatalf("cancelled export became error: %v", err)
			}
			a.viewMu.Lock()
			busy = a.jsonExportCancel != nil
			a.viewMu.Unlock()
			if busy {
				t.Fatal("completed export retained busy")
			}
		})
	}
}
