package main

import (
	"context"
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/diagnostics"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type diagnosticExportFixture struct {
	save func(context.Context, string, []byte) (platform.DiagnosticExportResult, error)
}

func (f diagnosticExportFixture) SaveDiagnosticReport(ctx context.Context, name string, data []byte) (platform.DiagnosticExportResult, error) {
	return f.save(ctx, name, data)
}

func TestDiagnosticsAppUsesOnlyReviewedBytesAndNeverEventPayloads(t *testing.T) {
	const secret = "private-title-path-account-SECRET"
	p := fake.New()
	s := config.Default()
	s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: secret, Hide: config.HideAlways}}
	a := newApp(p, s, "")
	defer a.stopCapture()
	var exported string
	a.wireDiagnostics(diagnosticExportFixture{save: func(_ context.Context, name string, data []byte) (platform.DiagnosticExportResult, error) {
		if name != "option-tab-diagnostics.json" {
			t.Fatal("unexpected filename")
		}
		exported = string(data)
		return platform.DiagnosticExportResult{Status: "saved"}, nil
	}})
	initial, err := a.GetDiagnosticsReview()
	if err != nil || initial.Recording {
		t.Fatalf("default recording: %+v %v", initial, err)
	}
	if err := a.StartDiagnosticsRecording(); err != nil {
		t.Fatal(err)
	}
	a.emit("dock:show", map[string]string{"title": secret})
	a.emit("switcher:error", secret)
	a.emit("switcher:thumbnails", secret)
	a.emit("switcher:key", secret)
	a.emit(secret, secret)
	review, err := a.GetDiagnosticsReview()
	if err != nil || strings.Contains(review.JSON, secret) || !strings.Contains(review.JSON, "presentationRequested") || !strings.Contains(review.JSON, "errorReported") {
		t.Fatalf("unsafe or missing diagnostics: %+v %v", review, err)
	}
	a.emit("dock:hide", nil)
	result, err := a.SaveDiagnosticsReport(review.Token)
	if err != nil || result.Status != "saved" || exported != review.JSON {
		t.Fatalf("saved unreviewed bytes: %+v %v", result, err)
	}
	if len(p.FocusCalls) != 0 || len(p.RequestCalls) != 0 {
		t.Fatal("diagnostics changed foreground or requested permission")
	}
}

func TestDiagnosticsAppClearAndInactivityRetireExports(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	defer a.stopCapture()
	entered, joined := make(chan struct{}), make(chan struct{})
	a.wireDiagnostics(diagnosticExportFixture{save: func(ctx context.Context, _ string, _ []byte) (platform.DiagnosticExportResult, error) {
		close(entered)
		<-ctx.Done()
		close(joined)
		return platform.DiagnosticExportResult{Status: "cancelled"}, ctx.Err()
	}})
	review, err := a.GetDiagnosticsReview()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := a.SaveDiagnosticsReport(review.Token); done <- err }()
	mediaReceive(t, entered)
	a.setSessionInactive(true)
	mediaReceive(t, joined)
	if err := mediaReceive(t, done); err == nil || err.Error() != "diagnostics: cancelled" {
		t.Fatalf("inactive export was not cancelled: %v", err)
	}
	if err := a.StartDiagnosticsRecording(); err == nil {
		t.Fatal("inactive recording admitted")
	}
	a.setSessionInactive(false)
	if _, err := a.SaveDiagnosticsReport(review.Token); err == nil {
		t.Fatal("inactive preview token revived")
	}
	if err := a.StartDiagnosticsRecording(); err != nil {
		t.Fatal(err)
	}
	a.recordDiagnostic(diagnostics.Dock, diagnostics.PresentationRequested)
	a.ClearDiagnostics()
	cleared, err := a.GetDiagnosticsReview()
	if err != nil || cleared.Recording || strings.Contains(cleared.JSON, "presentationRequested") {
		t.Fatalf("clear retained history: %+v %v", cleared, err)
	}
	a.stopCapture()
	if _, err := a.GetDiagnosticsReview(); err == nil {
		t.Fatal("shutdown still admitted report")
	}
}
