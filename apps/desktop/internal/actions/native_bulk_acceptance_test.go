//go:build darwin

package actions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type bulkWindow struct {
	ID        domain.WindowID `json:"id"`
	Visible   bool            `json:"visible"`
	Minimized bool            `json:"minimized"`
	Closed    bool            `json:"closed"`
}
type bulkDocument struct {
	WindowID      domain.WindowID `json:"windowID"`
	Edited        bool            `json:"edited"`
	CloseChecks   int             `json:"closeChecks"`
	WriteAttempts int             `json:"writeAttempts"`
	SheetID       domain.WindowID `json:"sheetID"`
	SheetVisible  bool            `json:"sheetVisible"`
	IsSheet       bool            `json:"isSheet"`
	ButtonCount   int             `json:"buttonCount"`
}
type bulkState struct {
	Sequence          uint64                   `json:"sequence"`
	PID               domain.AppID             `json:"pid"`
	Nonce             string                   `json:"nonce"`
	Role              string                   `json:"role"`
	Bundle            string                   `json:"bundle"`
	Executable        string                   `json:"executable"`
	Process           platform.ProcessIdentity `json:"process"`
	Foreground        domain.AppID             `json:"foreground"`
	Activated         bool                     `json:"activated"`
	ForegroundChanged bool                     `json:"foregroundChanged"`
	Windows           []bulkWindow             `json:"windows"`
	Document          bulkDocument             `json:"document"`
}
type bulkOwner struct {
	Process                         platform.ProcessIdentity
	Nonce, Role, Bundle, Executable string
	Foreground                      domain.AppID
	WindowIDs                       []domain.WindowID
}

func admitBulkFixture(owner bulkOwner, state bulkState, current platform.ProcessIdentity) error {
	if owner.Process.PID <= 0 || owner.Process.StartSeconds == 0 || owner.Nonce == "" || owner.Bundle == "" || owner.Executable == "" || owner.Foreground <= 0 || len(owner.WindowIDs) == 0 {
		return errors.New("incomplete owned process admission")
	}
	if current != owner.Process || state.Process != owner.Process || state.PID != owner.Process.PID || state.Nonce != owner.Nonce || state.Role != owner.Role || state.Bundle != owner.Bundle || state.Executable != owner.Executable {
		return errors.New("fixture process identity changed")
	}
	if state.Foreground != owner.Foreground || state.Activated || state.ForegroundChanged {
		return errors.New("fixture caused or observed foreground activation")
	}
	if state.Document.WriteAttempts != 0 {
		return errors.New("document write attempted")
	}
	if len(state.Windows) != len(owner.WindowIDs) {
		return errors.New("fixture window set changed")
	}
	seen := map[domain.WindowID]bool{}
	for _, w := range state.Windows {
		if w.ID == 0 || seen[w.ID] || !slices.Contains(owner.WindowIDs, w.ID) {
			return errors.New("foreign or duplicate fixture window")
		}
		seen[w.ID] = true
	}
	return nil
}

func TestNativeBulkFixtureAdmission(t *testing.T) {
	owner := bulkOwner{Process: platform.ProcessIdentity{PID: 43, StartSeconds: 12, StartMicros: 34}, Nonce: "private-per-run-nonce", Role: "target", Bundle: "org.optiontab.NativeBulkFixture", Executable: "/private/owned/Fixture.app/Contents/MacOS/fixture", Foreground: 99, WindowIDs: []domain.WindowID{1, 2}}
	valid := bulkState{PID: 43, Nonce: owner.Nonce, Role: owner.Role, Bundle: owner.Bundle, Executable: owner.Executable, Process: owner.Process, Foreground: 99, Windows: []bulkWindow{{ID: 1, Visible: true}, {ID: 2, Visible: true}}}
	if err := admitBulkFixture(owner, valid, owner.Process); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*bulkState, *platform.ProcessIdentity){
		"nonce":                func(s *bulkState, _ *platform.ProcessIdentity) { s.Nonce = "foreign" },
		"pid":                  func(s *bulkState, _ *platform.ProcessIdentity) { s.PID = 44 },
		"incarnation":          func(_ *bulkState, p *platform.ProcessIdentity) { p.StartMicros++ },
		"reported incarnation": func(s *bulkState, _ *platform.ProcessIdentity) { s.Process.StartSeconds++ },
		"bundle":               func(s *bulkState, _ *platform.ProcessIdentity) { s.Bundle = "user.app" },
		"executable":           func(s *bulkState, _ *platform.ProcessIdentity) { s.Executable = "/Applications/User.app/executable" },
		"role":                 func(s *bulkState, _ *platform.ProcessIdentity) { s.Role = "bystander" },
		"foreground":           func(s *bulkState, _ *platform.ProcessIdentity) { s.Foreground = 43 },
		"transient activation": func(s *bulkState, _ *platform.ProcessIdentity) { s.Activated = true },
		"transient foreground": func(s *bulkState, _ *platform.ProcessIdentity) { s.ForegroundChanged = true },
		"foreign window":       func(s *bulkState, _ *platform.ProcessIdentity) { s.Windows[1].ID = 3 },
		"duplicate window":     func(s *bulkState, _ *platform.ProcessIdentity) { s.Windows[1].ID = 1 },
		"missing window":       func(s *bulkState, _ *platform.ProcessIdentity) { s.Windows = s.Windows[:1] },
		"write attempted":      func(s *bulkState, _ *platform.ProcessIdentity) { s.Document.WriteAttempts = 1 },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			s := valid
			s.Windows = append([]bulkWindow(nil), valid.Windows...)
			current := owner.Process
			change(&s, &current)
			if err := admitBulkFixture(owner, s, current); err == nil {
				t.Fatal("unsafe fixture state admitted")
			}
		})
	}
}

// bulkBackend keeps production enumeration and service behavior; only native
// mutation is fenced to exact owned roots and the two requested action kinds.
type bulkBackend struct {
	platform.Platform
	identities platform.AutomationIdentitySource
	target     bulkOwner
	guard      func() error
}

func (b *bulkBackend) ActionWindows(app domain.AppID) ([]domain.Window, error) {
	if app != b.target.Process.PID {
		return nil, errors.New("non-owned bulk target")
	}
	if err := b.guard(); err != nil {
		return nil, err
	}
	windows, err := b.Platform.(ActionWindowSource).ActionWindows(app)
	for _, w := range windows {
		if w.AppID != app || w.BundleID != b.target.Bundle || !slices.Contains(b.target.WindowIDs, w.ID) {
			return nil, errors.New("bulk snapshot contains a non-owned root")
		}
	}
	return windows, err
}

func (b *bulkBackend) PerformTargetAction(kind string, id domain.WindowID, app domain.AppID) error {
	if (kind != "close" && kind != "setMinimized") || app != b.target.Process.PID || !slices.Contains(b.target.WindowIDs, id) {
		return errors.New("refusing non-owned native action")
	}
	if err := b.guard(); err != nil {
		return err
	}
	identity, err := b.identities.WindowIdentity(id)
	if err != nil {
		return err
	}
	matching := identity.Process == b.target.Process && identity.ID == id
	current := false
	if matching {
		current = b.identities.WindowIdentityCurrent(identity)
	}
	if !matching || !current {
		return fmt.Errorf("native window admission refused: requested=%d expectedProcess=%+v observed=%+v currentChecked=%t current=%t", id, b.target.Process, identity, matching, current)
	}
	return b.Platform.(TargetPerformer).PerformTargetAction(kind, id, app)
}

type bulkFixture struct {
	owner     bulkOwner
	statePath string
}

func TestDisposableNativeBulkAcceptance(t *testing.T) {
	if os.Getenv("OPTION_TAB_NATIVE_BULK_ACCEPTANCE") != "1" {
		t.Skip("opt-in owned nonactivating bulk fixture")
	}
	evidence := os.Getenv("OPTION_TAB_NATIVE_BULK_EVIDENCE")
	if evidence == "" || !filepath.IsAbs(evidence) {
		t.Fatal("absolute private evidence directory required")
	}
	dir, err := os.MkdirTemp(evidence, "run-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("evidence=%s", dir)
	record := func(name string, value any) {
		t.Helper()
		data, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600); e != nil {
			t.Fatal(e)
		}
	}
	p, err := platform.New()
	if err != nil {
		t.Fatal(err)
	}
	before := p.ActiveApp()
	permission := p.Accessibility() // Read only: never request or open System Settings.
	record("preflight", map[string]any{"accessibility": permission, "foreground": before})
	if permission != platform.PermGranted || before <= 0 {
		t.Fatal("native admission denied: existing Accessibility and foreground required; no fixture launched")
	}
	identities, ok := p.(platform.AutomationIdentitySource)
	if !ok {
		t.Fatal("native identities unavailable")
	}
	if _, ok = p.(TargetPerformer); !ok {
		t.Fatal("native action boundary unavailable")
	}
	if _, ok = p.(ActionWindowSource); !ok {
		t.Fatal("native root enumeration unavailable")
	}
	bundleID := "org.optiontab.NativeBulkFixture"
	contents := filepath.Join(dir, "Bulk Fixture.app", "Contents")
	binDir := filepath.Join(contents, "MacOS")
	if err = os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>CFBundleIdentifier</key><string>org.optiontab.NativeBulkFixture</string><key>CFBundleName</key><string>Bulk Fixture</string><key>CFBundleExecutable</key><string>fixture</string><key>CFBundlePackageType</key><string>APPL</string><key>LSUIElement</key><true/></dict></plist>`
	if err = os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(binDir, "fixture")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	output, err := exec.CommandContext(ctx, "clang", "-fobjc-arc", "-Wall", "-Wextra", "-Wno-unused-parameter", "-framework", "Cocoa", "testdata/native-bulk/fixture.m", "-o", binary).CombinedOutput()
	cancel()
	if e := os.WriteFile(filepath.Join(dir, "compile.log"), output, 0o600); e != nil {
		t.Fatal(e)
	}
	if err != nil {
		t.Fatalf("fixture compile: %v\n%s", err, output)
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	read := func(f bulkFixture) (bulkState, error) {
		var s bulkState
		data, e := os.ReadFile(f.statePath)
		if e != nil {
			return s, e
		}
		if e = json.Unmarshal(data, &s); e != nil {
			return s, e
		}
		current, e := identities.ProcessIdentity(f.owner.Process.PID)
		if e != nil {
			return s, e
		}
		if e = admitBulkFixture(f.owner, s, current); e != nil {
			return s, e
		}
		stat, e := os.Stat(f.statePath)
		if e != nil {
			return s, e
		}
		if time.Since(stat.ModTime()) > time.Second {
			return s, errors.New("fixture heartbeat stale")
		}
		if p.ActiveApp() != before {
			return s, errors.New("foreground changed")
		}
		return s, nil
	}
	launch := func(scenario, role string) (bulkFixture, func()) {
		t.Helper()
		random := make([]byte, 32)
		if _, e := rand.Read(random); e != nil {
			t.Fatal(e)
		}
		nonce := hex.EncodeToString(random)
		path := filepath.Join(dir, scenario+"-"+role+"-state.json")
		log, e := os.Create(filepath.Join(dir, scenario+"-"+role+".log"))
		if e != nil {
			t.Fatal(e)
		}
		cmd := exec.Command(binary, path, nonce, role, scenario, fmt.Sprint(before))
		cmd.Stdout = log
		cmd.Stderr = log
		if e = cmd.Start(); e != nil {
			_ = log.Close()
			t.Fatal(e)
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		cleaned := false
		stop := func() {
			if cleaned {
				return
			}
			cleaned = true
			// This handle is the child we created, never a PID read from JSON.
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
			_ = log.Close()
			t.Logf("joined owned fixture %s/%s PID=%d", scenario, role, cmd.Process.Pid)
		}
		t.Cleanup(stop)
		process, e := identities.ProcessIdentity(domain.AppID(cmd.Process.Pid))
		if e != nil {
			t.Fatal(e)
		}
		f := bulkFixture{owner: bulkOwner{Process: process, Nonce: nonce, Role: role, Bundle: bundleID, Executable: binary, Foreground: before}, statePath: path}
		deadline := time.Now().Add(4 * time.Second)
		for {
			select {
			case e := <-done:
				done <- e
				t.Fatalf("fixture exited before ready: %v", e)
			default:
			}
			data, e := os.ReadFile(path)
			if e == nil {
				var s bulkState
				if e = json.Unmarshal(data, &s); e != nil {
					t.Fatal(e)
				}
				expected := 2
				if scenario == "minimize" && role == "target" {
					expected = 3
				}
				if len(s.Windows) == expected {
					f.owner.WindowIDs = nil
					for _, w := range s.Windows {
						f.owner.WindowIDs = append(f.owner.WindowIDs, w.ID)
					}
					if _, e = read(f); e != nil {
						t.Fatal(e)
					}
					ready := true
					for i, w := range s.Windows {
						expectMinimized := scenario == "minimize" && role == "target" && i == 0
						if w.Closed || w.Minimized != expectMinimized || (!expectMinimized && !w.Visible) {
							ready = false
						}
					}
					if ready {
						return f, stop
					}
				}
			}
			if p.ActiveApp() != before {
				t.Fatal("fixture setup changed foreground")
			}
			if time.Now().After(deadline) {
				t.Fatalf("fixture readiness deadline: %v", e)
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
	// A08 completes and records its independent result before the A07 attempt.
	for _, scenario := range []string{"minimize", "close"} {
		target, stopTarget := launch(scenario, "target")
		bystander, stopBystander := launch(scenario, "bystander")
		initialTarget, e := read(target)
		if e != nil {
			t.Fatal(e)
		}
		initialBystander, e := read(bystander)
		if e != nil {
			t.Fatal(e)
		}
		guard := func() error {
			if _, e := read(target); e != nil {
				return e
			}
			s, e := read(bystander)
			if e != nil {
				return e
			}
			if !reflect.DeepEqual(s.Windows, initialBystander.Windows) {
				return errors.New("untouched bystander mutated")
			}
			return nil
		}
		backend := &bulkBackend{Platform: p, identities: identities, target: target.owner, guard: guard}
		if os.Getenv("OPTION_TAB_NATIVE_BULK_DIAGNOSTICS") == "1" {
			// Three bounded read-only samples; this branch never constructs or
			// invokes Service and does not advance to the document scenario.
			for sample := 1; sample <= 3; sample++ {
				if e = guard(); e != nil {
					t.Fatal(e)
				}
				observed := make([]map[string]any, 0, len(target.owner.WindowIDs))
				for _, id := range target.owner.WindowIDs {
					identity, err := identities.WindowIdentity(id)
					matching := err == nil && identity.Process == target.owner.Process && identity.ID == id
					current := false
					var currentElapsed time.Duration
					if matching {
						started := time.Now()
						current = identities.WindowIdentityCurrent(identity)
						currentElapsed = time.Since(started)
					}
					observed = append(observed, map[string]any{"requested": id, "expectedProcess": target.owner.Process, "observed": identity, "identityError": fmt.Sprint(err), "currentChecked": matching, "current": current, "currentElapsedMS": float64(currentElapsed.Microseconds()) / 1000})
				}
				roots, err := backend.ActionWindows(target.owner.Process.PID)
				record(fmt.Sprintf("diagnostic-%d", sample), map[string]any{"identities": observed, "actionWindows": roots, "actionWindowsError": fmt.Sprint(err)})
				if e = guard(); e != nil {
					t.Fatal(e)
				}
				if sample < 3 {
					time.Sleep(100 * time.Millisecond)
				}
			}
			stopTarget()
			stopBystander()
			if p.ActiveApp() != before {
				t.Fatal("foreground changed after read-only fixture cleanup")
			}
			t.Log("READ-ONLY diagnostics complete; no bulk mutation or native acceptance claimed")
			return
		}
		// Read-only AX readiness: no mutation retries, and never accept unknown roots.
		ready := time.Now().Add(4 * time.Second)
		for {
			roots, e := backend.ActionWindows(target.owner.Process.PID)
			if len(roots) == len(target.owner.WindowIDs) {
				break
			}
			if e != nil && !strings.Contains(e.Error(), "enumeration incomplete") {
				t.Fatal(e)
			}
			if time.Now().After(ready) {
				t.Fatalf("native roots did not settle: %+v %v", roots, e)
			}
			time.Sleep(50 * time.Millisecond)
		}
		record(scenario+"-before", map[string]any{"target": initialTarget, "bystander": initialBystander})
		service := New(backend)
		run := func(kind, label string, settled func(bulkState) bool) {
			t.Helper()
			prior, e := read(target)
			if e != nil {
				t.Fatal(e)
			}
			priorBystander, e := read(bystander)
			if e != nil {
				t.Fatal(e)
			}
			result, e := service.PerformGuarded(kind, 0, target.owner.Process.PID, guard)
			t.Logf("%s %s: accepted=%d failures=%+v err=%v", scenario, label, result.Succeeded, result.Failures, e)
			record(scenario+"-"+label+"-action", map[string]any{"result": result, "error": fmt.Sprint(e)})
			if e != nil || result.Succeeded != len(target.owner.WindowIDs) {
				t.Fatalf("bulk action rejected: %+v %v", result, e)
			}
			for _, failure := range result.Failures {
				if failure.WindowID != 0 || !strings.Contains(failure.Error, "enumeration incomplete") {
					t.Fatalf("native root failure: %+v", failure)
				}
			}
			deadline := time.Now().Add(4 * time.Second)
			var stableSince time.Time
			var stableTargetSequence, stableBystanderSequence uint64
			for {
				if e = guard(); e != nil {
					t.Fatal(e)
				}
				s, e := read(target)
				if e != nil {
					t.Fatal(e)
				}
				b, err := read(bystander)
				if err != nil {
					t.Fatal(err)
				}
				if s.Sequence > prior.Sequence && b.Sequence > priorBystander.Sequence && settled(s) {
					if stableSince.IsZero() {
						stableSince = time.Now()
						stableTargetSequence, stableBystanderSequence = s.Sequence, b.Sequence
					}
					if time.Since(stableSince) >= 500*time.Millisecond && s.Sequence > stableTargetSequence && b.Sequence > stableBystanderSequence {
						record(scenario+"-"+label, map[string]any{"target": s, "bystander": b, "observedStableMS": time.Since(stableSince).Milliseconds()})
						return
					}
				} else {
					stableSince = time.Time{}
					stableTargetSequence, stableBystanderSequence = 0, 0
				}
				if time.Now().After(deadline) {
					record(scenario+"-"+label+"-unsettled", s)
					t.Fatalf("native outcome did not settle: %+v", s)
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
		if scenario == "minimize" {
			minimized := 0
			for _, w := range initialTarget.Windows {
				if w.Minimized {
					minimized++
				}
			}
			if minimized != 1 {
				t.Fatalf("expected one already-minimized root, got %d", minimized)
			}
			allMinimized := func(s bulkState) bool {
				for _, w := range s.Windows {
					if !w.Minimized || w.Closed {
						return false
					}
				}
				return true
			}
			run("minimizeAll", "first", allMinimized)
			run("minimizeAll", "repeat", allMinimized)
			t.Log("PASS A08: all three exact target roots minimized; repeat preserved state; bystander unchanged")
		} else {
			if !initialTarget.Document.Edited || initialTarget.Document.SheetID != 0 {
				t.Fatal("edited document not initially ready")
			}
			preserved := func(s bulkState) bool {
				d := s.Document
				if !d.Edited || d.CloseChecks < 1 || d.WriteAttempts != 0 || d.SheetID == 0 || !d.SheetVisible || !d.IsSheet || d.ButtonCount < 3 {
					return false
				}
				for _, w := range s.Windows {
					if w.ID == d.WindowID {
						if w.Closed || !w.Visible {
							return false
						}
					} else if !w.Closed {
						return false
					}
				}
				return true
			}
			run("closeAll", "after", preserved)
			t.Log("PASS A07: clean sibling closed; edited NSDocument and AppKit save sheet preserved; no document writes; bystander unchanged")
		}
		stopTarget()
		stopBystander()
		if p.ActiveApp() != before {
			t.Fatal("foreground changed after fixture cleanup")
		}
	}
}
