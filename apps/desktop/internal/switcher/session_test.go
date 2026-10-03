package switcher

import (
	"testing"

	"option-tab/internal/platform"
)

func TestSessionSuspendCancelsAndPreservesUserPause(t *testing.T) {
	c, _, view := newController(t, threeWins(), nil)
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	c.HandleHotkey(activate)
	if !c.IsOpen() {
		t.Fatal("fixture did not activate")
	}
	c.Suspend(true)
	if c.IsOpen() || view.hides != 1 {
		t.Fatal("suspension did not cancel current overlay")
	}
	c.HandleHotkey(activate)
	if c.IsOpen() {
		t.Fatal("activation admitted during suspension")
	}
	if c.Paused() {
		t.Fatal("transient suspension changed persisted pause")
	}
	c.SetPaused(true)
	c.Suspend(false)
	c.HandleHotkey(activate)
	if c.IsOpen() || !c.Paused() {
		t.Fatal("resume cleared user pause")
	}
	c.SetPaused(false)
	c.HandleHotkey(activate)
	if !c.IsOpen() {
		t.Fatal("activation did not resume")
	}
}

func TestSessionPresentationTokenInvalidatedOnCancelAndSuspend(t *testing.T) {
	c, _, _ := newController(t, threeWins(), nil)
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	c.HandleHotkey(activate)
	first := c.State()
	if first.Session == 0 || c.PresentationSession() != first.Session {
		t.Fatal("missing first presentation token")
	}
	c.Cancel()
	if c.PresentationSession() != 0 {
		t.Fatal("Cancel retained presentation token")
	}
	c.HandleHotkey(activate)
	second := c.State()
	if second.Session <= first.Session || c.PresentationSession() != second.Session {
		t.Fatal("new presentation reused token")
	}
	c.Suspend(true)
	if c.PresentationSession() != 0 {
		t.Fatal("Suspend retained presentation token")
	}
	c.Suspend(false)
	if c.PresentationSession() != 0 {
		t.Fatal("Resume revived old presentation token")
	}
}

func TestSessionStopCannotBeUndoneByLateResume(t *testing.T) {
	c, native, view := newController(t, threeWins(), nil)
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	c.HandleHotkey(activate)
	c.Stop()
	c.Suspend(false)
	c.SetPaused(false)
	c.HandleHotkey(activate)
	if err := c.Confirm(); err != nil {
		t.Fatal(err)
	}
	if c.IsOpen() || c.PresentationSession() != 0 || len(native.FocusCalls) != 0 || view.hides != 1 {
		t.Fatalf("terminal controller revived: open=%v token=%d focus=%v hides=%d", c.IsOpen(), c.PresentationSession(), native.FocusCalls, view.hides)
	}
	c.Stop()
	if view.hides != 1 {
		t.Fatal("repeated stop reissued hide")
	}
}

func TestSessionStopRetiresBeforeReentrantLateResume(t *testing.T) {
	c, native, _ := newController(t, threeWins(), nil)
	activate := platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1}
	view := &terminalReentrantView{onHide: func() {
		// Models a queued resume completing while terminal Hide is delivered.
		// Reentrancy also proves the controller mutex is not held by Stop.
		c.Suspend(false)
		c.HandleHotkey(activate)
		if err := c.Confirm(); err != nil {
			t.Error(err)
		}
	}}
	c.deps.View = view
	c.HandleHotkey(activate)
	before := c.State().Session
	c.Stop()
	if view.retired != before || c.IsOpen() || c.PresentationSession() != 0 || len(native.FocusCalls) != 0 {
		t.Fatal("late resume inside retirement defeated terminal state")
	}
}

type terminalReentrantView struct {
	retired uint64
	onHide  func()
}

func (v *terminalReentrantView) Show(State)                 {}
func (v *terminalReentrantView) Update(State)               {}
func (v *terminalReentrantView) Hide()                      { panic("terminal hide must retain presentation token") }
func (v *terminalReentrantView) HideSession(session uint64) { v.retired = session; v.onHide() }
