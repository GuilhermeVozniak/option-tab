//go:build darwin

package update

import (
	"os"
	"path/filepath"
	"testing"
)

// An unsigned bundle must never pass verification: codesign --verify fails
// before the Team ID check is even reached.
func TestVerifyBundleRejectsUnsignedBundle(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Option Tab.app")
	bin := filepath.Join(app, "Contents", "MacOS", "option-tab")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyBundle(app); err == nil {
		t.Fatal("expected an unsigned bundle to be rejected")
	}
}
