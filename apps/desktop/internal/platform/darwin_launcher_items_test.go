//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLauncherItemNativeIdentity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "items")
	out, err := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "UniformTypeIdentifiers", "testdata/launcher-items/main.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}
	t.Log(string(out))
}
