package platform

import (
	"testing"
)

type keyboardFixture struct {
	prepared, resume chan struct{}
	installed        bool
}

func (f *keyboardFixture) Set(_ uint64, _ LauncherKeyboardPolicy, current func() bool) error {
	if f.prepared != nil {
		close(f.prepared)
		<-f.resume
	}
	if !current() {
		return ErrDockPanelClosed
	}
	f.installed = true
	return nil
}
func (*keyboardFixture) Valid(uint64, LauncherKeyboardPolicy) bool { return true }
func keyboardPolicy() LauncherKeyboardPolicy {
	return LauncherKeyboardPolicy{Epoch: 1, Session: 1, Revision: 1, Admission: 1, DisplayUUID: "fixture", Enabled: true}
}

func TestLauncherKeyboardRetiredInstallation(t *testing.T) {
	f := &keyboardFixture{prepared: make(chan struct{}), resume: make(chan struct{})}
	o := newLauncherKeyboardOwner(1, f)
	result := make(chan error, 1)
	go func() { result <- o.set(keyboardPolicy(), func() bool { return true }) }()
	<-f.prepared
	o.retire(true)
	close(f.resume)
	if err := <-result; err == nil || f.installed {
		t.Fatal("retired keyboard permission installed")
	}
}

func TestLauncherKeyboardFinalGuard(t *testing.T) {
	f := &keyboardFixture{}
	o := newLauncherKeyboardOwner(1, f)
	if err := o.set(keyboardPolicy(), func() bool { return false }); err == nil || f.installed {
		t.Fatal("refused guard installed")
	}
	if err := o.set(keyboardPolicy(), func() bool { return true }); err != nil {
		t.Fatal(err)
	}
	if !o.valid(keyboardPolicy()) {
		t.Fatal("current permission refused")
	}
	o.retire(false)
	if o.valid(keyboardPolicy()) {
		t.Fatal("retired permission valid")
	}
}

func TestLauncherKeyboardGuardRunsUnlockedAndRechecksRetirement(t *testing.T) {
	f := &keyboardFixture{}
	o := newLauncherKeyboardOwner(1, f)
	if err := o.set(keyboardPolicy(), func() bool { o.retire(false); return true }); err == nil || f.installed {
		t.Fatal("guard-time retirement admitted")
	}
}
