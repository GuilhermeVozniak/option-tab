//go:build darwin

package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"option-tab/internal/domain"
)

type (
	dragFixtureWindow struct {
		ID        domain.WindowID `json:"id"`
		Visible   bool            `json:"visible"`
		Minimized bool            `json:"minimized"`
	}
	dragFixtureState struct {
		PID                   domain.AppID `json:"pid"`
		Kept, Eligible, Sheet dragFixtureWindow
		EligibleClosed        bool
		SheetAttached         bool
	}
)

func TestWindowDragNativeFixture(t *testing.T) {
	if os.Getenv("OPTION_TAB_WINDOW_DRAG_FIXTURE_SMOKE") != "1" {
		t.Skip("opt-in disposable fixture")
	}
	for _, kind := range []string{"setMinimized", "close"} {
		t.Run(kind, func(t *testing.T) { runWindowDragNativeFixture(t, kind) })
	}
}

func runWindowDragNativeFixture(t *testing.T, kind string) {
	if os.Getenv("OPTION_TAB_WINDOW_DRAG_FIXTURE_SMOKE") != "1" {
		t.Skip("opt-in disposable fixture")
	}
	p := &darwinPlatform{}
	before := p.ActiveApp()
	dir := t.TempDir()
	bundle := filepath.Join(dir, "D14 Window Roles.app", "Contents")
	binDir := filepath.Join(bundle, "MacOS")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>com.optiontab.windowdragfixture</string><key>CFBundleName</key><string>D14 Window Roles</string><key>CFBundleExecutable</key><string>fixture</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	binary := filepath.Join(binDir, "fixture")
	if output, err := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-framework", "Cocoa", "testdata/window-drag/fixture.m", "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v\n%s", err, output)
	}
	statePath := filepath.Join(dir, "state.json")
	cmd := exec.Command(binary, statePath)
	log, err := os.Create(filepath.Join(dir, "fixture.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := log.Close(); err != nil {
			t.Error(err)
		}
	}()
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		if err := os.WriteFile(statePath+".command", []byte("stop"), 0o600); err != nil {
			t.Error(err)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("fixture exit: %v", err)
			}
		case <-time.After(2 * time.Second):
			if err := cmd.Process.Kill(); err != nil {
				t.Error(err)
			}
			<-done
			t.Error("fixture required forced cleanup")
		}
		t.Logf("fixture PID=%d joined and cleaned", cmd.Process.Pid)
		if after := p.ActiveApp(); after != before {
			t.Errorf("foreground changed: before=%d after=%d", before, after)
		}
	})
	read := func() dragFixtureState {
		var state dragFixtureState
		data, err := os.ReadFile(statePath)
		if err == nil {
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatalf("fixture decode: %v JSON=%s", err, data)
			}
		}
		return state
	}
	var state dragFixtureState
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		state = read()
		if int(state.PID) == cmd.Process.Pid && state.Kept.ID > 0 && state.Eligible.ID > 0 && state.Sheet.ID > 0 && state.SheetAttached {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if int(state.PID) != cmd.Process.Pid || state.Kept.ID == 0 || state.Eligible.ID == 0 || state.Sheet.ID == 0 || !state.SheetAttached {
		data, _ := os.ReadFile(filepath.Join(dir, "fixture.log"))
		t.Fatalf("fixture not ready %+v\n%s", state, data)
	}
	if current := p.ActiveApp(); current != before {
		t.Fatalf("fixture activated: before=%d current=%d", before, current)
	}
	t.Logf("fixture PID=%d kept=%d eligible=%d sheet=%d foreground=%d", state.PID, state.Kept.ID, state.Eligible.ID, state.Sheet.ID, before)
	allowed := map[domain.WindowID]bool{state.Kept.ID: true, state.Eligible.ID: true, state.Sheet.ID: true}
	if os.Getenv("OPTION_TAB_WINDOW_DRAG_ROLE_DIAGNOSTICS") == "1" {
		probe := filepath.Join(dir, "role-probe")
		if output, err := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-framework", "Cocoa", "-framework", "ApplicationServices", "testdata/window-drag/roles.m", "-o", probe).CombinedOutput(); err != nil {
			t.Fatalf("role probe compile: %v %s", err, output)
		}
		output, err := exec.CommandContext(ctx, probe, fmt.Sprint(state.PID)).CombinedOutput()
		t.Logf("exact-fixture AX diagnostics err=%v\n%s", err, output)
	}

	var found map[domain.WindowID]ActionWindowRole
	readyDeadline := time.Now().Add(4 * time.Second)
	for attempt := 1; ; attempt++ {
		roles, err := p.ActionWindowRoles()
		if err != nil {
			t.Fatal(err)
		}
		found = map[domain.WindowID]ActionWindowRole{}
		for _, role := range roles {
			if role.AppID == state.PID && allowed[role.WindowID] {
				found[role.WindowID] = role
			}
		}
		normal, parent, child := found[state.Eligible.ID], found[state.Kept.ID], found[state.Sheet.ID]
		if normal.RootConfirmed && normal.RelationshipsKnown && parent.RootConfirmed && parent.RelationshipsKnown && parent.HasAttachedSheet && child.Role == "AXSheet" && child.ParentWindowID == state.Kept.ID {
			t.Logf("fixture AX ready after %d read-only snapshots", attempt)
			break
		}
		if time.Now().After(readyDeadline) {
			t.Fatalf("fixture AX readiness did not settle: %+v", found)
		}
		t.Logf("fixture AX settling; no action admitted: %+v", found)
		time.Sleep(50 * time.Millisecond)
	}
	for _, role := range found {
		t.Logf("role %+v", role)
	}
	eligibleRole, ok := found[state.Eligible.ID]
	if !ok || !eligibleRole.RootConfirmed || !eligibleRole.RelationshipsKnown || eligibleRole.Role != "AXWindow" || eligibleRole.Subrole != "AXStandardWindow" || eligibleRole.Modal || eligibleRole.HasAttachedSheet || eligibleRole.HasModalChild || eligibleRole.SelfApplication || eligibleRole.ParentWindowID != 0 {
		t.Fatalf("eligible classification refused: %+v", eligibleRole)
	}
	sheetRole := found[state.Sheet.ID]
	if sheetRole.Role != "AXSheet" || sheetRole.RootConfirmed || sheetRole.ParentWindowID != state.Kept.ID {
		t.Fatalf("exact attached sheet identity/parent unresolved: %+v", sheetRole)
	}
	keptRole := found[state.Kept.ID]
	if !keptRole.RootConfirmed || !keptRole.RelationshipsKnown || (!keptRole.HasAttachedSheet && !keptRole.HasModalChild) {
		t.Fatalf("attached sheet not protected: %+v", keptRole)
	}
	action := func(kind string, id domain.WindowID) error {
		if id != state.Eligible.ID {
			return fmt.Errorf("fixture action boundary refused %d", id)
		}
		return p.PerformOtherWindowAction(kind, id, state.PID)
	}

	refused := errors.New("fixture gesture retired after native preparation")
	calls := 0
	guardErr := p.PerformOtherWindowActionGuarded(kind, state.Eligible.ID, state.PID, func() error { calls++; return refused })
	if guardErr != refused || calls != 1 {
		t.Fatalf("final native guard not reached/refusal lost: calls=%d err=%v", calls, guardErr)
	}
	untouched := read()
	if untouched.EligibleClosed || untouched.Eligible.Minimized {
		t.Fatal("guard-refused native action mutated fixture")
	}
	if err := action(kind, state.Eligible.ID); err != nil {
		t.Fatal(err)
	}
	until = time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		state = read()
		if (kind == "setMinimized" && state.Eligible.Minimized) || (kind == "close" && state.EligibleClosed) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if (kind == "setMinimized" && !state.Eligible.Minimized) || (kind == "close" && !state.EligibleClosed) || !state.Kept.Visible || !state.Sheet.Visible || !state.SheetAttached {
		t.Fatalf("fixture preservation/action failed: kind=%s state=%+v", kind, state)
	}

	if after := p.ActiveApp(); after != before {
		t.Fatalf("native action changed foreground: %d -> %d", before, after)
	}
	t.Logf("PASS %s exact eligible; final guard refusal caused zero mutation; kept parent/sheet survived; foreground=%d unchanged", kind, before)
}
