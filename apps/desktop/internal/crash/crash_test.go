package crash

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotate_PendingDismissCycle(t *testing.T) {
	dir := t.TempDir()

	// No crash last run: nothing pending.
	if err := Rotate(dir); err != nil {
		t.Fatalf("Rotate(empty dir) error: %v", err)
	}
	if got := Pending(dir); got != "" {
		t.Errorf("Pending() = %q, want empty", got)
	}

	// Simulate a crash log from the previous run.
	if err := os.WriteFile(filepath.Join(dir, currentName), []byte("panic: boom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rotate(dir); err != nil {
		t.Fatalf("Rotate() error: %v", err)
	}
	if got := Pending(dir); got != "panic: boom" {
		t.Errorf("Pending() = %q, want %q", got, "panic: boom")
	}
	if _, err := os.Stat(filepath.Join(dir, currentName)); !os.IsNotExist(err) {
		t.Error("current log should have been promoted away")
	}

	if err := Dismiss(dir); err != nil {
		t.Fatalf("Dismiss() error: %v", err)
	}
	if got := Pending(dir); got != "" {
		t.Errorf("Pending() after dismiss = %q, want empty", got)
	}
	if err := Dismiss(dir); err != nil {
		t.Errorf("Dismiss() on nothing should be nil, got %v", err)
	}
}

func TestRotate_EmptyCurrentIsNotPromoted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, currentName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rotate(dir); err != nil {
		t.Fatalf("Rotate() error: %v", err)
	}
	if got := Pending(dir); got != "" {
		t.Errorf("empty crash log must not become pending, got %q", got)
	}
}

func TestRotate_ReplacesExistingPending(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, pendingName), []byte("panic: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, currentName), []byte("panic: new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rotate(dir); err != nil {
		t.Fatalf("Rotate() error: %v", err)
	}
	if got := Pending(dir); got != "panic: new" {
		t.Errorf("Pending() = %q, want %q (new crash must replace stale pending)", got, "panic: new")
	}
	if _, err := os.Stat(filepath.Join(dir, currentName)); !os.IsNotExist(err) {
		t.Error("current log should have been promoted away")
	}
}

func TestArm_TruncatesExistingCurrent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, currentName), []byte("panic: leftover\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Arm(dir); err != nil {
		t.Fatalf("Arm() error: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, currentName))
	if err != nil {
		t.Fatalf("Arm() should keep the current crash file present: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("Arm() should truncate the current crash file, size = %d, want 0", info.Size())
	}
}

func TestArm_CreatesCrashFile(t *testing.T) {
	dir := t.TempDir()
	if err := Arm(dir); err != nil {
		t.Fatalf("Arm() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, currentName)); err != nil {
		t.Errorf("Arm() should create the current crash file: %v", err)
	}
}
