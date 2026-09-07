//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMediaLyricsNativeFileIdentity(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "lyrics-file")
	output, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "UniformTypeIdentifiers", "testdata/media-events/lyrics.m", "-o", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("native fixture build: %v\n%s", err, output)
	}
	output, err = exec.Command(binary, t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatalf("native file fixture: %v\n%s", err, output)
	}
	t.Log(string(output))
}
