package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppBundleFromExecutable(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want string
	}{
		{"standard install", "/Applications/Option Tab.app/Contents/MacOS/option-tab", "/Applications/Option Tab.app"},
		{"user applications", "/home/u/Applications/Option Tab.app/Contents/MacOS/option-tab", "/home/u/Applications/Option Tab.app"},
		{"directly in bundle root", "/tmp/X.app/x", "/tmp/X.app"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AppBundleFromExecutable(tt.exec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppBundleFromExecutableNotInBundle(t *testing.T) {
	for _, exec := range []string{
		"/usr/local/bin/option-tab",
		"option-tab",
		"/tmp/go-build1234/exe/main",
	} {
		if got, err := AppBundleFromExecutable(exec); err == nil {
			t.Fatalf("%s: expected error, got %q", exec, got)
		}
	}
}

func TestSwapApp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Option Tab.app")
	staged := filepath.Join(dir, ".option-tab-new")
	writeFile(t, filepath.Join(target, "Contents", "MacOS", "option-tab"), "old-binary")
	writeFile(t, filepath.Join(staged, "Contents", "MacOS", "option-tab"), "new-binary")

	if err := SwapApp(staged, target); err != nil {
		t.Fatalf("SwapApp: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(target, "Contents", "MacOS", "option-tab"))
	if err != nil || string(data) != "new-binary" {
		t.Fatalf("target holds %q, err=%v; want new-binary", data, err)
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Fatalf("backup should be removed, stat err=%v", err)
	}
}

func TestSwapAppRollsBackWhenNewAppMissing(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "Option Tab.app")
	writeFile(t, filepath.Join(target, "Contents", "MacOS", "option-tab"), "old-binary")

	err := SwapApp(filepath.Join(dir, "does-not-exist"), target)
	if err == nil {
		t.Fatal("expected an error when the staged bundle is missing")
	}
	data, readErr := os.ReadFile(filepath.Join(target, "Contents", "MacOS", "option-tab"))
	if readErr != nil || string(data) != "old-binary" {
		t.Fatalf("rollback failed: target holds %q, err=%v", data, readErr)
	}
}

func TestRelaunchScript(t *testing.T) {
	s := relaunchScript(4242, "/Applications/Option Tab.app")
	if !strings.Contains(s, "kill -0 4242") {
		t.Fatalf("script should wait on the pid, got %q", s)
	}
	if !strings.Contains(s, `open -n "/Applications/Option Tab.app"`) {
		t.Fatalf("script should open the quoted bundle path, got %q", s)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
