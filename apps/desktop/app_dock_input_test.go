package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type appInputPlatform struct {
	*fake.Fake
	apps             []domain.App
	appsEntered      chan struct{}
	appsRelease      chan struct{}
	activateCalls    []domain.AppID
	observeCalls     int
	completedGesture uint64
	observeStarted   chan struct{}
	observeStopped   chan struct{}
}

func (p *appInputPlatform) Apps() ([]domain.App, error) {
	if p.appsEntered != nil {
		select {
		case p.appsEntered <- struct{}{}:
		default:
		}
		<-p.appsRelease
	}
	return append([]domain.App(nil), p.apps...), nil
}

func (p *appInputPlatform) ActivateApp(id domain.AppID) error {
	p.activateCalls = append(p.activateCalls, id)
	return nil
}

func (p *appInputPlatform) ObserveDockInput(ctx context.Context, _ platform.DockInputPolicy, _ <-chan platform.DockInputTarget, _ func(platform.DockInputEvent)) error {
	p.observeCalls++
	if p.observeStarted != nil {
		close(p.observeStarted)
	}
	<-ctx.Done()
	if p.observeStopped != nil {
		close(p.observeStopped)
	}
	return ctx.Err()
}

func TestDockInputObserverStaysOffUntilEnabledAndStopsAtShutdown(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}, observeStarted: make(chan struct{}), observeStopped: make(chan struct{})}
	a := newApp(p, dockEnabledSettings(), "")
	a.startDock()
	select {
	case <-p.observeStarted:
		t.Fatal("disabled input installed a native observer")
	default:
	}
	s := dockEnabledSettings()
	s.Dock.Input.ClickToHide = true
	a.saveMu.Lock()
	if err := a.saveSettingsLocked(s); err != nil {
		t.Fatal(err)
	}
	a.saveMu.Unlock()
	<-p.observeStarted
	a.stopCapture()
	<-p.observeStopped
}
func (p *appInputPlatform) DockInputGestureCurrent(uint64, uint64) bool { return true }
func (p *appInputPlatform) CompleteDockInputGesture(_, gesture uint64) {
	p.completedGesture = gesture
}

func inputAction(kind string) dock.InputAction {
	return dock.InputAction{Intent: dock.Intent{Generation: 4, GestureID: 8, Kind: kind, AppID: 10}, Item: platform.DockItem{AppID: 10, BundleID: "fixture.app", Path: "/Fixture.app"}}
}

func TestDockInputInitializesOnlyWithCompleteSourceCapabilities(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	a := newApp(p, dockEnabledSettings(), "")
	defer a.stopCapture()
	if a.dockInput == nil {
		t.Fatal("complete native input source was not wired")
	}
	if a.dockInputError != "" {
		t.Fatal("initial input status must be clear")
	}
}

func enabledDockActionSettings() config.Settings {
	s := dockEnabledSettings()
	s.Dock.Input.ClickToHide = true
	s.Dock.Input.ScrollShowHide = true
	s.Dock.Input.ModifiedRightClick = true
	return s
}

func TestDockInputWindowlessShowActivatesExactFreshApp(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	a := newApp(p, enabledDockActionSettings(), "")
	defer a.stopCapture()
	if err := a.executeDockInput(inputAction("show"), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(p.activateCalls) != 1 || p.activateCalls[0] != 10 {
		t.Fatalf("exact windowless app was not activated: %v", p.activateCalls)
	}
}

func TestDockInputBlockedLookupThenSuspensionPreventsNativeAction(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}, appsEntered: make(chan struct{}, 1), appsRelease: make(chan struct{})}
	a := newApp(p, enabledDockActionSettings(), "")
	defer a.stopCapture()
	done := make(chan error, 1)
	go func() { done <- a.executeDockInput(inputAction("hide"), func() error { return nil }) }()
	<-p.appsEntered
	a.viewMu.Lock()
	a.switcherVisible = true
	a.syncDockInputLocked()
	a.viewMu.Unlock()
	close(p.appsRelease)
	if err := <-done; !errors.Is(err, dock.ErrInputRetired) {
		t.Fatalf("retired lookup error=%v", err)
	}
	if len(p.HideCalls) != 0 {
		t.Fatal("retired lookup reached native hide")
	}
}

func TestDockInputRespectsAppBlacklistAndRetainsVisibleError(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	s := enabledDockActionSettings()
	s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "fixture.app", Hide: config.HideAlways}}
	a := newApp(p, s, "")
	defer a.stopCapture()
	if err := a.executeDockInput(inputAction("hide"), func() error { return nil }); err == nil {
		t.Fatal("blacklisted app action accepted")
	}
	a.setDockInputError(errors.New("Input Monitoring permission required"))
	var statusEvents int
	a.eventSink = func(name string, _ any) {
		if name == "dock:input-status" {
			statusEvents++
		}
	}
	a.setDockInputError(errors.New("Input Monitoring permission required again"))
	if got := a.GetDockState(); got.Error == "" {
		t.Fatal("hidden input failure was silently lost")
	}
	a.setDockInputError(nil)
	if got := a.GetDockState(); got.Error != "" {
		t.Fatal("recovered source did not clear retained status")
	}
	if statusEvents != 2 {
		t.Fatalf("hidden status changes were not published: %d", statusEvents)
	}
}

func TestDockInputSettingsChangeAfterEligibilityRetiresAction(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	a := newApp(p, enabledDockActionSettings(), "")
	defer a.stopCapture()
	a.dockController = dock.NewController(dock.Deps{}, enabledDockActionSettings())
	calls := 0
	err := a.executeDockInput(inputAction("hide"), func() error {
		calls++
		if calls == 2 {
			s := a.settingsSnapshot()
			s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "fixture.app", Hide: config.HideAlways}}
			a.settingsMu.Lock()
			a.settings = s
			a.settingsMu.Unlock()
			a.configureDock(s)
		}
		return nil
	})
	if !errors.Is(err, dock.ErrInputRetired) || len(p.HideCalls) != 0 {
		t.Fatalf("settings-invalidated action reached native: error=%v calls=%v", err, p.HideCalls)
	}
}

func TestDockInputCannotActInWindowPreviewDisablePublicationGap(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	a := newApp(p, enabledDockActionSettings(), "")
	defer a.stopCapture()
	calls := 0
	err := a.executeDockInput(inputAction("hide"), func() error {
		calls++
		if calls == 2 {
			s := folderEnabledSettings()
			a.settingsMu.Lock()
			a.settings = s
			a.settingsMu.Unlock()
		}
		return nil
	})
	if !errors.Is(err, dock.ErrInputRetired) || len(p.HideCalls) != 0 {
		t.Fatalf("app action survived window-preview disable: %v calls=%v", err, p.HideCalls)
	}
}

func TestDockInputRejectsChangedPolicyBeforeConfigurationDelivery(t *testing.T) {
	for _, allOff := range []bool{true, false} {
		t.Run(fmt.Sprint(allOff), func(t *testing.T) {
			p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
			a := newApp(p, enabledDockActionSettings(), "")
			defer a.stopCapture()
			calls := 0
			err := a.executeDockInput(inputAction("hide"), func() error {
				calls++
				if calls == 2 {
					s := enabledDockActionSettings()
					s.Dock.Input.ClickToHide = false
					if allOff {
						s.Dock.Input.ScrollShowHide = false
						s.Dock.Input.ModifiedRightClick = false
					}
					a.settingsMu.Lock()
					a.settings = s
					a.settingsMu.Unlock()
				}
				return nil
			})
			if !errors.Is(err, dock.ErrInputRetired) || len(p.HideCalls) != 0 {
				t.Fatalf("policy change reached native action: %v calls=%v", err, p.HideCalls)
			}
		})
	}
}

func TestDockInputRejectsNonAppCapturedItem(t *testing.T) {
	p := &appInputPlatform{Fake: fake.New(), apps: []domain.App{{ID: 10, BundleID: "fixture.app"}}}
	a := newApp(p, enabledDockActionSettings(), "")
	defer a.stopCapture()
	action := inputAction("hide")
	action.Item.Kind = "folder"
	if err := a.executeDockInput(action, func() error { return nil }); err == nil || len(p.HideCalls) != 0 {
		t.Fatalf("folder item reached native app action: %v calls=%v", err, p.HideCalls)
	}
}
