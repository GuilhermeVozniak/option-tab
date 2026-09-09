package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type launcherStatusPlatform struct {
	*appMonitorLockPlatform
	readDisplays func(context.Context) ([]platform.DockLockDisplay, error)
}

func (p *launcherStatusPlatform) DockMonitorLockDisplays(ctx context.Context) ([]platform.DockLockDisplay, error) {
	return p.readDisplays(ctx)
}

func TestLauncherDisabledStatusEventRetainsFreshConnectedDisplaysAfterSave(t *testing.T) {
	displays := []platform.DockLockDisplay{{UUID: "main", Name: "Main", Main: true}, {UUID: "second", Name: "Second"}}
	p := &launcherStatusPlatform{appMonitorLockPlatform: &appMonitorLockPlatform{Fake: fake.New()}, readDisplays: func(context.Context) ([]platform.DockLockDisplay, error) { return displays, nil }}
	a := newApp(p, config.Default(), "")
	t.Cleanup(a.stopCapture)
	a.viewMu.Lock()
	a.prefsOpen = true
	a.viewMu.Unlock()
	if initial := a.GetLauncherStatus(); len(initial.Displays) != 2 {
		t.Fatal("initial static display inventory missing")
	}
	events := make(chan LauncherStatus, 4)
	a.eventSink = func(name string, data any) {
		if name == "launcher:status" {
			events <- data.(LauncherStatus)
		}
	}
	s := a.settingsSnapshot()
	s.ReplacementDock.Profiles[0].Name = "Changed while disabled"
	document, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveSettingsAtRevision(string(document), a.GetSettingsState().Revision); err != nil {
		t.Fatal(err)
	}
	// This is the same publication path used by the controller after Configure.
	a.publishLauncher(a.launcher.core.Snapshot())
	select {
	case status := <-events:
		if status.Enabled || len(status.Displays) != 2 || status.Displays[1].UUID != "second" {
			t.Fatalf("saved status erased connected displays: %+v", status)
		}
	case <-time.After(time.Second):
		t.Fatal("status event missing")
	}
	// The next event reads current topology rather than preserving a disconnected
	// display forever in a settings-only cache.
	displays = displays[:1]
	a.publishLauncher(a.launcher.core.Snapshot())
	select {
	case status := <-events:
		if len(status.Displays) != 1 || status.Displays[0].UUID != "main" {
			t.Fatalf("status did not refresh connected displays: %+v", status)
		}
	case <-time.After(time.Second):
		t.Fatal("fresh status event missing")
	}
}

func TestLauncherDisplayRefreshRejectsRetiredStatusOwners(t *testing.T) {
	for _, retire := range []string{"revision", "session", "preferences", "closed", "shutdown"} {
		t.Run(retire, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			p := &launcherStatusPlatform{appMonitorLockPlatform: &appMonitorLockPlatform{Fake: fake.New()}, readDisplays: func(ctx context.Context) ([]platform.DockLockDisplay, error) {
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > 2*time.Second {
					t.Error("display query must be bounded")
				}
				close(entered)
				<-release
				return []platform.DockLockDisplay{{UUID: "retired", Name: "Retired"}}, nil
			}}
			a := newApp(p, config.Default(), "")
			t.Cleanup(a.stopCapture)
			events := make(chan LauncherStatus, 4)
			a.eventSink = func(name string, data any) {
				if name == "launcher:status" {
					events <- data.(LauncherStatus)
				}
			}
			a.viewMu.Lock()
			a.prefsOpen = true
			owner := a.launcherStatusLocked()
			session, prefs := a.sessionGeneration, a.prefsRefreshGeneration
			a.viewMu.Unlock()
			done := make(chan struct{})
			go func() { defer close(done); a.refreshLauncherStatusTopology(a.launcher, owner, session, prefs) }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("topology query not started")
			}
			// A blocked reader must not retain viewMu: session/preferences/native
			// owner retirement remains synchronous and independent of the query.
			a.viewMu.Lock()
			switch retire {
			case "revision":
				a.launcher.statusRevision++
			case "session":
				a.sessionGeneration++
			case "preferences":
				a.prefsRefreshGeneration++
			case "closed":
				a.prefsOpen = false
			}
			a.viewMu.Unlock()
			if retire == "shutdown" {
				a.stopCapture()
			}
			close(release)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("refresh did not finish")
			}
			select {
			case got := <-events:
				t.Fatalf("retired display refresh published: %+v", got)
			default:
			}
		})
	}
}
