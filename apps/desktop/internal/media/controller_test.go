package media

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

type observation struct {
	provider  platform.MediaProvider
	emit      func(platform.MediaSample)
	cancelled chan struct{}
	release   chan struct{}
}
type fakeMedia struct {
	preparationRelease <-chan struct{}
	observe            func(context.Context, platform.MediaProvider, func(platform.MediaSample)) error
	starts             chan observation
	commands           chan func() error
	hold               bool
	mu                 sync.Mutex
	dispatched         int
}

func (f *fakeMedia) ObserveMedia(ctx context.Context, p platform.MediaProvider, emit func(platform.MediaSample)) error {
	if f.observe != nil {
		return f.observe(ctx, p, emit)
	}
	o := observation{p, emit, make(chan struct{}), make(chan struct{})}
	f.starts <- o
	<-ctx.Done()
	close(o.cancelled)
	if f.hold {
		<-o.release
	}
	return ctx.Err()
}

func (*fakeMedia) RequestMediaPermission(context.Context, platform.MediaProvider) (platform.MediaPermission, error) {
	panic("controller must not request permission")
}

func (*fakeMedia) ReadMediaArtwork(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
	panic("controller must not read artwork")
}

func (f *fakeMedia) PerformMediaCommandGuarded(ctx context.Context, _ platform.MediaCommand, g func() error) error {
	if f.commands != nil {
		f.commands <- g
		var timeout <-chan time.Time
		if f.preparationRelease == nil {
			timeout = time.After(100 * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
		case <-f.preparationRelease:
		}
	}
	if e := g(); e != nil {
		return e
	}
	f.mu.Lock()
	f.dispatched++
	f.mu.Unlock()
	return nil
}

func settings() config.DockMediaSettings {
	return config.DockMediaSettings{Enabled: true, MusicEnabled: true, SpotifyEnabled: true}
}

func sample(p platform.MediaProvider, seq, epoch uint64) platform.MediaSample {
	return platform.MediaSample{Provider: p, Generation: 1, Sequence: seq, TrackEpoch: epoch, Process: platform.MediaProcess{PID: 19, LaunchID: "one"}, Track: platform.MediaTrack{ID: "A", DurationMS: 1000}, Status: "ready", Capabilities: platform.MediaCapabilities{Play: true, Pause: true, Next: true, Previous: true, Seek: true}}
}

func scope(s platform.MediaSample) platform.MediaScope {
	return platform.MediaScope{Provider: s.Provider, Process: s.Process, Generation: s.Generation, TrackEpoch: s.TrackEpoch, TrackID: s.Track.ID}
}

func receive(t *testing.T, ch <-chan observation) observation {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(time.Second):
		t.Fatal("no observation")
		return observation{}
	}
}

func wait(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !f() {
		if time.Now().After(deadline) {
			t.Fatal("condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func run(t *testing.T, f *fakeMedia) (*Controller, context.CancelFunc, chan struct{}) {
	t.Helper()
	c := NewController(Deps{Source: f})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("controller did not join")
		}
	})
	return c, cancel, done
}

func TestControllerSharedSubscriptionsAndDisabledReservations(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 8), hold: true}
	c, _, _ := run(t, f)
	a := c.Subscribe(platform.MediaMusic)
	b := c.Subscribe(platform.MediaMusic)
	if a == 0 || a == b {
		t.Fatal("unique subscriptions")
	}
	c.Configure(settings())
	o := receive(t, f.starts)
	o.emit(sample(o.provider, 1, 1))
	wait(t, func() bool { return c.Snapshot(o.provider).Sample.Sequence == 1 })
	c.Unsubscribe(a)
	select {
	case <-o.cancelled:
		t.Fatal("remaining subscriber lost owner")
	default:
	}
	c.Configure(config.DockMediaSettings{})
	<-o.cancelled
	c.Configure(settings())
	o.emit(sample(o.provider, 2, 1))
	if c.Snapshot(o.provider).Sample.Status == "ready" {
		t.Fatal("disabled callback adopted")
	}
	select {
	case <-f.starts:
		t.Fatal("replacement before join")
	default:
	}
	close(o.release)
	next := receive(t, f.starts)
	next.emit(sample(next.provider, 1, 1))
	wait(t, func() bool { return c.Snapshot(next.provider).Sample.Status == "ready" })
	c.Unsubscribe(b)
	<-next.cancelled
	close(next.release)
}

func TestControllerCommandBusyAndSubscriptionRetirement(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 4), commands: make(chan func() error, 4)}
	c, _, _ := run(t, f)
	c.Configure(settings())
	id := c.Subscribe(platform.MediaMusic)
	other := c.Subscribe(platform.MediaMusic)
	o := receive(t, f.starts)
	s := sample(o.provider, 1, 1)
	o.emit(s)
	wait(t, func() bool { return c.Snapshot(o.provider).Sample.Sequence == 1 })
	done := make(chan error, 1)
	go func() { done <- c.Command(context.Background(), id, scope(s), "play", 0, func() error { return nil }) }()
	guard := <-f.commands
	if e := c.Command(context.Background(), other, scope(s), "play", 0, func() error { return nil }); !errors.Is(e, ErrBusy) {
		t.Fatalf("busy=%v", e)
	}
	c.Unsubscribe(id)
	if e := guard(); e == nil {
		t.Fatal("closed subscription guard accepted")
	}
	if e := <-done; e == nil {
		t.Fatal("retired command accepted")
	}
}

func TestControllerTrackABAAndGuardRace(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 4)}
	c, _, _ := run(t, f)
	c.Configure(settings())
	id := c.Subscribe(platform.MediaMusic)
	o := receive(t, f.starts)
	s := sample(o.provider, 1, 1)
	o.emit(s)
	wait(t, func() bool { return c.Snapshot(o.provider).Sample.Sequence == 1 })
	b := sample(o.provider, 2, 2)
	b.Track.ID = "B"
	o.emit(b)
	back := sample(o.provider, 3, 3)
	o.emit(back)
	if e := c.Command(context.Background(), id, scope(s), "seek", 20, func() error { return nil }); e == nil {
		t.Fatal("ABA accepted")
	}
	if e := c.Command(context.Background(), id, scope(back), "play", 0, func() error { c.Configure(config.DockMediaSettings{}); return nil }); e == nil {
		t.Fatal("disable inside external guard accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dispatched != 0 {
		t.Fatal("stale native dispatch")
	}
}

func TestControllerIndependentFailureRetryAndRecovery(t *testing.T) {
	starts := make(chan platform.MediaProvider, 8)
	fail := make(chan struct{})
	f := &fakeMedia{}
	var mu sync.Mutex
	attempts := 0
	f.observe = func(ctx context.Context, p platform.MediaProvider, emit func(platform.MediaSample)) error {
		starts <- p
		if p == platform.MediaMusic {
			mu.Lock()
			attempts++
			n := attempts
			mu.Unlock()
			if n == 1 {
				<-fail
				return errors.New("music denied")
			}
		}
		emit(sample(p, 1, 1))
		<-ctx.Done()
		return ctx.Err()
	}
	c, _, _ := run(t, f)
	c.Configure(settings())
	c.Subscribe(platform.MediaMusic)
	c.Subscribe(platform.MediaSpotify)
	<-starts
	<-starts
	close(fail)
	wait(t, func() bool { return c.Snapshot(platform.MediaMusic).Sample.Reason == "music denied" })
	if c.Snapshot(platform.MediaSpotify).Sample.Status != "ready" {
		t.Fatal("provider failure leaked")
	}
	select {
	case <-starts:
		t.Fatal("unbounded immediate retry")
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case p := <-starts:
		if p != platform.MediaMusic {
			t.Fatal("wrong provider restarted")
		}
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("no bounded recovery retry")
	}
	wait(t, func() bool { return c.Snapshot(platform.MediaMusic).Sample.Status == "ready" })
}

func TestControllerBlockedChangedDoesNotBlockProviderRetirement(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 8)}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	c := NewController(Deps{Source: f, Changed: func(s State) {
		if s.Sample.Status == "ready" {
			once.Do(func() { close(entered); <-release })
		}
	}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()
	c.Configure(settings())
	c.Subscribe(platform.MediaMusic)
	o := receive(t, f.starts)
	o.emit(sample(o.provider, 1, 1))
	<-entered
	c.Configure(config.DockMediaSettings{})
	select {
	case <-o.cancelled:
	case <-time.After(time.Second):
		t.Fatal("blocked Changed retained native owner")
	}
	c.Configure(settings())
	newOwner := receive(t, f.starts)
	o.emit(sample(o.provider, 2, 1))
	if c.Snapshot(o.provider).Sample.Status == "ready" {
		t.Fatal("old callback revived")
	}
	newOwner.emit(sample(o.provider, 1, 1))
	cancel()
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run failed to join")
	}
	if c.Subscribe(platform.MediaMusic) != 0 {
		t.Fatal("terminal owner accepted subscription")
	}
}

func TestControllerProcessRestartAndMalformedCommands(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 4)}
	c, _, _ := run(t, f)
	c.Configure(settings())
	id := c.Subscribe(platform.MediaMusic)
	o := receive(t, f.starts)
	s := sample(o.provider, 1, 1)
	o.emit(s)
	replacement := s
	replacement.Sequence = 2
	replacement.Generation = 2
	replacement.Process.LaunchID = "replacement"
	o.emit(replacement)
	if e := c.Command(context.Background(), id, scope(s), "play", 0, func() error { return nil }); e == nil {
		t.Fatal("old process admitted")
	}
	for _, cmd := range []struct {
		kind string
		pos  int64
	}{{"toggle", 0}, {"seek", -1}, {"seek", 1000}, {"seek", 1001}} {
		if e := c.Command(context.Background(), id, scope(replacement), cmd.kind, cmd.pos, func() error { return nil }); e == nil {
			t.Fatalf("invalid command accepted %+v", cmd)
		}
	}
	if e := c.Command(context.Background(), id, scope(replacement), "seek", 500, func() error { return nil }); e != nil {
		t.Fatal(e)
	}
}

func TestControllerProcessRestartDuringNativePreparation(t *testing.T) {
	release := make(chan struct{})
	f := &fakeMedia{starts: make(chan observation, 4), commands: make(chan func() error, 1), preparationRelease: release}
	c, _, _ := run(t, f)
	c.Configure(settings())
	id := c.Subscribe(platform.MediaMusic)
	o := receive(t, f.starts)
	initial := sample(o.provider, 1, 1)
	o.emit(initial)
	result := make(chan error, 1)
	go func() {
		result <- c.Command(context.Background(), id, scope(initial), "play", 0, func() error { return nil })
	}()
	<-f.commands // The accepted command has reached native preparation.
	replacement := initial
	replacement.Sequence = 2
	replacement.Generation = 2
	replacement.Process.LaunchID = "second-launch"
	o.emit(replacement)
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, ErrRetired) {
			t.Fatalf("old process completion=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("native preparation did not finish")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dispatched != 0 {
		t.Fatal("old process received native dispatch")
	}
}

func TestControllerImmediateResubscribeWaitsForOldOwnerJoin(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 4), hold: true}
	c, _, _ := run(t, f)
	c.Configure(settings())
	oldID := c.Subscribe(platform.MediaMusic)
	old := receive(t, f.starts)
	initial := sample(old.provider, 1, 1)
	old.emit(initial)
	c.Unsubscribe(oldID)
	newID := c.Subscribe(platform.MediaMusic)
	<-old.cancelled
	old.emit(sample(old.provider, 2, 1))
	if err := c.Command(context.Background(), newID, scope(initial), "play", 0, func() error { return nil }); err == nil {
		t.Fatal("new reservation used retiring native owner")
	}
	select {
	case <-f.starts:
		t.Fatal("new owner started before old join")
	case <-time.After(20 * time.Millisecond):
	}
	f.mu.Lock()
	dispatched := f.dispatched
	f.mu.Unlock()
	if dispatched != 0 {
		t.Fatal("dispatch before old owner joined")
	}
	close(old.release)
	next := receive(t, f.starts)
	next.emit(initial)
	if err := c.Command(context.Background(), newID, scope(initial), "play", 0, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	c.Unsubscribe(newID)
	<-next.cancelled
	close(next.release)
}

func TestConfigurePreservesUnchangedProviderAndArtworkObservation(t *testing.T) {
	f := &fakeMedia{starts: make(chan observation, 4)}
	c, cancel, done := run(t, f)
	defer func() { cancel(); <-done }()
	c.Configure(settings())
	c.Subscribe(platform.MediaMusic)
	spotifyID := c.Subscribe(platform.MediaSpotify)
	first, second := receive(t, f.starts), receive(t, f.starts)
	music, spotify := first, second
	if first.provider == platform.MediaSpotify {
		music, spotify = second, first
	}
	spotifySample := sample(platform.MediaSpotify, 1, 1)
	spotify.emit(spotifySample)
	wait(t, func() bool { return c.Snapshot(platform.MediaSpotify).Sample.Sequence == 1 })
	before := c.Snapshot(platform.MediaSpotify).Epoch
	changed := settings()
	changed.MusicEnabled = false
	c.Configure(changed)
	if c.Snapshot(platform.MediaSpotify).Epoch != before {
		t.Fatal("unrelated provider setting retired Spotify")
	}
	select {
	case <-music.cancelled:
	case <-time.After(time.Second):
		t.Fatal("Music not cancelled")
	}
	changed.RemoteArtwork = true
	c.Configure(changed)
	if c.Snapshot(platform.MediaSpotify).Epoch != before {
		t.Fatal("artwork opt-in retired provider observation")
	}
	if err := c.Command(context.Background(), spotifyID, scope(spotifySample), "play", 0, func() error { return nil }); err != nil {
		t.Fatalf("unaffected provider lost command authority: %v", err)
	}
	select {
	case <-spotify.cancelled:
		t.Fatal("Spotify source cancelled")
	case <-f.starts:
		t.Fatal("duplicate source started")
	default:
	}
}
