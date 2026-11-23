//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestKeyTextIgnoresTheHeldOptionKey(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "key-text")
	out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "Carbon", "testdata/key-text/main.m", "darwin_key_text.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture compile: %v\n%s", err, out)
	}
	if out, err = exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("fixture run: %v\n%s", err, out)
	}
}
