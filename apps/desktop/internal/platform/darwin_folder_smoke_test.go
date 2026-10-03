//go:build darwin

package platform

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFolderNativeRunloopFixture(t *testing.T) {
	if os.Getenv("OPTION_TAB_FOLDER_NATIVE_FIXTURE") != "1" {
		t.Skip("opt-in injected AppKit fixture; no actual chooser or workspace open")
	}
	root, err := os.MkdirTemp("", "option-tab-folder-smoke-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	}()
	if err = os.WriteFile(filepath.Join(root, "fixture.txt"), []byte("Option Tab disposable native fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(root, "subfolder"), 0o700); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "folder-native-fixture")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-fblocks", "testdata/folder-pop/native_fixture.m", "darwin_folder.m", "-framework", "Cocoa", "-o", binary)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture compile: %v\n%s", err, out)
	}
	out, err := exec.CommandContext(ctx, binary, root).CombinedOutput()
	t.Logf("native fixture: %s", out)
	if err != nil {
		t.Fatalf("fixture run: %v", err)
	}
}

func TestFolderNativeReadOnlyDockClassification(t *testing.T) {
	if os.Getenv("OPTION_TAB_FOLDER_DOCK_READONLY") != "1" {
		t.Skip("opt-in read-only actual Dock folder AX inventory")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "dock-folder-readonly")
	out, err := exec.CommandContext(ctx, "clang", "-fobjc-arc", "testdata/folder-pop/dock_readonly.m", "-framework", "Cocoa", "-framework", "ApplicationServices", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.CommandContext(ctx, binary).CombinedOutput()
	t.Logf("read-only Dock: %s", out)
	if err != nil {
		t.Fatalf("actual folder classification unavailable: %v", err)
	}
}
