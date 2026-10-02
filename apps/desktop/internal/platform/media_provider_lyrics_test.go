package platform

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type providerLyricsTransport struct {
	fakeMediaTransport
	readLyrics func(context.Context, MediaScope, func() error) (MediaProviderLyrics, error)
	reads      atomic.Int32
}

func (f *providerLyricsTransport) lyrics(ctx context.Context, scope MediaScope, guard func() error) (MediaProviderLyrics, error) {
	f.reads.Add(1)
	if f.readLyrics != nil {
		return f.readLyrics(ctx, scope, guard)
	}
	if err := guard(); err != nil {
		return MediaProviderLyrics{}, err
	}
	return MediaProviderLyrics{Scope: scope, Data: []byte("[00:01]Original fixture"), Status: "ready"}, nil
}

func providerLyricsOwner(t *testing.T, transport mediaTransport) (*mediaSource, MediaProviderLyricsSource, MediaScope) {
	t.Helper()
	s := newMediaSource(transport)
	o := s.owners[MediaMusic]
	o.active = true
	o.observation = context.Background()
	o.sample = MediaSample{Provider: MediaMusic, Process: MediaProcess{PID: 123, LaunchID: "123.000000"}, Generation: 2, TrackEpoch: 3, Track: MediaTrack{ID: "fixture-A"}, Status: "ready", Capabilities: MediaCapabilities{Pause: true}}
	capability, ok := any(s).(MediaProviderLyricsSource)
	if !ok {
		t.Fatal("media source does not expose guarded provider lyrics")
	}
	return s, capability, mediaScope(o.sample)
}

func TestMediaProviderLyricsReturnsScopedCopiedText(t *testing.T) {
	data := []byte("Plain provider text without invented timestamps")
	f := &providerLyricsTransport{readLyrics: func(_ context.Context, scope MediaScope, guard func() error) (MediaProviderLyrics, error) {
		if err := guard(); err != nil {
			return MediaProviderLyrics{}, err
		}
		return MediaProviderLyrics{Scope: scope, Data: data, Status: "ready"}, nil
	}}
	_, source, scope := providerLyricsOwner(t, f)
	result, err := source.ReadMediaProviderLyricsGuarded(context.Background(), scope, func() error { return nil })
	if err != nil || result.Scope != scope || result.Status != "ready" || string(result.Data) != string(data) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	data[0] = '!'
	if result.Data[0] != 'P' || f.reads.Load() != 1 {
		t.Fatal("provider bytes escaped without copying or duplicate native read")
	}
}

func TestMediaProviderLyricsRejectsRetiredOrInvalidAdmission(t *testing.T) {
	for _, scenario := range []string{"pid", "birth", "generation", "epoch", "track", "emptyTrack", "zeroPID", "emptyBirth", "zeroGeneration", "zeroEpoch", "inactive", "notReady", "noObservation", "observationCancelled", "contextCancelled", "nilGuard", "guardRefused"} {
		t.Run(scenario, func(t *testing.T) {
			f := &providerLyricsTransport{}
			s, source, scope := providerLyricsOwner(t, f)
			ctx := context.Background()
			guard := func() error { return nil }
			switch scenario {
			case "pid":
				scope.Process.PID++
			case "birth":
				scope.Process.LaunchID = "replacement"
			case "generation":
				scope.Generation++
			case "epoch":
				scope.TrackEpoch++
			case "track":
				scope.TrackID = "fixture-B"
			case "emptyTrack":
				scope.TrackID = ""
				s.owners[MediaMusic].sample.Track.ID = ""
			case "zeroPID":
				scope.Process.PID = 0
				s.owners[MediaMusic].sample.Process.PID = 0
			case "emptyBirth":
				scope.Process.LaunchID = ""
				s.owners[MediaMusic].sample.Process.LaunchID = ""
			case "zeroGeneration":
				scope.Generation = 0
				s.owners[MediaMusic].sample.Generation = 0
			case "zeroEpoch":
				scope.TrackEpoch = 0
				s.owners[MediaMusic].sample.TrackEpoch = 0
			case "inactive":
				s.owners[MediaMusic].active = false
			case "notReady":
				s.owners[MediaMusic].sample.Status = "unavailable"
			case "noObservation":
				s.owners[MediaMusic].observation = nil
			case "observationCancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				s.owners[MediaMusic].observation = cancelled
			case "contextCancelled":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			case "nilGuard":
				guard = nil
			case "guardRefused":
				guard = func() error { return errors.New("retired asset") }
			}
			result, err := source.ReadMediaProviderLyricsGuarded(ctx, scope, guard)
			if err == nil || f.reads.Load() != 0 || len(result.Data) != 0 || result.Status != "unavailable" {
				t.Fatalf("result=%+v err=%v reads=%d", result, err, f.reads.Load())
			}
		})
	}
}

func TestMediaProviderLyricsUnsupportedAndMissing(t *testing.T) {
	_, source, scope := providerLyricsOwner(t, &fakeMediaTransport{})
	result, err := source.ReadMediaProviderLyricsGuarded(context.Background(), scope, func() error { return nil })
	if err != nil || result.Status != "unsupported" || result.Scope != scope {
		t.Fatalf("absent optional transport result=%+v err=%v", result, err)
	}
	f := &providerLyricsTransport{readLyrics: func(_ context.Context, scope MediaScope, _ func() error) (MediaProviderLyrics, error) {
		return MediaProviderLyrics{Scope: scope, Status: "missing"}, nil
	}}
	_, source, scope = providerLyricsOwner(t, f)
	result, err = source.ReadMediaProviderLyricsGuarded(context.Background(), scope, func() error { return nil })
	if err != nil || result.Status != "missing" || result.Scope != scope {
		t.Fatalf("missing result=%+v err=%v", result, err)
	}
	scope.Provider = MediaSpotify
	result, err = source.ReadMediaProviderLyricsGuarded(context.Background(), scope, func() error { return nil })
	if err != nil || result.Status != "unsupported" || f.reads.Load() != 1 {
		t.Fatalf("other provider result=%+v err=%v reads=%d", result, err, f.reads.Load())
	}
}

func TestMediaProviderLyricsRejectsLateResult(t *testing.T) {
	for _, scenario := range []string{"cancel", "observation", "guard", "scope", "oversize", "invalidUTF8", "transportError"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			observation, retire := context.WithCancel(context.Background())
			defer retire()
			admitted := true
			f := &providerLyricsTransport{readLyrics: func(_ context.Context, scope MediaScope, guard func() error) (MediaProviderLyrics, error) {
				if err := guard(); err != nil {
					return MediaProviderLyrics{}, err
				}
				result := MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:01]One")}
				switch scenario {
				case "cancel":
					cancel()
				case "observation":
					retire()
				case "guard":
					admitted = false
				case "scope":
					result.Scope.TrackEpoch++
				case "oversize":
					result.Data = make([]byte, (1<<20)+1)
				case "invalidUTF8":
					result.Data = []byte{0xff}
				case "transportError":
					return result, errors.New("transport failed")
				}
				return result, nil
			}}
			s, source, scope := providerLyricsOwner(t, f)
			s.owners[MediaMusic].observation = observation
			result, err := source.ReadMediaProviderLyricsGuarded(ctx, scope, func() error {
				if !admitted {
					return errors.New("asset retired")
				}
				return nil
			})
			if err == nil || len(result.Data) != 0 || result.Status != "unavailable" {
				t.Fatalf("late status=%q dataBytes=%d err=%v", result.Status, len(result.Data), err)
			}
		})
	}
}

func TestMediaProviderLyricsSerializesWithCommandsAndRechecksNativeGuard(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	observation, cancel := context.WithCancel(context.Background())
	defer cancel()
	gets := 0
	f := &providerLyricsTransport{readLyrics: func(_ context.Context, scope MediaScope, guard func() error) (MediaProviderLyrics, error) {
		close(entered)
		<-release
		if err := guard(); err != nil {
			return MediaProviderLyrics{}, err
		}
		gets++
		return MediaProviderLyrics{Scope: scope, Status: "missing"}, nil
	}}
	s, source, scope := providerLyricsOwner(t, f)
	s.owners[MediaMusic].observation = observation
	done := make(chan error, 1)
	go func() {
		_, err := source.ReadMediaProviderLyricsGuarded(context.Background(), scope, func() error { return nil })
		done <- err
	}()
	<-entered
	commandCtx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	err := s.PerformMediaCommandGuarded(commandCtx, MediaCommand{Scope: scope, Kind: "pause"}, func() error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) || f.commands != 0 {
		close(release)
		<-done
		t.Fatalf("command bypassed provider gate: %v", err)
	}
	cancel()
	close(release)
	if err := <-done; err == nil || gets != 0 {
		t.Fatalf("native preparation retirement reached getter: err=%v gets=%d", err, gets)
	}
}
