//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMediaPanelNativeGeometryAndHeader(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "media-panel")
	out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "testdata/media-events/panel.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(binary).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}
	t.Log(string(out))
}
