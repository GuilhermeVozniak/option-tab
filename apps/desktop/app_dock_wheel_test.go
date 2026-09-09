package main

import (
	"errors"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type panelWheelFixture struct {
	mu              sync.Mutex
	policy          platform.DockPanelWheelPolicy
	emit            func(platform.DockPanelWheelEvent)
	valid           bool
	completed       chan uint64
	validateEntered chan struct{}
	validateRelease chan struct{}
	publishEntered  chan struct{}
	publishRelease  chan struct{}
}

func (f *panelWheelFixture) SetDockPanelWheelPolicy(p platform.DockPanelWheelPolicy, emit func(platform.DockPanelWheelEvent)) error {
	if p.Enabled && f.publishEntered != nil {
		select {
		case f.publishEntered <- struct{}{}:
		default:
		}
		<-f.publishRelease
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	p.Regions = append([]platform.PreviewRegion(nil), p.Regions...)
	f.policy, f.emit = p, emit
	return nil
}

func (f *panelWheelFixture) ValidateDockPanelWheelGesture(session, revision, gesture uint64) bool {
	f.mu.Lock()
	valid := f.valid && session == f.policy.Session && revision == f.policy.Revision && gesture != 0
	f.mu.Unlock()
	if f.validateEntered != nil {
		select {
		case f.validateEntered <- struct{}{}:
		default:
		}
		<-f.validateRelease
	}
	return valid
}

func (f *panelWheelFixture) CompleteDockPanelWheelGesture(_, _, gesture uint64) {
	f.completed <- gesture
}

func (f *panelWheelFixture) snapshot() platform.DockPanelWheelPolicy {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.policy
}

func (f *panelWheelFixture) send(e platform.DockPanelWheelEvent) {
	f.mu.Lock()
	emit := f.emit
	f.mu.Unlock()
	emit(e)
}

func wheelApp(t *testing.T, input config.DockInputSettings) (*App, *fake.Fake, *panelWheelFixture) {
	t.Helper()
	p := fake.New()
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, BundleID: "fixture.app", Title: "Exact", OnScreen: true, SpaceID: 1, ScreenID: 1}})
	s := dockEnabledSettings()
	s.Dock.Input = input
	a := newApp(p, s, "")
	a.showDock(dockFixtureState(7, 101, 10), true)
	w := &panelWheelFixture{valid: true, completed: make(chan uint64, 4)}
	a.dockWheelSource = w
	t.Cleanup(a.stopCapture)
	return a, p, w
}

func region101() []DockPreviewRegion {
	return []DockPreviewRegion{{WindowID: 101, AppID: 10, Bounds: DockBounds{X: 10, Y: 10, W: 80, H: 50}}}
}

func TestDockPanelWheelPolicyDefaultsDisabledAndCopiesRegions(t *testing.T) {
	a, _, wheel := wheelApp(t, config.Default().Dock.Input)
	regions := region101()
	if err := a.SetDockPreviewRegions(7, 3, regions); err != nil {
		t.Fatal(err)
	}
	regions[0].WindowID = 999
	policy := wheel.snapshot()
	if policy.Enabled || policy.Session != 7 || policy.Revision != 3 || policy.Regions[0].WindowID != 101 {
		t.Fatalf("unexpected copied default policy: %+v", policy)
	}
}

func TestDockPanelWheelActsOnImmutableExactWindowAndAcknowledges(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, p, wheel := wheelApp(t, input)
	if err := a.SetDockPreviewRegions(7, 4, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	at := time.Now()
	for i, dx := range []float64{10, 75} {
		wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: uint64(i + 1), GestureID: 8, Timestamp: at.Add(time.Duration(i) * time.Millisecond), WindowID: 101, AppID: 10, DeltaX: dx, Owned: true, Precise: true, Phase: "changed"})
	}
	select {
	case gesture := <-wheel.completed:
		if gesture != 8 {
			t.Fatalf("ack=%d", gesture)
		}
	case <-time.After(time.Second):
		t.Fatal("gesture was not acknowledged")
	}
	if len(p.CloseCalls) != 1 || p.CloseCalls[0] != 101 {
		t.Fatalf("exact action calls=%v", p.CloseCalls)
	}
}

func TestDockPanelWheelRetirementPreventsActionButStillAcknowledges(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, p, wheel := wheelApp(t, input)
	if err := a.SetDockPreviewRegions(7, 2, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	a.viewMu.Lock()
	a.switcherVisible = true
	a.viewMu.Unlock()
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 9, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaX: 90, Owned: true, Precise: true, Phase: "changed"})
	select {
	case <-wheel.completed:
	case <-time.After(time.Second):
		t.Fatal("retired gesture was not acknowledged")
	}
	if len(p.CloseCalls) != 0 {
		t.Fatal("retired gesture reached native window action")
	}
}

func TestDockPanelWheelTerminalWithoutIntentAcknowledges(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 12, Timestamp: time.Now(), WindowID: 101, AppID: 10, Owned: true, Precise: true, Phase: "ended"})
	select {
	case gesture := <-wheel.completed:
		if gesture != 12 {
			t.Fatal(gesture)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal no-intent gesture was not acknowledged")
	}
}

func TestDockPanelWheelSettingsReconfigurationInvalidatesNativePolicy(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatal(err)
	}
	s := a.settingsSnapshot()
	s.Dock.Input.SwipeNext = config.PointerNone
	a.saveMu.Lock()
	if err := a.saveSettingsLocked(s); err != nil {
		t.Fatal(err)
	}
	a.saveMu.Unlock()
	policy := wheel.snapshot()
	if policy.Enabled || policy.Session != 0 || policy.Revision != 0 {
		t.Fatalf("old native wheel policy survived settings change: %+v", policy)
	}
}

func TestDockPanelWheelDoesNotAcknowledgeTerminalWhileAcceptedActionIsPending(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, p, wheel := wheelApp(t, input)
	wheel.validateEntered, wheel.validateRelease = make(chan struct{}, 1), make(chan struct{})
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 20, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaX: 90, Owned: true, Precise: true, Phase: "changed"})
	<-wheel.validateEntered
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 2, GestureID: 20, Timestamp: time.Now().Add(time.Millisecond), WindowID: 101, AppID: 10, Owned: true, Precise: true, Phase: "ended"})
	select {
	case <-wheel.completed:
		t.Fatal("terminal packet retired pending action token")
	default:
	}
	close(wheel.validateRelease)
	select {
	case <-wheel.completed:
	case <-time.After(time.Second):
		t.Fatal("completed action was not acknowledged")
	}
	if len(p.CloseCalls) != 1 {
		t.Fatalf("accepted action calls=%v", p.CloseCalls)
	}
}

func TestDockPanelWheelRechecksAppStateAfterNativeValidator(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, p, wheel := wheelApp(t, input)
	wheel.validateEntered, wheel.validateRelease = make(chan struct{}, 1), make(chan struct{})
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 21, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaX: 90, Owned: true, Precise: true, Phase: "changed"})
	<-wheel.validateEntered
	a.viewMu.Lock()
	a.switcherVisible = true
	a.viewMu.Unlock()
	close(wheel.validateRelease)
	select {
	case <-wheel.completed:
	case <-time.After(time.Second):
		t.Fatal("retired action was not acknowledged")
	}
	if len(p.CloseCalls) != 0 {
		t.Fatal("state retired during validator still reached action")
	}
}

func TestDockPanelWheelRechecksPolicyRevisionAfterNativeValidator(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, p, wheel := wheelApp(t, input)
	wheel.validateEntered, wheel.validateRelease = make(chan struct{}, 1), make(chan struct{})
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatal(err)
	}
	policy := wheel.snapshot()
	wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 22, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaX: 90, Owned: true, Precise: true, Phase: "changed"})
	<-wheel.validateEntered
	if err := a.SetDockPreviewRegions(7, 2, region101()); err != nil {
		t.Fatal(err)
	}
	close(wheel.validateRelease)
	select {
	case <-wheel.completed:
	case <-time.After(time.Second):
		t.Fatal("revision-retired action was not acknowledged")
	}
	if len(p.CloseCalls) != 0 {
		t.Fatal("old policy action survived newer region revision")
	}
}

func TestDockPanelWheelBlockedOldPublicationCannotResurrectHiddenPolicy(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	wheel.publishEntered, wheel.publishRelease = make(chan struct{}, 1), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- a.SetDockPreviewRegions(7, 2, region101()) }()
	<-wheel.publishEntered
	a.hideDock(7)
	close(wheel.publishRelease)
	if err := <-done; !errors.Is(err, errStaleDockSession) {
		t.Fatalf("publication error=%v", err)
	}
	if policy := wheel.snapshot(); policy.Enabled || policy.Session != 0 {
		t.Fatalf("stale policy resurrected after hide: %+v", policy)
	}
}

func TestDockPanelWheelRejectsReorderedFrontendRevision(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	if err := a.SetDockPreviewRegions(7, 5, region101()); err != nil {
		t.Fatal(err)
	}
	if err := a.SetDockPreviewRegions(7, 4, region101()); !errors.Is(err, errStaleDockSession) {
		t.Fatalf("old revision error=%v", err)
	}
	if policy := wheel.snapshot(); policy.Revision != 5 {
		t.Fatalf("old geometry replaced revision: %+v", policy)
	}
}

func TestDockPanelWheelRejectsMediaEvenWithEmptyRegions(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	a.viewMu.Lock()
	a.dockState.ContentKind = "media"
	a.dockState.Windows = nil
	a.viewMu.Unlock()
	if err := a.SetDockPreviewRegions(7, 4, nil); !errors.Is(err, errStaleDockSession) {
		t.Fatalf("media accepted wheel policy: %v", err)
	}
	if wheel.snapshot().Session != 0 {
		t.Fatal("media installed native window wheel policy")
	}
	if a.currentDockWheelPresentation(7, a.dockState.AdmissionEpoch) {
		t.Fatal("media remained eligible after publication")
	}
}

func TestDockPanelWheelSettingsPublishedDuringValidatorRetiresWindowAction(t *testing.T) {
	for _, disable := range []bool{false, true} {
		name := "media enabled"
		if disable {
			name = "window previews disabled"
		}
		t.Run(name, func(t *testing.T) {
			input := config.Default().Dock.Input
			input.SwipeNext = config.PointerClose
			a, p, wheel := wheelApp(t, input)
			a.viewMu.Lock()
			a.dockState.Item.BundleID = "com.apple.Music"
			a.viewMu.Unlock()
			wheel.validateEntered, wheel.validateRelease = make(chan struct{}, 1), make(chan struct{})
			if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
				t.Fatal(err)
			}
			policy := wheel.snapshot()
			wheel.send(platform.DockPanelWheelEvent{Session: 7, Revision: policy.Revision, Sequence: 1, GestureID: 27, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaX: 90, Owned: true, Precise: true, Phase: "changed"})
			<-wheel.validateEntered
			a.settingsMu.Lock()
			a.settings.Dock.Media = config.DockMediaSettings{Enabled: true, MusicEnabled: !disable, SpotifyEnabled: true}
			if disable {
				a.settings.Dock.Enabled = false
			}
			a.settingsMu.Unlock()
			close(wheel.validateRelease)
			select {
			case <-wheel.completed:
			case <-time.After(time.Second):
				t.Fatal("action not acknowledged")
			}
			if len(p.CloseCalls) != 0 {
				t.Fatal("window action survived current media/window-disabled settings")
			}
			if a.currentDockWheelPresentation(7, a.dockState.AdmissionEpoch) {
				t.Fatal("stale window presentation still admitted")
			}
			if err := a.SetDockPreviewRegions(7, 2, nil); !errors.Is(err, errStaleDockSession) {
				t.Fatalf("stale content published new policy: %v", err)
			}
		})
	}
}

func TestDockPanelWheelExplicitMediaAppWindowsAdmitsRegions(t *testing.T) {
	input := config.Default().Dock.Input
	input.SwipeNext = config.PointerClose
	a, _, wheel := wheelApp(t, input)
	a.settingsMu.Lock()
	a.settings.Dock.Media = config.DockMediaSettings{Enabled: true, MusicEnabled: true}
	a.settingsMu.Unlock()
	a.viewMu.Lock()
	a.dockState.Item.BundleID = "com.apple.Music"
	a.dockState.ContentKind = "windows"
	a.dockState.ContentOptions = []string{"windows", "media"}
	a.viewMu.Unlock()
	if err := a.SetDockPreviewRegions(7, 1, region101()); err != nil {
		t.Fatalf("selected windows rejected: %v", err)
	}
	if !wheel.snapshot().Enabled || !a.currentDockWheelPresentation(7, a.dockState.AdmissionEpoch) {
		t.Fatal("selected windows did not acquire native policy")
	}
	a.viewMu.Lock()
	a.dockState.ContentKind = "media"
	a.viewMu.Unlock()
	if a.currentDockWheelPresentation(7, a.dockState.AdmissionEpoch) {
		t.Fatal("media replacement retained window policy")
	}
}
