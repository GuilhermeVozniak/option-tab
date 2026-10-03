//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWidgetPackageNativeChooserAndRead(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "widget-package-fixture")
	cmd := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "UniformTypeIdentifiers", "testdata/widget-package/main.m", "-o", binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(binary, t.TempDir()).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
