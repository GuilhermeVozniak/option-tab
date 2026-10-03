package main

import (
	"context"
	"testing"
	"time"

	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func (p *launcherIntegrationPlatform) RelaunchLauncherApp(ctx context.Context, target platform.LauncherAppTarget, guard func() error) error {
	return p.ActivateLauncherApp(ctx, target, guard)
}

func TestRunningLauncherRelaunchExactScope(t *testing.T) {
	a, backend, q, panel := launcherIntegrationApp(t)
	a.wireLauncherItems(nil, nil)
	p := launcherIntegrationVisible(t, a, q)
	if p.Items[0].ReferenceRevision != 0 {
		t.Fatal("fixture is not an unpinned application")
	}
	if err := a.RelaunchLauncherItem(p.Epoch, p.DisplayUUID, p.Session, p.Revision, p.Items[0].ID); err != nil {
		t.Fatalf("explicit unpinned relaunch was refused: %v", err)
	}
	target := <-backend.entered
	if target.Process != (platform.ProcessIdentity{PID: 4242, StartSeconds: 123}) || target.BundleID != "test.launcher.fixture" || target.PanelToken != panel.token || target.DisplayUUID != integrationDisplay || backend.mutations.Load() != 1 {
		t.Fatalf("wrong native relaunch admission: %+v", target)
	}
}

func TestRunningLauncherRelaunchLateAdmission(t *testing.T) {
	for _, change := range []string{"content", "pause", "host", "owner"} {
		t.Run(change, func(t *testing.T) {
			a, backend, q, _ := launcherIntegrationApp(t)
			a.wireLauncherItems(nil, nil)
			p := launcherIntegrationVisible(t, a, q)
			backend.release = make(chan struct{})
			var retiredOwner <-chan struct{}
			done := make(chan error, 1)
			go func() { done <- a.RelaunchLauncherItem(p.Epoch, p.DisplayUUID, p.Session, p.Revision, p.Items[0].ID) }()
			select {
			case <-backend.entered:
			case err := <-done:
				t.Fatalf("unpinned relaunch never reached native preparation: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("native preparation timed out")
			}
			switch change {
			case "content":
				a.viewMu.Lock()
				a.launcher.hosts[p.Session].presentation.Revision++
				a.viewMu.Unlock()
			case "pause":
				a.settingsMu.Lock()
				a.settings.Behavior.Paused = true
				a.settingsMu.Unlock()
			case "host":
				a.viewMu.Lock()
				h := *a.launcher.hosts[p.Session]
				a.launcher.hosts[p.Session] = &h
				a.viewMu.Unlock()
			case "owner":
				retiredOwner = a.launcherItems.done
				a.wireLauncherItems(nil, nil)
				select {
				case <-retiredOwner:
					t.Fatal("retired owner completed before the native operation drained")
				default:
				}
			}
			close(backend.release)
			err := <-done
			if retiredOwner != nil {
				select {
				case <-retiredOwner:
				case <-time.After(3 * time.Second):
					t.Fatal("retired owner did not join completed native operation")
				}
			}
			if change == "content" {
				if err != nil || backend.mutations.Load() != 1 {
					t.Fatalf("content tick retired the admitted relaunch: %v", err)
				}
			} else if err == nil || backend.mutations.Load() != 0 {
				t.Fatalf("retired relaunch reached dispatch: %v", err)
			}
		})
	}
}

func TestRunningLauncherRelaunchStaleInitialScope(t *testing.T) {
	a, backend, q, _ := launcherIntegrationApp(t)
	a.wireLauncherItems(nil, nil)
	p := launcherIntegrationVisible(t, a, q)
	err := a.RelaunchLauncherItem(p.Epoch, p.DisplayUUID, p.Session, p.Revision+1, p.Items[0].ID)
	if err != launcher.ErrRetired || backend.mutations.Load() != 0 {
		t.Fatalf("unrendered revision admitted: %v", err)
	}
}
