package main

import (
	"testing"

	"option-tab/internal/dock"
	"option-tab/internal/platform/fake"
)

func TestDockContentDTOCopiesOptionsAndPreservesQueryError(t *testing.T) {
	a := newApp(fake.New(), dockEnabledSettings(), "")
	defer a.stopCapture()
	s := dockFixtureState(3, 101, 10)
	s.ContentOptions = []string{"windows", "media"}
	s.Error = "fixture inventory unavailable"
	a.showDock(s, true)
	s.ContentOptions[0] = "corrupted"
	dto := a.GetDockState()
	if len(dto.ContentOptions) != 2 || dto.ContentOptions[0] != "windows" || dto.Error != s.Error {
		t.Fatalf("incorrect state: %+v", dto)
	}
	dto.ContentOptions[0] = "corrupted snapshot"
	if a.GetDockState().ContentOptions[0] != "windows" {
		t.Fatal("snapshot aliases live options")
	}
}

func TestDockContentRPCRejectsStaleRevisionAndUnavailableOption(t *testing.T) {
	a := newApp(fake.New(), dockEnabledSettings(), "")
	defer a.stopCapture()
	a.dockController = dock.NewController(dock.Deps{}, dockEnabledSettings())
	s := dockFixtureState(1, 101, 10)
	s.ContentOptions = []string{"windows"}
	a.showDock(s, true)
	dto := a.GetDockState()
	for _, request := range []struct {
		session, revision uint64
		kind              string
	}{
		{1, 0, "windows"}, {1, dto.Revision + 1, "windows"}, {2, dto.Revision, "windows"}, {1, dto.Revision, "media"},
	} {
		if err := a.SelectDockContent(request.session, request.revision, request.kind); err == nil {
			t.Fatalf("admitted stale/unavailable request %+v", request)
		}
	}
	// Settings publication can precede the queued controller Configure command.
	s.ContentOptions = []string{"windows", "media"}
	s.Item.BundleID = "com.apple.Music"
	a.showDock(s, false)
	dto = a.GetDockState()
	a.settingsMu.Lock()
	a.settings.Dock.Enabled = false
	a.settings.Dock.Media.Enabled = true
	a.settings.Dock.Media.MusicEnabled = true
	a.settingsMu.Unlock()
	if err := a.SelectDockContent(1, dto.Revision, "windows"); err != errStaleDockSession {
		t.Fatalf("current settings did not refuse immediately: %v", err)
	}
}

func TestDockContentReplacementRetiresMediaAndWindowAuthority(t *testing.T) {
	a, source := newAppMediaFixture(t)
	settings := a.settingsSnapshot()
	settings.Dock.Enabled = true
	a.settingsMu.Lock()
	a.settings = settings
	a.settingsMu.Unlock()
	first := mediaDockFixture(1)
	first.ContentOptions = []string{"windows", "media"}
	a.showDock(first, true)
	mediaReceive(t, source.started)
	oldMedia := a.GetDockState().Media.Session
	windows := dockFixtureState(2, 99, 42)
	windows.Item.BundleID = "com.apple.Music"
	windows.ContentOptions = first.ContentOptions
	a.showDock(windows, true)
	mediaReceive(t, source.stopped)
	if a.GetMediaState(oldMedia) != nil || a.GetDockState().Media != nil || a.captureDockSession.Load() != 2 {
		t.Fatal("windows retained media or lost capture owner")
	}
	if err := a.validateDockTarget(2, 99, 42, true); err != nil {
		t.Fatal(err)
	}
	first.Session = 3
	a.showDock(first, true)
	mediaReceive(t, source.started)
	if a.GetDockState().Media.Session == oldMedia || a.captureDockSession.Load() != 0 {
		t.Fatal("media reused owner or retained capture")
	}
	if err := a.validateDockTarget(2, 99, 42, true); err == nil {
		t.Fatal("old window authority survived media switch")
	}
}
