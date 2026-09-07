package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeMediaTransport struct {
	sample          MediaSample
	commands        int
	permissionCalls []bool
	before          func()
	commandEntered  chan struct{}
	commandRelease  <-chan time.Time
	err             error
}

func (f *fakeMediaTransport) read(context.Context, MediaProvider) (MediaSample, string, error) {
	return f.sample, "", f.err
}

func (f *fakeMediaTransport) permission(_ context.Context, _ MediaProvider, ask bool) (MediaPermission, error) {
	f.permissionCalls = append(f.permissionCalls, ask)
	return MediaPermission{Status: "denied"}, nil
}

func (f *fakeMediaTransport) command(_ context.Context, _ MediaCommand, guard func() error) error {
	if f.commandEntered != nil {
		close(f.commandEntered)
		<-f.commandRelease
	}
	if f.before != nil {
		f.before()
	}
	if err := guard(); err != nil {
		return err
	}
	f.commands++
	return nil
}

func TestMediaCancelledInFlightCommandJoinsBounded(t *testing.T) {
	observation, cancel := context.WithCancel(context.Background())
	entered := make(chan struct{})
	f := &fakeMediaTransport{commandEntered: entered, commandRelease: time.After(100 * time.Millisecond)}
	s := newMediaSource(f)
	o := s.owners[MediaMusic]
	o.active = true
	o.observation = observation
	o.sample = MediaSample{Provider: MediaMusic, Generation: 1, TrackEpoch: 1, Process: MediaProcess{PID: 123, LaunchID: "one"}, Track: MediaTrack{ID: "track"}, Status: "ready", Capabilities: MediaCapabilities{Play: true}}
	done := make(chan error, 1)
	go func() {
		done <- s.PerformMediaCommandGuarded(context.Background(), MediaCommand{Scope: mediaScope(o.sample), Kind: "play"}, func() error { return nil })
	}()
	<-entered
	cancel()
	select {
	case err := <-done:
		if err == nil || f.commands != 0 {
			t.Fatalf("cancelled in-flight command err=%v commands=%d", err, f.commands)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("cancelled in-flight command did not join")
	}
}

func (f *fakeMediaTransport) artwork(context.Context, MediaScope, string) ([]byte, error) {
	return nil, nil
}

func TestMediaRetiredGuardDoesNotMutate(t *testing.T) {
	f := &fakeMediaTransport{sample: MediaSample{Provider: MediaMusic, Process: MediaProcess{PID: 123, LaunchID: "one"}, Track: MediaTrack{ID: "track", DurationMS: 1000}, Status: "ready", Capabilities: MediaCapabilities{Play: true}}}
	s := newMediaSource(f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan MediaSample, 1)
	done := make(chan error, 1)
	go func() {
		done <- s.ObserveMedia(ctx, MediaMusic, func(v MediaSample) {
			select {
			case ready <- v:
			default:
			}
		})
	}()
	v := <-ready
	refusal := errors.New("retired")
	err := s.PerformMediaCommandGuarded(ctx, MediaCommand{Scope: mediaScope(v), Kind: "play"}, func() error { return refusal })
	if !errors.Is(err, refusal) || f.commands != 0 {
		t.Fatalf("err=%v commands=%d", err, f.commands)
	}
	cancel()
	<-done
	if err := s.PerformMediaCommandGuarded(context.Background(), MediaCommand{Scope: mediaScope(v), Kind: "play"}, func() error { return nil }); err == nil {
		t.Fatal("retired observer admitted")
	}
}

func TestMediaPermissionIsExplicitAndProviderValidated(t *testing.T) {
	f := &fakeMediaTransport{}
	s := newMediaSource(f)
	if _, err := s.RequestMediaPermission(context.Background(), "other"); err == nil {
		t.Fatal("unknown provider accepted")
	}
	if len(f.permissionCalls) != 0 {
		t.Fatal("unknown provider reached transport")
	}
	_, err := s.RequestMediaPermission(context.Background(), MediaMusic)
	if err != nil || len(f.permissionCalls) != 1 || !f.permissionCalls[0] {
		t.Fatal("explicit permission missing")
	}
}

type blockingMediaArtwork struct {
	fakeMediaTransport
	entered, release chan struct{}
}

func (f *blockingMediaArtwork) artwork(context.Context, MediaScope, string) ([]byte, error) {
	close(f.entered)
	<-f.release
	return []byte("image"), nil
}

func TestMediaRemoteArtworkDoesNotBlockCommandsAndRejectsRetirement(t *testing.T) {
	f := &blockingMediaArtwork{entered: make(chan struct{}), release: make(chan struct{})}
	s := newMediaSource(f)
	o := s.owners[MediaSpotify]
	o.active = true
	o.sample = MediaSample{Provider: MediaSpotify, Generation: 1, TrackEpoch: 1, Track: MediaTrack{ID: "A"}, Status: "ready", Capabilities: MediaCapabilities{Pause: true}, ArtworkToken: "opaque"}
	o.artwork = "https://fixture.invalid/image"
	result := make(chan error, 1)
	go func() {
		_, err := s.ReadMediaArtwork(context.Background(), mediaScope(o.sample), "opaque")
		result <- err
	}()
	<-f.entered
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := s.PerformMediaCommandGuarded(ctx, MediaCommand{Scope: mediaScope(o.sample), Kind: "pause"}, func() error { return nil })
	if err != nil {
		close(f.release)
		<-result
		t.Fatalf("remote artwork blocked command: %v", err)
	}
	owned, _ := s.acquire(context.Background(), MediaSpotify)
	owned.active = false
	mediaRelease(owned)
	close(f.release)
	if err := <-result; err == nil {
		t.Fatal("retired remote artwork published")
	}
}

func TestMediaCancelledObservationInvalidatesPreparedCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &fakeMediaTransport{before: cancel}
	s := newMediaSource(f)
	o := s.owners[MediaMusic]
	o.active = true
	o.observation = ctx
	o.sample = MediaSample{Provider: MediaMusic, Track: MediaTrack{ID: "A"}, Status: "ready", Capabilities: MediaCapabilities{Play: true}}
	err := s.PerformMediaCommandGuarded(context.Background(), MediaCommand{Scope: mediaScope(o.sample), Kind: "play"}, func() error { return nil })
	if err == nil || f.commands != 0 {
		t.Fatalf("cancelled observer mutated: %v %d", err, f.commands)
	}
}
