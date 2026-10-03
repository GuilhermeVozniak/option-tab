//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSystemWidgetNativeNormalization(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "status")
	out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Foundation", "-framework", "IOKit", "-framework", "SystemConfiguration", "testdata/system-widget-status/main.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}
	t.Log(string(out))
}
