//go:build darwin

package platform

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAudioOutputNativeGuardPipeline(t *testing.T) { runAudioFixture(t, "main.m") }
func TestAudioOutputInjectedHAL(t *testing.T)         { runAudioFixture(t, "hal.m") }
func runAudioFixture(t *testing.T, source string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "audio")
	out, err := exec.Command("clang", "-fobjc-arc", "-framework", "Foundation", "-framework", "CoreAudio", filepath.Join("testdata/audio-output", source), "-o", bin).CombinedOutput()
	if err != nil {
		t.Fatalf("compile: %v\n%s", err, out)
	}
	out, err = exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("fixture: %v\n%s", err, out)
	}
	t.Log(string(out))
}
