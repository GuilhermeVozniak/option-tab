package main

import (
	"errors"
	"testing"

	"option-tab/internal/launcher"
)

func TestLauncherAutoHideHoldRequiresExactLiveHostAndRecoveryAdmission(t *testing.T) {
	a, _, q, _ := launcherIntegrationApp(t)
	p := launcherIntegrationVisible(t, a, q)
	hold := func(revision, sequence uint64, phase string) error {
		return a.SetLauncherAutoHideHold(p.Epoch, p.DisplayUUID, p.Session, revision, sequence, phase)
	}
	if err := hold(p.Revision, 1, "begin"); err != nil {
		t.Fatal(err)
	}
	if err := hold(p.Revision+1, 2, "end"); !errors.Is(err, launcher.ErrRetired) {
		t.Fatal("invented revision admitted", err)
	}
	if err := hold(p.Revision, 3, "end"); err != nil {
		t.Fatal(err)
	}
	if err := hold(p.Revision, 2, "begin"); !errors.Is(err, launcher.ErrRetired) {
		t.Fatal("late begin revived release", err)
	}
	a.viewMu.Lock()
	a.launcher.recovery = true
	a.viewMu.Unlock()
	if err := hold(p.Revision, 4, "begin"); !errors.Is(err, launcher.ErrRetired) {
		t.Fatal("hold overrode native Dock recovery", err)
	}
}
