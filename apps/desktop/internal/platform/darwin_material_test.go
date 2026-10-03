//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMaterialNativeLifecycle(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "material")
	out, err := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "QuartzCore", "-framework", "ApplicationServices", "testdata/material/main.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	if out, err = exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
