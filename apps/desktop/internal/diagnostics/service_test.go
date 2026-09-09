package diagnostics

import (
	"context"
	"strings"
	"testing"
	"time"

	"option-tab/internal/platform"
)

type saveFixture struct {
	entered chan []byte
	release chan struct{}
}

func (f *saveFixture) SaveDiagnosticReport(ctx context.Context, _ string, data []byte) (platform.DiagnosticExportResult, error) {
	f.entered <- data
	<-f.release
	return platform.DiagnosticExportResult{Status: "cancelled"}, ctx.Err()
}

func statusFixture() StatusSnapshot {
	return StatusSnapshot{Version: "0.4.8", OS: "darwin", Arch: "arm64", Runtime: "go1.26.0"}
}

func TestDiagnosticPrivacyRejectsArbitraryPayload(t *testing.T) {
	s := New(Deps{})
	if err := s.StartRecording(); err != nil {
		t.Fatal(err)
	}
	secret := "/Users/person/Private/file.txt"
	if err := s.Record(Event{Component: Component(secret), Code: Code("sourceStarted")}); err == nil {
		t.Fatal("raw payload admitted")
	}
	snapshot := statusFixture()
	snapshot.Version = secret
	if _, err := s.Preview(snapshot); err == nil {
		t.Fatal("path admitted as version")
	}
	review, err := s.Preview(statusFixture())
	if err != nil || strings.Contains(review.JSON, secret) {
		t.Fatal("secret exported")
	}
}

func TestDiagnosticLimitsAndImmutableReview(t *testing.T) {
	now := time.Unix(100, 0)
	s := New(Deps{Now: func() time.Time { return now }})
	if s.Recording() {
		t.Fatal("recording default on")
	}
	if err := s.StartRecording(); err != nil {
		t.Fatal(err)
	}
	for range 1100 {
		if err := s.Record(Event{Component: Capture, Code: SourceStarted, Count: 1}); err != nil {
			t.Fatal(err)
		}
	}
	review, err := s.Preview(statusFixture())
	if err != nil || review.Dropped != 76 || len(review.JSON) > platform.MaxDiagnosticExportBytes {
		t.Fatalf("limits %+v %v", review, err)
	}
	before := review.JSON
	s.Clear()
	if review.JSON != before {
		t.Fatal("review bytes mutated")
	}
	now = now.Add(11 * time.Minute)
	if s.Recording() {
		t.Fatal("recording exceeded deadline")
	}
}

func TestDiagnosticExportCancellationKeepsSlotUntilDrain(t *testing.T) {
	f := &saveFixture{entered: make(chan []byte, 1), release: make(chan struct{})}
	s := New(Deps{Exporter: f})
	review, err := s.Preview(statusFixture())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := s.Export(context.Background(), review.Token); done <- err }()
	data := <-f.entered
	if string(data) != review.JSON {
		t.Fatal("export differs from reviewed bytes")
	}
	s.Clear()
	if _, err := s.Preview(statusFixture()); err != ErrBusy {
		t.Fatalf("cancel released slot before drain: %v", err)
	}
	close(f.release)
	if err := <-done; err == nil {
		t.Fatal("cancelled export succeeded")
	}
	if _, err := s.Export(context.Background(), review.Token); err != ErrStale {
		t.Fatalf("cleared token reused %v", err)
	}
}

func TestDiagnosticCancelExportRetainsRecordedData(t *testing.T) {
	s := New(Deps{})
	if err := s.StartRecording(); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(Event{Component: Dock, Code: SourceStarted, Count: 1}); err != nil {
		t.Fatal(err)
	}
	before, err := s.Preview(statusFixture())
	if err != nil {
		t.Fatal(err)
	}
	s.CancelExport()
	after, err := s.Preview(statusFixture())
	if err != nil || after.JSON != before.JSON || !s.Recording() {
		t.Fatal("cancel export discarded recording")
	}
	if _, err = s.Export(context.Background(), before.Token); err != ErrStale {
		t.Fatal("cancelled token admitted")
	}
}

func TestDiagnosticRecordingExpiryAndStartIdempotence(t *testing.T) {
	now := time.Unix(100, 0)
	s := New(Deps{Now: func() time.Time { return now }})
	if err := s.StartRecording(); err != nil {
		t.Fatal(err)
	}
	if err := s.Record(Event{Component: Dock, Code: SourceStarted}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(9 * time.Minute)
	if err := s.StartRecording(); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if s.Recording() {
		t.Fatal("idempotent start extended deadline")
	}
	if err := s.Record(Event{Component: Dock, Code: SourceStopped}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Preview(statusFixture())
	if err != nil || strings.Count(r.JSON, "sourceStarted") != 1 || strings.Contains(r.JSON, "sourceStopped") {
		t.Fatal("expired recording changed")
	}
	now = now.Add(ReviewLifetime)
	if _, err = s.Export(context.Background(), r.Token); err != ErrStale {
		t.Fatal("expired review admitted")
	}
}
