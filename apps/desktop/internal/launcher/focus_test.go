package launcher

import (
	"context"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func focusedFixture(t *testing.T) *Controller {
	c := prepared(t)
	p := c.settings.Profiles[0]
	p.ID = "editor"
	p.Appearance.Theme = "dark"
	c.settings.Profiles = append(c.settings.Profiles, p)
	c.settings.Rules = []config.LauncherProfileRule{{ID: "editor-rule", Enabled: true, BundleID: "org.example.Editor", ProfileID: "editor", BindingID: "main"}}
	return c
}

func focus(e platform.LauncherEnvironment, seq uint64, bundle string) platform.LauncherEnvironment {
	e.Sequence = seq
	e.FocusKnown = true
	e.FocusedProcess = platform.ProcessIdentity{PID: 72, StartSeconds: 88}
	e.FocusedBundleID = bundle
	return e
}

func TestFocusedRuleSwitchRetiresBeforeOwnerAndABA(t *testing.T) {
	c := focusedFixture(t)
	p := c.Snapshot().Presentations[0]
	c.acceptEnvironment(c.epoch, focus(env(), 2, "org.example.Editor"))
	if err := c.Activate(context.Background(), p.Scope, "one"); err != ErrRetired {
		t.Fatalf("old profile click admitted %v", err)
	}
	// Both transitions may precede reconciliation; the earlier A route stays dead.
	c.acceptEnvironment(c.epoch, focus(env(), 3, "org.example.Unmatched"))
	c.reconcileLocked()
	a := c.Snapshot().Presentations[0]
	if a.ProfileID != "default" || a.Session == p.Session {
		t.Fatal("coalesced ABA revived old session")
	}
	c.acceptEnvironment(c.epoch, focus(env(), 4, "org.example.Editor"))
	c.reconcileLocked()
	b := c.Snapshot().Presentations[0]
	if b.ProfileID != "editor" || b.Session == a.Session || b.Appearance.Theme != "dark" {
		t.Fatal("focus rule did not replace profile host")
	}
	c.acceptEnvironment(c.epoch, focus(env(), 5, "org.example.Unmatched"))
	c.reconcileLocked()
	a2 := c.Snapshot().Presentations[0]
	if a2.ProfileID != "default" || a2.Session == a.Session {
		t.Fatal("return revived A")
	}
}

func TestFocusedRulesOrderedScopedUnknownAndNoChurn(t *testing.T) {
	c := focusedFixture(t)
	second := c.env.Displays[0]
	second.Main = false
	second.UUID = "22222222-2222-2222-2222-222222222222"
	second.Frame.X = 1000
	second.UsableFrame.X = 1000
	c.env.Displays = append(c.env.Displays, second)
	c.settings.Bindings = append(c.settings.Bindings, config.LauncherBinding{ID: "second", Target: "display", DisplayUUID: second.UUID, ProfileID: "default"})
	c.reconcileLocked()
	original := c.Snapshot()
	e := focus(c.env, 2, "org.example.Editor")
	c.acceptEnvironment(c.epoch, e)
	c.reconcileLocked()
	s := c.Snapshot()
	if s.Presentations[0].ProfileID != "editor" || s.Presentations[1].ProfileID != "default" || s.Presentations[1].Session != original.Presentations[1].Session {
		t.Fatal("scoped rule affected another binding")
	}
	c.settings.Rules = append([]config.LauncherProfileRule{{ID: "first", Enabled: true, BundleID: "org.example.Editor", ProfileID: "default"}}, c.settings.Rules...)
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].ProfileID != "default" {
		t.Fatal("first rule did not win")
	}
	c.settings.Rules[0].Enabled = false
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].ProfileID != "editor" {
		t.Fatal("disabled rule matched")
	}
	e = focus(c.env, 3, "org.example.Editor")
	e.FocusedProcess.StartSeconds = 0
	c.acceptEnvironment(c.epoch, e)
	c.reconcileLocked()
	fallback := c.Snapshot().Presentations[0]
	if fallback.ProfileID != "default" {
		t.Fatal("invalid focus matched rule")
	}
	e = focus(c.env, 4, "org.example.editor")
	c.acceptEnvironment(c.epoch, e)
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Session != fallback.Session {
		t.Fatal("case mismatch or unmatched focus churned base")
	}
}

func TestFocusedRuleInvalidatesPreparedAction(t *testing.T) {
	c := focusedFixture(t)
	p := c.Snapshot().Presentations[0]
	entered := make(chan struct{})
	resume := make(chan struct{})
	done := make(chan error, 1)
	c.deps.Activate = func(_ context.Context, _ Scope, _ platform.LauncherAppTarget, guard func() error) error {
		close(entered)
		<-resume
		return guard()
	}
	go func() { done <- c.Activate(context.Background(), p.Scope, "one") }()
	recv(t, entered)
	c.acceptEnvironment(c.epoch, focus(env(), 2, "org.example.Editor"))
	close(resume)
	if err := recv(t, done); err != ErrRetired {
		t.Fatalf("prepared action survived focus switch: %v", err)
	}
}

func TestFocusedRuleCannotCreateUnboundSurface(t *testing.T) {
	c := focusedFixture(t)
	c.settings.Bindings = nil
	c.settings.Rules[0].BindingID = ""
	c.env = focus(env(), 2, "org.example.Editor")
	c.reconcileLocked()
	if len(c.Snapshot().Presentations) != 0 {
		t.Fatal("rule created unbound display")
	}
}
