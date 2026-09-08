package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type appMediaSource struct {
	platform.MediaProviderSource
	started         chan func(platform.MediaSample)
	stopped         chan struct{}
	permissionCalls atomic.Int32
	command         func(context.Context, platform.MediaCommand, func() error) error
	permission      func(context.Context, platform.MediaProvider) (platform.MediaPermission, error)
}

func (s *appMediaSource) ObserveMedia(ctx context.Context, _ platform.MediaProvider, emit func(platform.MediaSample)) error {
	s.started <- emit
	<-ctx.Done()
	s.stopped <- struct{}{}
	return ctx.Err()
}

func (s *appMediaSource) RequestMediaPermission(ctx context.Context, p platform.MediaProvider) (platform.MediaPermission, error) {
	s.permissionCalls.Add(1)
	if s.permission != nil {
		return s.permission(ctx, p)
	}
	return platform.MediaPermission{Status: "ready"}, nil
}

func (s *appMediaSource) PerformMediaCommandGuarded(ctx context.Context, c platform.MediaCommand, g func() error) error {
	return s.command(ctx, c, g)
}

func (s *appMediaSource) ReadMediaArtwork(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
	return platform.MediaArtwork{Status: "missing"}, nil
}

func mediaSettings() config.Settings {
	s := config.Default()
	s.Dock.Media = config.DockMediaSettings{Enabled: true, MusicEnabled: true}
	return s
}

func mediaDockFixture(session uint64) dock.State {
	s := dockFixtureState(session, 0, 42)
	s.ContentKind = "media"
	s.Item.BundleID = "com.apple.Music"
	s.Windows = nil
	return s
}

func appMediaSample() platform.MediaSample {
	return platform.MediaSample{Provider: platform.MediaMusic, Process: platform.MediaProcess{PID: 42, LaunchID: "test-player"}, Generation: 1, Sequence: 1, TrackEpoch: 1, Track: platform.MediaTrack{ID: "first", Title: "Original fixture song", DurationMS: 60_000}, Playback: "playing", PositionMS: 1000, ObservedAt: time.Now(), Status: "ready", Capabilities: platform.MediaCapabilities{Pause: true, Next: true, Seek: true}}
}

func newAppMediaFixture(t *testing.T) (*App, *appMediaSource) {
	t.Helper()
	a := newApp(fake.New(), mediaSettings(), "")
	s := &appMediaSource{started: make(chan func(platform.MediaSample), 8), stopped: make(chan struct{}, 8)}
	a.wireMedia(s, nil)
	a.startMedia()
	t.Cleanup(a.stopCapture)
	return a, s
}

func mediaReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("media fixture deadline")
		var zero T
		return zero
	}
}

func waitAppMedia(t *testing.T, a *App, id uint64, predicate func(*MediaViewState) bool) *MediaViewState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := a.GetMediaState(id)
		if s != nil && predicate(s) {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("media presentation did not converge")
	return nil
}

func TestAppMediaHoverOwnsObservationWithoutCaptureOrPermission(t *testing.T) {
	a, s := newAppMediaFixture(t)
	select {
	case <-s.started:
		t.Fatal("media observed without a presentation")
	default:
	}
	a.showDock(mediaDockFixture(1), true)
	dockState := a.GetDockState()
	if dockState.ContentKind != "media" || dockState.Media == nil || a.captureDockSession.Load() != 0 {
		t.Fatalf("invalid media hover: %+v", dockState)
	}
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, dockState.Media.Session, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	if state.Scope.TrackID != "first" || s.permissionCalls.Load() != 0 {
		t.Fatal("hover prompted or lost captured track")
	}
	if err := a.validateDockTarget(1, 0, 42, false); err == nil {
		t.Fatal("media acquired window-action authority")
	}
	a.hideDock(1)
	mediaReceive(t, s.stopped)
	if a.GetMediaState(state.Session) != nil {
		t.Fatal("hidden hover retained media session")
	}
}

func TestAppMediaExistingHoverUpdatesAppearance(t *testing.T) {
	a, _ := newAppMediaFixture(t)
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	st := mediaDockFixture(1)
	st.Appearance.Theme = "light"
	st.Appearance.FontSizePx = 19
	a.showDock(st, false)
	if got := a.GetMediaState(id); got.Appearance.Theme != "light" || got.Appearance.FontSizePx != 19 {
		t.Fatalf("media appearance stayed stale: %+v", got.Appearance)
	}
}

func TestAppMediaPreparedCommandRejectsDockAdmissionChange(t *testing.T) {
	a, s := newAppMediaFixture(t)
	a.dockController = dock.NewController(dock.Deps{}, mediaSettings())
	entered, release := make(chan struct{}), make(chan struct{})
	var actions atomic.Int32
	s.command = func(_ context.Context, _ platform.MediaCommand, guard func() error) error {
		close(entered)
		<-release
		if err := guard(); err != nil {
			return err
		}
		actions.Add(1)
		return nil
	}
	st := mediaDockFixture(1)
	st.AdmissionEpoch = a.dockController.AdmissionEpoch()
	a.showDock(st, true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	done := make(chan error, 1)
	go func() { done <- a.PerformMediaAction(id, state.Revision, "pause", 0) }()
	mediaReceive(t, entered)
	a.dockController.Suspend(true)
	a.dockController.Suspend(false)
	close(release)
	if err := mediaReceive(t, done); err == nil || actions.Load() != 0 {
		t.Fatalf("retired Dock admission dispatched: %v", err)
	}
}

func TestAppMediaPreparedCommandCannotTargetReopenedPanel(t *testing.T) {
	a, s := newAppMediaFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var actions atomic.Int32
	s.command = func(_ context.Context, _ platform.MediaCommand, guard func() error) error {
		close(entered)
		<-release
		if err := guard(); err != nil {
			return err
		}
		actions.Add(1)
		return nil
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	done := make(chan error, 1)
	go func() { done <- a.PerformMediaAction(id, state.Revision, "pause", 0) }()
	mediaReceive(t, entered)
	a.hideDock(1)
	a.showDock(mediaDockFixture(2), true)
	next := a.GetDockState().Media.Session
	close(release)
	if err := mediaReceive(t, done); err == nil || actions.Load() != 0 {
		t.Fatalf("retired command dispatched: %v", err)
	}
	if got := a.GetMediaState(next); got == nil || got.Error != "" {
		t.Fatal("old failure contaminated replacement panel")
	}
}

func TestAppMediaFailurePublishesUsableRevision(t *testing.T) {
	a, s := newAppMediaFixture(t)
	var calls atomic.Int32
	s.command = func(_ context.Context, _ platform.MediaCommand, g func() error) error {
		if err := g(); err != nil {
			return err
		}
		if calls.Add(1) == 1 {
			return errors.New("fixture transport failure")
		}
		return nil
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	if err := a.PerformMediaAction(id, state.Revision, "pause", 0); err == nil {
		t.Fatal("fixture failure lost")
	}
	next := a.GetMediaState(id)
	if next.Revision <= state.Revision || next.Error == "" {
		t.Fatal("failure didn't publish fresh revision")
	}
	if err := a.PerformMediaAction(id, next.Revision, "pause", 0); err != nil {
		t.Fatalf("updated revision refused: %v", err)
	}
}

func TestAppMediaPermissionPendingSurvivesDisableUntilNativeDrain(t *testing.T) {
	for _, cancelBeforeDrain := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "disabled and reenabled"}[cancelBeforeDrain], func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			events := make(chan platform.MediaPermission, 8)
			a.eventSink = func(name string, data any) {
				if name != "media:permission" {
					return
				}
				encoded, _ := json.Marshal(data)
				var event struct {
					Provider string
					Status   string
					Reason   string
				}
				if json.Unmarshal(encoded, &event) == nil && event.Provider == "music" {
					events <- platform.MediaPermission{Status: event.Status, Reason: event.Reason}
				}
			}
			source.permission = func(context.Context, platform.MediaProvider) (platform.MediaPermission, error) {
				close(entered)
				<-release
				return platform.MediaPermission{Status: "ready"}, nil
			}
			done := make(chan error, 1)
			go func() { _, err := a.ConnectMediaProvider("music"); done <- err }()
			mediaReceive(t, entered)
			status := a.GetMediaPermissions()
			if status["music"].Status != "connecting" || status["spotify"].Status == "connecting" {
				t.Fatalf("pending snapshot is not provider-scoped: %#v", status)
			}
			if got := mediaReceive(t, events); got.Status != "connecting" {
				t.Fatalf("start event = %#v", got)
			}
			if cancelBeforeDrain {
				for _, enabled := range []bool{false, true} {
					a.settingsMu.Lock()
					a.settings.Dock.Media.Enabled = enabled
					a.settingsMu.Unlock()
					a.viewMu.Lock()
					a.syncMediaLocked()
					a.viewMu.Unlock()
					if got := a.GetMediaPermissions()["music"].Status; got != "connecting" {
						t.Fatalf("released pending status before native drain: %s", got)
					}
				}
			}
			if _, err := a.ConnectMediaProvider("music"); err == nil {
				t.Fatal("duplicate permission request was admitted")
			}
			if source.permissionCalls.Load() != 1 {
				t.Fatal("duplicate native permission call")
			}
			close(release)
			err := mediaReceive(t, done)
			want := "ready"
			if cancelBeforeDrain {
				want = "permissionRequired"
				if err == nil {
					t.Fatal("retired native success was accepted")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got := a.GetMediaPermissions()["music"].Status; got != want {
				t.Fatalf("drained status = %s; want %s", got, want)
			}
			if got := mediaReceive(t, events); got.Status != want {
				t.Fatalf("drain event = %#v; want %s", got, want)
			}
		})
	}
}

func TestAppMediaAcceptedPermissionSurvivesHoverHideButDisableCancels(t *testing.T) {
	a, s := newAppMediaFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	s.permission = func(ctx context.Context, _ platform.MediaProvider) (platform.MediaPermission, error) {
		close(entered)
		<-ctx.Done()
		<-release
		return platform.MediaPermission{Status: "ready"}, nil
	}
	done := make(chan error, 1)
	go func() { _, err := a.ConnectMediaProvider("music"); done <- err }()
	mediaReceive(t, entered)
	a.showDock(mediaDockFixture(1), true)
	a.hideDock(1)
	select {
	case <-done:
		t.Fatal("hover hide abandoned accepted permission")
	default:
	}
	a.settingsMu.Lock()
	a.settings.Dock.Media.Enabled = false
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	if _, err := a.ConnectMediaProvider("music"); err == nil {
		t.Fatal("disabled media accepted another prompt")
	}
	close(release)
	if err := mediaReceive(t, done); err == nil {
		t.Fatal("retired permission result accepted")
	}
}

func TestAppMediaPinSharesProviderAndOutlivesHover(t *testing.T) {
	a, s := newAppMediaFixture(t)
	a.platform.(*fake.Fake).ScreenList = []domain.Screen{{ID: 1, Main: true, Visible: domain.Bounds{W: 1440, H: 900}}}
	var created int
	a.mediaPinFactory = func(uint64, platform.MediaProvider, func(platform.MediaPanelEvent)) *dockWindow {
		created++
		d, _, _, _ := dockHarness()
		return d
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	pin, err := a.PinMediaPanel(id, state.Revision)
	if err != nil {
		t.Fatal(err)
	}
	current := a.GetMediaState(id)
	again, err := a.PinMediaPanel(id, current.Revision)
	if err != nil || again != pin || created != 1 {
		t.Fatal("repeated pin created another host")
	}
	a.hideDock(1)
	if p := a.GetMediaState(pin); p == nil || !p.Open || !p.Pinned {
		t.Fatal("hover hide closed the pin")
	}
	select {
	case <-s.stopped:
		t.Fatal("hover hide stopped pinned provider")
	case <-s.started:
		t.Fatal("pin started duplicate provider observation")
	default:
	}
	p := a.GetMediaState(pin)
	if err := a.CloseMediaPanel(pin, p.Revision); err != nil {
		t.Fatal(err)
	}
	mediaReceive(t, s.stopped)
	if a.GetMediaState(pin) != nil {
		t.Fatal("closed pin retained presentation")
	}
}

func TestAppMediaPinSuspendsAndResumesWithoutReusingOldEvents(t *testing.T) {
	a, s := newAppMediaFixture(t)
	a.platform.(*fake.Fake).ScreenList = []domain.Screen{{ID: 1, Main: true, Visible: domain.Bounds{W: 1440, H: 900}}}
	a.mediaPinFactory = func(uint64, platform.MediaProvider, func(platform.MediaPanelEvent)) *dockWindow {
		d, _, _, _ := dockHarness()
		return d
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, s.started)
	emit(appMediaSample())
	state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	pin, err := a.PinMediaPanel(id, state.Revision)
	if err != nil {
		t.Fatal(err)
	}
	before := a.GetMediaState(pin)
	a.setSessionInactive(true)
	mediaReceive(t, s.stopped)
	if p := a.GetMediaState(pin); p == nil || p.Open {
		t.Fatal("inactive session kept pin visible")
	}
	a.setSessionInactive(false)
	emit = mediaReceive(t, s.started)
	sample := appMediaSample()
	sample.Generation = 2
	emit(sample)
	after := waitAppMedia(t, a, pin, func(s *MediaViewState) bool { return s.Open && s.Sample.Generation == 2 })
	if after.Revision <= before.Revision {
		t.Fatal("resumed pin reused previous revision")
	}
	if err := a.PerformMediaAction(pin, before.Revision, "pause", 0); err == nil {
		t.Fatal("old queued pin event was admitted")
	}
}
