//go:build darwin

package actions

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// Opt-in only: requires the disposable NSApplication fixture's exact PID and
// state file. No production/user application may be supplied to this test.
func TestDisposableNativeActionSmoke(t *testing.T) {
	raw := os.Getenv("OPTION_TAB_ACTION_FIXTURE_PID")
	if raw == "" {
		t.Skip("requires disposable org.optiontab.ActionSmokeFixture")
	}
	pid, err := strconv.Atoi(raw)
	if err != nil || pid <= 0 {
		t.Fatal("invalid explicit fixture PID")
	}
	path := os.Getenv("OPTION_TAB_ACTION_FIXTURE_STATE")
	if path == "" {
		t.Fatal("missing fixture state file")
	}
	type window struct {
		ID        domain.WindowID `json:"id"`
		Title     string          `json:"title"`
		Minimized bool            `json:"minimized"`
		Key       bool            `json:"key"`
	}
	type state struct {
		PID     int      `json:"pid"`
		Bundle  string   `json:"bundle"`
		Windows []window `json:"windows"`
	}
	read := func() state {
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var s state
		if e = json.Unmarshal(data, &s); e != nil {
			t.Fatal(e)
		}
		if s.PID != pid || s.Bundle != "org.optiontab.ActionSmokeFixture" {
			t.Fatal("refusing non-fixture identity")
		}
		return s
	}
	first := read()
	if len(first.Windows) != 2 {
		t.Fatalf("fixture must start with two windows: %+v", first)
	}
	p, err := platform.New()
	if err != nil {
		t.Fatal(err)
	}
	wins, err := p.Windows()
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	nativeCandidates := 0
	for _, w := range wins {
		if int(w.AppID) == pid {
			nativeCandidates++
			if w.BundleID != "org.optiontab.ActionSmokeFixture" {
				t.Fatalf("native identity is not disposable fixture: %+v", w)
			}
			if strings.HasPrefix(w.Title, "ActionSmokeFixture ") {
				matched++
			}
		}
	}
	if matched != 2 {
		t.Fatalf("expected two freshly identified native fixture windows, got %d", matched)
	}
	service := New(p)
	app := domain.AppID(pid)
	defer func() { _, _ = service.Perform("forceQuit", 0, app) }()
	wait := func(label string, check func(state) bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if check(read()) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s: state=%+v", label, read())
	}
	run := func(kind string, id domain.WindowID, want int) {
		t.Helper()
		r, e := service.Perform(kind, id, app)
		t.Logf("%s: %+v err=%v", kind, r, e)
		if e != nil || r.Succeeded != want || len(r.Failures) != 0 {
			t.Fatalf("%s rejected: %+v %v", kind, r, e)
		}
	}
	runBulk := func(kind string, want int) {
		t.Helper()
		r, e := service.Perform(kind, 0, app)
		t.Logf("%s: %+v err=%v", kind, r, e)
		if e != nil || r.Succeeded != want {
			t.Fatalf("known fixture windows not handled: %+v %v", r, e)
		}
		if len(r.Failures) > 1 {
			t.Fatalf("expected one aggregated incomplete-enumeration warning, got %+v", r.Failures)
		}
		if len(r.Failures) == 1 && (r.Failures[0].WindowID != 0 || !strings.Contains(r.Failures[0].Error, "enumeration incomplete")) {
			t.Fatalf("unexpected real-window failure: %+v", r.Failures)
		}
		if nativeCandidates > matched && len(r.Failures) != 1 {
			t.Fatal("unresolved native replicas silently omitted from bulk result")
		}
	}
	run("newWindow", 0, 1)
	wait("new window created", func(s state) bool { return len(s.Windows) == 3 })
	target := first.Windows[0].ID
	run("focus", target, 1)
	wait("target became key", func(s state) bool {
		for _, w := range s.Windows {
			if w.ID == target {
				return w.Key
			}
		}
		return false
	})
	runBulk("minimizeAll", 3)
	wait("all actually minimized", func(s state) bool {
		if len(s.Windows) != 3 {
			return false
		}
		for _, w := range s.Windows {
			if !w.Minimized {
				return false
			}
		}
		return true
	})
	run("minimize", target, 1)
	wait("single toggle restored target", func(s state) bool {
		for _, w := range s.Windows {
			if w.ID == target {
				return !w.Minimized
			}
		}
		return false
	})
	runBulk("closeAll", 3)
	wait("all closed", func(s state) bool { return len(s.Windows) == 0 })
	run("newWindow", 0, 1)
	wait("refusal fixture created", func(s state) bool {
		return len(s.Windows) == 1 && strings.Contains(s.Windows[0].Title, "Refuses Close")
	})
	refusal := read().Windows[0].ID
	run("close", refusal, 1)
	time.Sleep(250 * time.Millisecond)
	if s := read(); len(s.Windows) != 1 || s.Windows[0].ID != refusal {
		t.Fatal("fixture did not veto close as configured")
	}
	t.Log("AX close request accepted while fixture delegate vetoed closure; success means request acceptance, not final closure")
	run("forceQuit", 0, 1)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			t.Log("exact disposable PID exited")
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("force quit did not terminate fixture")
}
