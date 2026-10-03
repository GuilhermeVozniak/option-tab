package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type shakeAppPlatform struct {
	*fake.Fake
	roots        []platform.ActionWindowRole
	calls        []domain.WindowID
	beforeRoles  func()
	beforeNative func()
	fail         domain.WindowID
}

func (p *shakeAppPlatform) ObserveWindowDrags(ctx context.Context, _ func(platform.WindowDragEvent)) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *shakeAppPlatform) WindowDragGestureCurrent(platform.WindowDragValidation) bool { return true }

func (p *shakeAppPlatform) ActionWindowRoles() ([]platform.ActionWindowRole, error) {
	if p.beforeRoles != nil {
		p.beforeRoles()
	}
	return p.roots, nil
}

func (p *shakeAppPlatform) PerformOtherWindowAction(_ string, id domain.WindowID, _ domain.AppID) error {
	if id == p.fail {
		return errors.New("close refused")
	}
	p.calls = append(p.calls, id)
	return nil
}

func (p *shakeAppPlatform) PerformOtherWindowActionGuarded(kind string, id domain.WindowID, app domain.AppID, guard func() error) error {
	if p.beforeNative != nil {
		p.beforeNative()
	}
	if err := guard(); err != nil {
		return err
	}
	return p.PerformOtherWindowAction(kind, id, app)
}

func shakeAppFixture(t *testing.T) (*App, *shakeAppPlatform, dock.ShakeIntent) {
	t.Helper()
	p := &shakeAppPlatform{Fake: fake.New()}
	for _, id := range []domain.WindowID{101, 102, 103} {
		p.roots = append(p.roots, platform.ActionWindowRole{WindowRole: platform.WindowRole{WindowID: id, AppID: 10, Role: "AXWindow", Subrole: "AXStandardWindow"}, RootConfirmed: true, RelationshipsKnown: true})
	}
	s := config.Default()
	s.Dock.Enabled = true
	s.Dock.Input.AeroShakeAction = "minimizeOthers"
	a := newApp(p, s, "")
	t.Cleanup(a.stopCapture)
	return a, p, dock.ShakeIntent{Generation: 1, GestureID: 2, Kind: "minimizeOthers", WindowID: 101, AppID: 10, Timestamp: time.Now()}
}

func TestDockShakePreservesKeptWindowAndReportsPartialFailure(t *testing.T) {
	a, p, intent := shakeAppFixture(t)
	p.fail = 103
	err := a.executeDockShake(intent, func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "1 window") || !strings.Contains(err.Error(), "1 failed") || len(p.calls) != 1 || p.calls[0] != 102 {
		t.Fatalf("wrong partial result: err=%v calls=%v", err, p.calls)
	}
}

func TestDockShakeRechecksLifecycleAfterRoleLookup(t *testing.T) {
	for _, change := range []string{"inactive", "disable", "shutdown", "action"} {
		t.Run(change, func(t *testing.T) {
			a, p, intent := shakeAppFixture(t)
			p.beforeRoles = func() {
				switch change {
				case "shutdown":
					a.stopCapture()
				case "inactive":
					a.viewMu.Lock()
					a.sessionInactive = true
					a.syncDockShakeLocked()
					a.viewMu.Unlock()
				default:
					a.settingsMu.Lock()
					if change == "disable" {
						a.settings.Dock.Enabled = false
					} else {
						a.settings.Dock.Input.AeroShakeAction = "closeOthers"
					}
					a.settingsMu.Unlock()
				}
			}
			err := a.executeDockShake(intent, func() error { return nil })
			if !errors.Is(err, dock.ErrShakeRetired) || len(p.calls) > 0 {
				t.Fatalf("retired shake dispatched: %v %v", err, p.calls)
			}
		})
	}
}

func TestDockShakeGuardRunsAfterNativePreparation(t *testing.T) {
	a, p, intent := shakeAppFixture(t)
	current := true
	p.beforeNative = func() { current = false }
	err := a.executeDockShake(intent, func() error {
		if !current {
			return dock.ErrShakeRetired
		}
		return nil
	})
	if !errors.Is(err, dock.ErrShakeRetired) || len(p.calls) > 0 {
		t.Fatalf("native preparation bypassed retirement: %v %v", err, p.calls)
	}
}

func TestDockShakeCanRunWhilePreferencesAreOpen(t *testing.T) {
	a, p, intent := shakeAppFixture(t)
	a.viewMu.Lock()
	a.prefsOpen = true
	a.syncDockShakeLocked()
	a.viewMu.Unlock()
	if err := a.executeDockShake(intent, func() error { return nil }); err != nil || len(p.calls) != 2 {
		t.Fatalf("passive shake unexpectedly suspended: %v %v", err, p.calls)
	}
}
