//go:build darwin

package platform

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMediaNativeDescriptors(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "media-events")
	command := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "Carbon", "testdata/media-events/main.m", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func TestMediaActualDeliveryOptIn(t *testing.T) {
	if os.Getenv("OPTION_TAB_MEDIA_DELIVERY_SMOKE") != "1" {
		t.Skip("explicit windowless delivery smoke opt-in")
	}
	binary := filepath.Join(t.TempDir(), "media-delivery")
	if output, err := exec.Command("clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "Carbon", "testdata/media-events/delivery.m", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("build receiver: %v\n%s", err, output)
	}
	output, err := exec.Command(binary).CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 3 {
		t.Skipf("no-prompt delivery unavailable: %s", output)
	}
	if err != nil {
		t.Fatalf("receiver: %v\n%s", err, output)
	}
	t.Log(string(output))
}
