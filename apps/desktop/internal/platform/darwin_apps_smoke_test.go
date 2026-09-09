//go:build darwin

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"option-tab/internal/domain"
)

func TestApplicationNativeSmokeWindowlessInventoryAndExactActivation(t *testing.T) {
	state := os.Getenv("OPTION_TAB_APP_SMOKE_STATE")
	if state == "" {
		t.Skip("run internal/platform/testdata/run_app_inventory_smoke.sh")
	}
	body, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(body))
	if len(fields) != 2 || fields[1] != "closed" {
		t.Fatalf("fixture state = %q", body)
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGTERM) })

	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	apps, err := p.(ApplicationSource).Apps()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, app := range apps {
		if app.ID == domain.AppID(pid) && app.BundleID == "com.optiontab.InventorySmoke" {
			found = true
		}
	}
	if !found {
		t.Fatalf("closed-window fixture pid %d absent from inventory: %+v", pid, apps)
	}
	if err := p.(ApplicationActivator).ActivateApp(domain.AppID(pid)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if frontmostApplicationPID() == pid {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(fmt.Errorf("activation accepted but foreground pid is %d, want %d", frontmostApplicationPID(), pid))
}
