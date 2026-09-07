//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLauncherGestureNativeExtraction(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gestures")
	out, err := exec.Command("clang", "-fobjc-arc", "-fblocks", "-mmacosx-version-min=14.0", "-framework", "Cocoa", "testdata/launcher-gestures/main.m", "-o", bin).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if out, err = exec.Command(bin).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
}
