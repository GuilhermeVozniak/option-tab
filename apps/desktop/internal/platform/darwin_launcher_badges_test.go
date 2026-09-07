//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLauncherBadgeNativeExtraction(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "badges")
	out, err := exec.Command("clang", "-fobjc-arc", "-fblocks", "-mmacosx-version-min=14.0", "-framework", "Cocoa", "-framework", "ApplicationServices", "testdata/launcher-badges/main.m", "-o", bin).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	if out, err = exec.Command(bin).CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
}
