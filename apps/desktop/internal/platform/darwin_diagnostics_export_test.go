//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDiagnosticNativeAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(t.TempDir(), "diagnostics-fixture")
	cmd := exec.Command("clang", "-fobjc-arc", "-fblocks", "-framework", "Cocoa", "-framework", "UniformTypeIdentifiers", "testdata/diagnostics-export/main.m", "-o", binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	if out, err := exec.Command(binary, dir).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}

func TestDiagnosticNativeContextAdmission(t *testing.T) {
	cmd := exec.Command("go", "run", "./testdata/diagnostics-export/context", t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("context fixture: %v\n%s", err, out)
	} else {
		t.Log(string(out))
	}
}
