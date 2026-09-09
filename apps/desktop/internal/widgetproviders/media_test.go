package widgetproviders

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"option-tab/internal/config"
	"option-tab/internal/media"
	"option-tab/internal/platform"
	"option-tab/internal/widgets"
)

type mediaAdapterFake struct {
	mu                sync.Mutex
	state             media.State
	next              uint64
	subs              map[uint64]platform.MediaProvider
	prepared, release chan struct{}
	commands          []platform.MediaCommand
}

func (f *mediaAdapterFake) Subscribe(p platform.MediaProvider) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.subs[f.next] = p
	return f.next
}

func (f *mediaAdapterFake) Unsubscribe(id uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.subs, id)
}

func (f *mediaAdapterFake) Snapshot(platform.MediaProvider) media.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *mediaAdapterFake) Command(ctx context.Context, id uint64, s platform.MediaScope, k string, pos int64, g func() error) error {
	if f.prepared != nil {
		close(f.prepared)
		<-f.release
	}
	if err := g(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.subs[id]; !ok {
		return media.ErrRetired
	}
	f.commands = append(f.commands, platform.MediaCommand{Scope: s, Kind: k, PositionMS: pos})
	return nil
}

func mediaAdapterFixture() *mediaAdapterFake {
	return &mediaAdapterFake{subs: map[uint64]platform.MediaProvider{}, state: media.State{Provider: platform.MediaMusic, Epoch: 2, Revision: 3, Sample: platform.MediaSample{Provider: platform.MediaMusic, Process: platform.MediaProcess{PID: 123, LaunchID: "launch"}, Generation: 4, Sequence: 1, TrackEpoch: 1, Track: platform.MediaTrack{ID: "A", Title: "Title", Artist: "Artist", Album: "Album", DurationMS: 10000}, Status: "ready", Playback: "playing", PositionMS: 1234, Capabilities: platform.MediaCapabilities{Play: true, Pause: true, Seek: true, Next: true}}}}
}

func startMediaAdapter(t *testing.T, f *mediaAdapterFake, caps ...string) (*Media, chan time.Time, <-chan widgets.Sample, context.CancelFunc, <-chan error) {
	t.Helper()
	p := newMedia(f, platform.MediaMusic)
	ticks := make(chan time.Time)
	p.clock = func() (<-chan time.Time, func()) { return ticks, func() {} }
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan widgets.Sample, 20)
	done := make(chan error, 1)
	go func() { done <- p.Observe(ctx, caps, func(s widgets.Sample) { out <- s }) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("observer failed to join")
		}
	})
	return p, ticks, out, cancel, done
}

func mediaNext(t *testing.T, ch <-chan widgets.Sample) widgets.Sample {
	t.Helper()
	select {
	case s := <-ch:
		return s
	case <-time.After(time.Second):
		t.Fatal("missing media sample")
		return widgets.Sample{}
	}
}

func TestMediaAdapterCapabilitiesAndSharedReservation(t *testing.T) {
	f := mediaAdapterFixture()
	other := f.Subscribe(platform.MediaMusic)
	p, _, out, cancel, _ := startMediaAdapter(t, f, "media.music.read")
	s := mediaNext(t, out)
	if len(s.Actions) != 0 || *s.Fields["position"].Number != 1234 || *s.Fields["title"].Text != "Title" {
		t.Fatalf("read projection: %+v", s)
	}
	if err := p.Perform(context.Background(), widgets.ProviderAction{Generation: s.Generation, Action: "pause"}, func() error { return nil }); err == nil {
		t.Fatal("read-only command accepted")
	}
	cancel()
	// Observe cleanup must remove only its own reservation.
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		_, kept := f.subs[other]
		n := len(f.subs)
		f.mu.Unlock()
		if kept && n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("unsubscribed unrelated owner or leaked widget")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestMediaAdapterControlOnlyAndTrackAuthority(t *testing.T) {
	f := mediaAdapterFixture()
	p, ticks, out, _, _ := startMediaAdapter(t, f, "media.music.control")
	a := mediaNext(t, out)
	if len(a.Fields) != 0 || !a.Actions["pause"].Enabled || a.Actions["seek"].Range.Max != 9999 {
		t.Fatalf("control projection: %+v", a)
	}
	f.mu.Lock()
	f.state.Sample.Sequence++
	f.state.Sample.PositionMS++
	f.mu.Unlock()
	ticks <- time.Now()
	progress := mediaNext(t, out)
	if progress.Generation != a.Generation {
		t.Fatal("progress invalidated command authority")
	}
	if err := p.Perform(context.Background(), widgets.ProviderAction{Generation: a.Generation, Action: "playPause"}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.state.Sample.Track.ID = "B"
	f.state.Sample.TrackEpoch++
	f.mu.Unlock()
	ticks <- time.Now()
	b := mediaNext(t, out)
	f.mu.Lock()
	f.state.Sample.Track.ID = "A"
	f.state.Sample.TrackEpoch++
	f.mu.Unlock()
	ticks <- time.Now()
	again := mediaNext(t, out)
	if a.Generation == b.Generation || a.Generation == again.Generation {
		t.Fatal("A-B-A reused authority")
	}
	if err := p.Perform(context.Background(), widgets.ProviderAction{Generation: a.Generation, Action: "pause"}, func() error { return nil }); err == nil {
		t.Fatal("old track accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.commands) != 1 || f.commands[0].Kind != "pause" {
		t.Fatalf("commands %+v", f.commands)
	}
}

func TestMediaAdapterBlockedPreparationRevoked(t *testing.T) {
	f := mediaAdapterFixture()
	f.prepared = make(chan struct{})
	f.release = make(chan struct{})
	p, _, out, cancel, _ := startMediaAdapter(t, f, "media.music.control")
	s := mediaNext(t, out)
	done := make(chan error, 1)
	go func() {
		done <- p.Perform(context.Background(), widgets.ProviderAction{Generation: s.Generation, Action: "pause"}, func() error { return nil })
	}()
	<-f.prepared
	cancel()
	close(f.release)
	if err := <-done; err == nil {
		t.Fatal("revoked prepared command dispatched")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.commands) != 0 {
		t.Fatal("native command dispatched")
	}
}

func TestMediaAdapterFinalGuardRechecksFreshScope(t *testing.T) {
	f := mediaAdapterFixture()
	p, _, out, _, _ := startMediaAdapter(t, f, "media.music.control")
	s := mediaNext(t, out)
	err := p.Perform(context.Background(), widgets.ProviderAction{Generation: s.Generation, Action: "pause"}, func() error { f.mu.Lock(); f.state.Sample.TrackEpoch += 2; f.mu.Unlock(); return nil })
	if !errors.Is(err, widgets.ErrRetired) {
		t.Fatalf("fresh A-B-A change not rejected: %v", err)
	}
}

type mediaAdapterSource struct{ starts chan context.Context }

func (f *mediaAdapterSource) ObserveMedia(ctx context.Context, p platform.MediaProvider, emit func(platform.MediaSample)) error {
	s := mediaAdapterFixture().state.Sample
	s.Provider = p
	emit(s)
	f.starts <- ctx
	<-ctx.Done()
	return ctx.Err()
}

func (*mediaAdapterSource) RequestMediaPermission(context.Context, platform.MediaProvider) (platform.MediaPermission, error) {
	panic("unexpected permission request")
}

func (*mediaAdapterSource) ReadMediaArtwork(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
	panic("unexpected artwork read")
}

func (*mediaAdapterSource) PerformMediaCommandGuarded(context.Context, platform.MediaCommand, func() error) error {
	panic("unexpected native command")
}

func TestMediaAdapterSharesRealControllerAndHonorsDisabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "shared"}[enabled], func(t *testing.T) {
			source := &mediaAdapterSource{starts: make(chan context.Context, 4)}
			controller := media.NewController(media.Deps{Source: source})
			controller.Configure(config.DockMediaSettings{Enabled: enabled, MusicEnabled: true})
			other := controller.Subscribe(platform.MediaMusic)
			runCtx, stop := context.WithCancel(context.Background())
			joined := make(chan struct{})
			go func() { controller.Run(runCtx); close(joined) }()
			t.Cleanup(func() {
				stop()
				select {
				case <-joined:
				case <-time.After(time.Second):
					t.Error("controller did not join")
				}
			})
			var native context.Context
			if enabled {
				select {
				case native = <-source.starts:
				case <-time.After(time.Second):
					t.Fatal("native owner not started")
				}
			}
			adapter := NewMedia(controller, platform.MediaMusic)
			ctx, cancel := context.WithCancel(context.Background())
			out := make(chan widgets.Sample, 2)
			done := make(chan error, 1)
			go func() {
				done <- adapter.Observe(ctx, []string{"media.music.read"}, func(s widgets.Sample) { out <- s })
			}()
			s := mediaNext(t, out)
			if enabled && s.Status != "ready" {
				t.Fatalf("unexpected state %+v", s)
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			select {
			case <-source.starts:
				t.Fatal("widget created another native owner")
			default:
			}
			if native != nil && native.Err() != nil {
				t.Fatal("widget unsubscribe cancelled unrelated subscriber")
			}
			controller.Unsubscribe(other)
			if native != nil {
				select {
				case <-native.Done():
				case <-time.After(time.Second):
					t.Fatal("last subscriber did not stop native owner")
				}
			}
			stop()
			<-joined
			select {
			case <-source.starts:
				t.Fatal("disabled/shutdown controller started native observation")
			default:
			}
		})
	}
}

func TestMediaAdapterRejectsFreshProcessAndControllerEpoch(t *testing.T) {
	for _, change := range []func(*media.State){func(s *media.State) { s.Sample.Process.LaunchID = "reusedPID" }, func(s *media.State) { s.Epoch++ }} {
		f := mediaAdapterFixture()
		p, _, out, _, _ := startMediaAdapter(t, f, "media.music.control")
		s := mediaNext(t, out)
		f.mu.Lock()
		change(&f.state)
		f.mu.Unlock()
		if err := p.Perform(context.Background(), widgets.ProviderAction{Generation: s.Generation, Action: "pause"}, func() error { return nil }); err == nil {
			t.Fatal("fresh identity replacement accepted before polling")
		}
	}
}

func TestMediaAdapterSeekRangeRecheckedAfterGuard(t *testing.T) {
	f := mediaAdapterFixture()
	p, _, out, _, _ := startMediaAdapter(t, f, "media.music.control")
	s := mediaNext(t, out)
	value := float64(9000)
	err := p.Perform(context.Background(), widgets.ProviderAction{Generation: s.Generation, Action: "seek", Value: &value}, func() error { f.mu.Lock(); f.state.Sample.Track.DurationMS = 5000; f.mu.Unlock(); return nil })
	if err == nil {
		t.Fatal("seek crossed changed duration after external guard")
	}
}

func TestMediaAdapterRuntimeTextAndStatusBounds(t *testing.T) {
	f := mediaAdapterFixture()
	f.state.Sample.Track.Title = strings.Repeat("A", 1025)
	f.state.Sample.Track.Artist = "artist\x00tail"
	f.state.Sample.Track.Album = strings.Repeat("界", 1025)
	f.state.Sample.Reason = strings.Repeat("界", 100)
	p := mediaProjection(f.state.Sample, true, true)
	if len(*p.Fields["title"].Text) != 1024 || utf8.RuneCountInString(*p.Fields["album"].Text) > 1024 || strings.ContainsRune(*p.Fields["artist"].Text, 0) || len(p.Reason) > 160 || !utf8.ValidString(p.Reason) || !p.Actions["pause"].Enabled {
		t.Fatal("valid native long metadata violates runtime contract and loses controls")
	}
	for status, want := range map[string]string{"notRunning": "unavailable", "denied": "permissionRequired"} {
		f.state.Sample.Status = status
		if got := mediaProjection(f.state.Sample, true, true); got.Status != want {
			t.Fatalf("%s projected as %s", status, got.Status)
		}
	}
}
