package main

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type switcherGesturePlatform struct {
	*fake.Fake
	replaced        atomic.Bool
	actions         chan string
	prepare         func()
	identityRead    func()
	validationCalls atomic.Int32
}

func (p *switcherGesturePlatform) Apps() ([]domain.App, error) {
	return []domain.App{{ID: 10, Name: "Fixture", BundleID: "fixture.app"}}, nil
}
func (p *switcherGesturePlatform) ActivateApp(domain.AppID) error { return nil }
func (p *switcherGesturePlatform) ProcessIdentity(pid domain.AppID) (platform.ProcessIdentity, error) {
	return platform.ProcessIdentity{PID: pid, StartSeconds: 100}, nil
}

func (p *switcherGesturePlatform) WindowIdentity(id domain.WindowID) (platform.AutomationWindowIdentity, error) {
	if p.identityRead != nil {
		p.identityRead()
	}
	return platform.AutomationWindowIdentity{ID: id, Process: platform.ProcessIdentity{PID: 10, StartSeconds: 100}}, nil
}

func (p *switcherGesturePlatform) WindowIdentityCurrent(id platform.AutomationWindowIdentity) bool {
	p.validationCalls.Add(1)
	return !p.replaced.Load() && id.ID == 101 && id.Process.PID == 10 && id.Process.StartSeconds == 100
}

func (p *switcherGesturePlatform) PerformPointerAction(ctx context.Context, kind string, id platform.AutomationWindowIdentity, guard func() error) error {
	if p.prepare != nil {
		p.prepare()
	}
	if err := guard(); err != nil {
		return err
	}
	if !p.WindowIdentityCurrent(id) {
		return errors.New("identity replaced")
	}
	p.actions <- kind
	return nil
}

type switcherGestureWheel struct {
	panelWheelFixture
	done chan struct{}
	once sync.Once
}

func (w *switcherGestureWheel) Close() error          { w.once.Do(func() { close(w.done) }); return nil }
func (w *switcherGestureWheel) Done() <-chan struct{} { return w.done }
func gestureApp(t *testing.T, mode config.SwitcherMode) (*App, *switcherGesturePlatform, *switcherGestureWheel) {
	t.Helper()
	p := &switcherGesturePlatform{Fake: fake.New(), actions: make(chan string, 8)}
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, BundleID: "fixture.app", Title: "Exact", OnScreen: true, SpaceID: 1, ScreenID: 1}})
	s := config.Default()
	s.Shortcuts[0].Mode = mode
	s.Appearance.Style = config.StyleTitles
	s.Behavior.SwipeUpAction = config.PointerClose
	s.Behavior.SwipeDownAction = config.PointerMinimize
	s.AppSwitcher.Behavior.SwipeUpAction = config.PointerHide
	s.AppSwitcher.Behavior.SwipeDownAction = config.PointerQuit
	a := newApp(p, s, "")
	w := &switcherGestureWheel{panelWheelFixture: panelWheelFixture{valid: true, completed: make(chan uint64, 32)}, done: make(chan struct{})}
	if !a.installSwitcherGestureSource(w) {
		t.Fatal("source installation failed")
	}
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	t.Cleanup(func() {
		a.stopCapture()
		select {
		case <-w.done:
		case <-time.After(time.Second):
			t.Error("source not closed")
		}
	})
	return a, p, w
}

func gestureRegions() []DockPreviewRegion {
	return []DockPreviewRegion{{WindowID: 101, AppID: 10, Bounds: DockBounds{X: 10, Y: 10, W: 80, H: 50}}}
}

func publishGesture(t *testing.T, a *App, sequence uint64) {
	t.Helper()
	if err := a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, sequence, gestureRegions()); err != nil {
		t.Fatal(err)
	}
}

func sendGesture(w *switcherGestureWheel, dy float64) {
	p := w.snapshot()
	w.send(platform.DockPanelWheelEvent{Session: p.Session, Revision: p.Revision, Sequence: 1, GestureID: 1, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaY: dy, Owned: true, Precise: true, Phase: "began"})
}

func waitGesture(t *testing.T, w *switcherGestureWheel) {
	t.Helper()
	select {
	case <-w.completed:
	case <-time.After(2 * time.Second):
		t.Fatal("missing acknowledgment")
	}
}

func TestSwitcherGesturesPublishExactCopiedRegionsAndRejectStale(t *testing.T) {
	a, _, w := gestureApp(t, config.ModeWindows)
	regions := gestureRegions()
	if err := a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, 1, regions); err != nil {
		t.Fatal(err)
	}
	regions[0].WindowID = 999
	if got := w.snapshot(); !got.Enabled || got.Regions[0].WindowID != 101 {
		t.Fatalf("policy=%+v", got)
	}
	for _, args := range [][3]uint64{{0, a.switcherRevision, 2}, {a.visibleSwitcherSession, a.switcherRevision - 1, 2}, {a.visibleSwitcherSession, a.switcherRevision, 1}} {
		if a.SetSwitcherGestureRegions(args[0], args[1], args[2], gestureRegions()) == nil {
			t.Fatal("stale publication accepted")
		}
	}
	bad := gestureRegions()
	bad[0].Bounds.X = math.NaN()
	if a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, 2, bad) == nil {
		t.Fatal("NaN accepted")
	}
	bad = gestureRegions()
	bad[0].AppID = 11
	if a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, 2, bad) == nil {
		t.Fatal("different owner accepted")
	}
}

func TestSwitcherGesturesUseIndependentModePolicy(t *testing.T) {
	for _, test := range []struct {
		mode config.SwitcherMode
		dy   float64
		want string
	}{{config.ModeWindows, 90, "close"}, {config.ModeWindows, -90, "minimize"}, {config.ModeApps, 90, "hide"}, {config.ModeApps, -90, "quit"}} {
		t.Run(string(test.mode)+test.want, func(t *testing.T) {
			a, p, w := gestureApp(t, test.mode)
			publishGesture(t, a, 1)
			sendGesture(w, test.dy)
			waitGesture(t, w)
			select {
			case got := <-p.actions:
				if got != test.want {
					t.Fatal(got)
				}
			default:
				t.Fatal("no action")
			}
		})
	}
}

func TestSwitcherGesturesRetireDuringNativePreparation(t *testing.T) {
	for _, retire := range []string{"hide", "settings", "session", "preferences", "host", "identity", "shutdown"} {
		t.Run(retire, func(t *testing.T) {
			a, p, w := gestureApp(t, config.ModeWindows)
			publishGesture(t, a, 1)
			p.prepare = func() {
				switch retire {
				case "hide":
					a.Hide()
				case "settings":
					a.saveMu.Lock()
					s := a.settingsSnapshot()
					s.Behavior.SwipeUpAction = config.PointerQuit
					err := a.saveSettingsLocked(s)
					a.saveMu.Unlock()
					if err != nil {
						panic(err)
					}
				case "session":
					a.transitionSession(1, true)
				case "preferences":
					a.OpenPreferences()
				case "host":
					a.invalidateSwitcherGestures()
				case "identity":
					p.replaced.Store(true)
				case "shutdown":
					a.stopCapture()
				}
			}
			sendGesture(w, 90)
			waitGesture(t, w)
			select {
			case got := <-p.actions:
				t.Fatalf("retired action %s dispatched", got)
			default:
			}
		})
	}
}

func TestSwitcherGesturesSelectionUpdateKeepsCapturedAuthority(t *testing.T) {
	a, p, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	before := w.snapshot()
	a.controller.Refresh()
	publishGesture(t, a, 2)
	if got := w.snapshot(); got.Revision != before.Revision {
		t.Fatal("unchanged gallery retired gesture authority")
	}
	sendGesture(w, 90)
	waitGesture(t, w)
	select {
	case <-p.actions:
	default:
		t.Fatal("selection-only state prevented action")
	}
}

func TestSwitcherGesturesEmptyGeometryAndReorderedSequenceCannotRearm(t *testing.T) {
	a, p, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	old := w.snapshot()
	if err := a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, 2, nil); err != nil {
		t.Fatal(err)
	}
	if a.SetSwitcherGestureRegions(a.visibleSwitcherSession, a.switcherRevision, 1, gestureRegions()) == nil {
		t.Fatal("late geometry rearmed")
	}
	a.handleSwitcherWheel(w, platform.DockPanelWheelEvent{Session: old.Session, Revision: old.Revision, Sequence: 1, GestureID: 1, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaY: 90, Owned: true, Precise: true, Phase: "began"})
	select {
	case got := <-p.actions:
		t.Fatal(got)
	default:
	}
}

func TestSwitcherGesturesLateUpdateCannotRearmHiddenOrPreferences(t *testing.T) {
	for _, retire := range []string{"hide", "preferences", "pause"} {
		t.Run(retire, func(t *testing.T) {
			a, _, _ := gestureApp(t, config.ModeWindows)
			publishGesture(t, a, 1)
			switch retire {
			case "hide":
				a.Hide()
			case "preferences":
				a.OpenPreferences()
			case "pause":
				a.SetPaused(true)
			}
			a.controller.Refresh()
			if err := a.SetSwitcherGestureRegions(a.controller.State().Session, a.switcherRevision, 2, gestureRegions()); err == nil {
				t.Fatal("late controller update rearmed retired surface")
			}
		})
	}
}

func TestSwitcherGesturesResizeAdmitsFreshGeometry(t *testing.T) {
	a, _, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	before := w.snapshot()
	a.retireSwitcherGestureGeometry()
	publishGesture(t, a, 2)
	after := w.snapshot()
	if !after.Enabled || after.Revision <= before.Revision {
		t.Fatalf("resized host not rearmed: %+v -> %+v", before, after)
	}
}

func TestSwitcherGesturesBlockedPublicationCannotResurrectHiddenSurface(t *testing.T) {
	a, _, w := gestureApp(t, config.ModeWindows)
	w.publishEntered = make(chan struct{}, 1)
	w.publishRelease = make(chan struct{})
	session, revision := a.visibleSwitcherSession, a.switcherRevision
	result := make(chan error, 1)
	go func() { result <- a.SetSwitcherGestureRegions(session, revision, 1, gestureRegions()) }()
	<-w.publishEntered
	a.Hide()
	close(w.publishRelease)
	if err := <-result; err == nil {
		t.Fatal("blocked old publication revived")
	}
	if w.snapshot().Enabled {
		t.Fatal("native policy left enabled")
	}
}

func TestSwitcherGesturesNativeValidationRechecksRetirement(t *testing.T) {
	a, p, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	w.validateEntered = make(chan struct{}, 1)
	w.validateRelease = make(chan struct{})
	sendGesture(w, 90)
	<-w.validateEntered
	a.Hide()
	close(w.validateRelease)
	waitGesture(t, w)
	select {
	case kind := <-p.actions:
		t.Fatal(kind)
	default:
	}
}

func TestSwitcherGesturesRetiredGeometrySuppressesLateFailure(t *testing.T) {
	a, _, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	events := make(chan any, 1)
	a.eventSink = func(name string, value any) {
		if name == "switcher:gestureError" {
			events <- value
		}
	}
	before := w.snapshot()
	a.retireSwitcherGestureGeometry()
	a.emitSwitcherGestureError(before.Session, before.Revision, "native rejection arrived late")
	select {
	case event := <-events:
		t.Fatalf("retired error surfaced: %+v", event)
	default:
	}
}

func TestSwitcherGestureGeometryDoesNotPerformAXWindowLookup(t *testing.T) {
	a, p, _ := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	if p.validationCalls.Load() != 0 {
		t.Fatal("geometry publication performed a blocking AX validation")
	}
}

func TestSwitcherGesturesSlowIdentityCaptureDoesNotBlockNativeRetirement(t *testing.T) {
	a, p, w := gestureApp(t, config.ModeWindows)
	entered, release := make(chan struct{}), make(chan struct{})
	p.identityRead = func() { close(entered); <-release }
	session, revision := a.visibleSwitcherSession, a.switcherRevision
	result := make(chan error, 1)
	go func() { result <- a.SetSwitcherGestureRegions(session, revision, 1, gestureRegions()) }()
	<-entered
	a.stopSwitcherGestures()
	select {
	case <-w.done:
	case <-time.After(time.Second):
		close(release)
		<-result
		t.Fatal("native retirement waited for identity capture")
	}
	close(release)
	if err := <-result; err == nil {
		t.Fatal("retired identity reply was published")
	}
}

type switcherGestureHostPlatform struct {
	*switcherGesturePlatform
	wheel    *switcherGestureWheel
	attached atomic.Int32
}

func (p *switcherGestureHostPlatform) CreateSwitcherWheel(host unsafe.Pointer) (platform.SwitcherWheel, error) {
	if host == nil {
		return nil, errors.New("missing host")
	}
	p.attached.Add(1)
	return p.wheel, nil
}

func TestSwitcherGesturesFirstPublicationAttachesWithoutPreferences(t *testing.T) {
	base := &switcherGesturePlatform{Fake: fake.New(), actions: make(chan string, 1)}
	base.SetWindows([]domain.Window{{ID: 101, AppID: 10, BundleID: "fixture.app", Title: "Exact", OnScreen: true, SpaceID: 1, ScreenID: 1}})
	wheel := &switcherGestureWheel{panelWheelFixture: panelWheelFixture{valid: true, completed: make(chan uint64, 8)}, done: make(chan struct{})}
	p := &switcherGestureHostPlatform{switcherGesturePlatform: base, wheel: wheel}
	settings := config.Default()
	settings.Shortcuts[0].Mode = config.ModeWindows
	settings.Behavior.SwipeUpAction = config.PointerClose
	a := newApp(p, settings, "")
	a.overlay = newLiveWindow(&fakeWindow{native: unsafe.Pointer(new(int))})
	defer a.stopCapture()
	a.controller.HandleHotkey(platform.HotkeyEvent{Kind: platform.HotkeyActivate, ShortcutID: 1})
	publishGesture(t, a, 1)
	if p.attached.Load() != 1 || !wheel.snapshot().Enabled {
		t.Fatal("overlay publication depends on preferences capability RPC")
	}
}

func TestSwitcherGesturesChangedGeometryDisablesOldTargetBeforeCapture(t *testing.T) {
	a, p, w := gestureApp(t, config.ModeWindows)
	publishGesture(t, a, 1)
	old := w.snapshot()
	entered, release := make(chan struct{}), make(chan struct{})
	p.identityRead = func() { close(entered); <-release }
	moved := gestureRegions()
	moved[0].Bounds.X = 120
	session, revision := a.visibleSwitcherSession, a.switcherRevision
	result := make(chan error, 1)
	go func() { result <- a.SetSwitcherGestureRegions(session, revision, 2, moved) }()
	<-entered
	if w.snapshot().Enabled {
		close(release)
		<-result
		t.Fatal("old geometry stayed enabled during identity preparation")
	}
	a.handleSwitcherWheel(w, platform.DockPanelWheelEvent{Session: old.Session, Revision: old.Revision, Sequence: 1, GestureID: 1, Timestamp: time.Now(), WindowID: 101, AppID: 10, DeltaY: 90, Owned: true, Precise: true, Phase: "began"})
	select {
	case kind := <-p.actions:
		close(release)
		<-result
		t.Fatalf("moved card retained old target: %s", kind)
	default:
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if got := w.snapshot(); !got.Enabled || got.Regions[0].Bounds.X != 120 {
		t.Fatal("replacement geometry missing")
	}
}

func TestSwitcherGesturesNoneAndFullscreenPolicies(t *testing.T) {
	for _, action := range []config.PointerAction{config.PointerNone, config.PointerFullscreen} {
		t.Run(string(action), func(t *testing.T) {
			a, p, w := gestureApp(t, config.ModeWindows)
			a.saveMu.Lock()
			s := a.settingsSnapshot()
			s.Behavior.SwipeUpAction = action
			s.Behavior.SwipeDownAction = config.PointerNone
			err := a.saveSettingsLocked(s)
			a.saveMu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			a.controller.Refresh()
			publishGesture(t, a, 1)
			if action == config.PointerNone {
				if w.snapshot().Enabled {
					t.Fatal("none consumed native scroll")
				}
				return
			}
			sendGesture(w, 90)
			waitGesture(t, w)
			select {
			case got := <-p.actions:
				if got != "fullscreen" {
					t.Fatal(got)
				}
			default:
				t.Fatal("fullscreen action missing")
			}
		})
	}
}

func TestSwitcherGesturesNewerIdenticalPublicationSurvivesBlockedReply(t *testing.T) {
	a, _, w := gestureApp(t, config.ModeWindows)
	w.publishEntered = make(chan struct{}, 1)
	w.publishRelease = make(chan struct{})
	session, revision := a.visibleSwitcherSession, a.switcherRevision
	first, second := make(chan error, 1), make(chan error, 1)
	go func() { first <- a.SetSwitcherGestureRegions(session, revision, 1, gestureRegions()) }()
	<-w.publishEntered
	go func() { second <- a.SetSwitcherGestureRegions(session, revision, 2, gestureRegions()) }()
	deadline := time.Now().Add(time.Second)
	for {
		a.switcherGestures.mu.Lock()
		sequence := a.switcherGestures.sequence
		a.switcherGestures.mu.Unlock()
		if sequence == 2 {
			break
		}
		if time.Now().After(deadline) {
			close(w.publishRelease)
			<-first
			<-second
			t.Fatal("newer publication was not accepted")
		}
		time.Sleep(time.Millisecond)
	}
	close(w.publishRelease)
	<-first
	if err := <-second; err != nil {
		t.Fatalf("latest identical geometry lost: %v", err)
	}
	if !w.snapshot().Enabled {
		t.Fatal("latest geometry left disabled")
	}
}
