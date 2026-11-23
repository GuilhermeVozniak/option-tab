//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOverlayWebViewActsOnAClickMadeWhileAnotherAppIsActive(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "overlay-mouse")
	out, err := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "WebKit", "testdata/overlay-mouse/main.m", "darwin_overlay_mouse.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture compile: %v\n%s", err, out)
	}
	if out, err = exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("fixture run: %v\n%s", err, out)
	}
}
