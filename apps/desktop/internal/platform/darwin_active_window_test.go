//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAutomationNativeActiveAndFinalGuardSeam(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "automation-active")
	out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "ApplicationServices", "testdata/automation-active/main.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile seam: %v\n%s", err, out)
	}
	out, err = exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("native seam: %v\n%s", err, out)
	}
	t.Log(string(out))
}
