package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform/fake"
)

func TestCrashReportLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	a := newApp(fake.New(), config.Default(), path)

	if got := a.GetCrashReport(); got != "" {
		t.Fatalf("GetCrashReport() with no crash = %q, want empty", got)
	}

	// Simulate a crash log left by the previous run, then a fresh startup.
	if err := os.WriteFile(filepath.Join(dir, "crash-current.log"), []byte("panic: kaboom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.setupCrashCapture()
	if got := a.GetCrashReport(); !strings.HasPrefix(got, "panic: kaboom") {
		t.Fatalf("GetCrashReport() = %q, want the promoted crash log", got)
	}

	a.DismissCrashReport()
	if got := a.GetCrashReport(); got != "" {
		t.Fatalf("GetCrashReport() after dismiss = %q, want empty", got)
	}
}

func TestCrashReportPolicyNeverClearsAndHides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	s := config.Default()
	s.Behavior.CrashReports = config.CrashNever
	a := newApp(fake.New(), s, path)

	if err := os.WriteFile(filepath.Join(dir, "crash-pending.log"), []byte("panic: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.setupCrashCapture() // policy never: clears leftovers, arms nothing
	if got := a.GetCrashReport(); got != "" {
		t.Fatalf("GetCrashReport() under never policy = %q, want empty", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "crash-pending.log")); !os.IsNotExist(err) {
		t.Error("pending crash log should be deleted under the never policy")
	}
	if _, err := os.Stat(filepath.Join(dir, "crash-current.log")); !os.IsNotExist(err) {
		t.Error("capture must not be armed under the never policy")
	}
}
