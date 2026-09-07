package media

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"sync/atomic"
	"testing"

	"option-tab/internal/platform"
)

type artworkSource struct {
	platform.MediaProviderSource
	read func(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error)
}

func (s artworkSource) ReadMediaArtwork(ctx context.Context, scope platform.MediaScope, token string) (platform.MediaArtwork, error) {
	return s.read(ctx, scope, token)
}

func artworkFixture(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 2, 1))); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func artworkScope() platform.MediaScope {
	return platform.MediaScope{Provider: platform.MediaSpotify, Process: platform.MediaProcess{PID: 42, LaunchID: "fixture-process"}, Generation: 1, TrackEpoch: 1, TrackID: "fixture-track"}
}

func TestArtworkCacheRequiresRemoteOptInAndCopiesCachedBytes(t *testing.T) {
	var calls atomic.Int32
	data := artworkFixture(t)
	source := artworkSource{read: func(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
		calls.Add(1)
		return platform.MediaArtwork{PNG: data, Status: "ready"}, nil
	}}
	cache := NewArtworkCache(source)
	scope := artworkScope()
	result, err := cache.Read(context.Background(), scope, "token", false, func() error { return nil })
	if err != nil || result.Status != "networkDisabled" || calls.Load() != 0 {
		t.Fatalf("remote artwork started withoutoptin: %+v %v", result, err)
	}
	result, err = cache.Read(context.Background(), scope, "token", true, func() error { return nil })
	if err != nil || result.Status != "ready" {
		t.Fatal(err)
	}
	result.PNG[0] = 0
	data[1] = 0
	again, err := cache.Read(context.Background(), scope, "token", true, func() error { return nil })
	if err != nil || calls.Load() != 1 || again.PNG[0] != 137 || again.PNG[1] != 80 {
		t.Fatal("cached bytes alias caller or source")
	}
	if _, err := cache.Read(context.Background(), scope, "token", true, func() error { return errors.New("retired") }); err == nil {
		t.Fatal("cache hit skipped current presentation guard")
	}
}

func TestArtworkCacheRejectsRetiredReadAndClearCancelsInFlight(t *testing.T) {
	for _, clear := range []bool{false, true} {
		entered, release := make(chan struct{}), make(chan struct{})
		var retired atomic.Bool
		data := artworkFixture(t)
		source := artworkSource{read: func(ctx context.Context, _ platform.MediaScope, _ string) (platform.MediaArtwork, error) {
			close(entered)
			if clear {
				<-ctx.Done()
			} else {
				<-release
			}
			return platform.MediaArtwork{PNG: data, Status: "ready"}, nil
		}}
		cache := NewArtworkCache(source)
		done := make(chan error, 1)
		go func() {
			_, err := cache.Read(context.Background(), artworkScope(), "token", true, func() error {
				if retired.Load() {
					return errors.New("retired")
				}
				return nil
			})
			done <- err
		}()
		<-entered
		if clear {
			cache.Clear()
		} else {
			retired.Store(true)
			close(release)
		}
		if err := <-done; err == nil {
			t.Fatal("retired artwork read was published")
		}
		if len(cache.entries) != 0 {
			t.Fatal("retired artwork entered cache")
		}
	}
}

func TestArtworkCacheDiscardsFailureFromRetiredPresentation(t *testing.T) {
	var retired bool
	source := artworkSource{read: func(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
		retired = true
		return platform.MediaArtwork{Status: "offline", Reason: "late network failure"}, errors.New("offline")
	}}
	cache := NewArtworkCache(source)
	result, err := cache.Read(context.Background(), artworkScope(), "token", true, func() error {
		if retired {
			return ErrRetired
		}
		return nil
	})
	if !errors.Is(err, ErrRetired) || result.Status != "" || result.Reason != "" {
		t.Fatalf("retired artwork failure escaped: %+v %v", result, err)
	}
}

func TestArtworkCacheEvictsLeastRecentlyUsedAndKeysFullTrackScope(t *testing.T) {
	data := artworkFixture(t)
	var calls int
	source := artworkSource{read: func(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
		calls++
		return platform.MediaArtwork{PNG: data, Status: "ready"}, nil
	}}
	cache := newArtworkCache(source, 2*len(data))
	scope := artworkScope()
	read := func(token string) {
		t.Helper()
		if _, err := cache.Read(context.Background(), scope, token, true, func() error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	read("a")
	read("b")
	read("a")
	read("c")
	read("a")
	if calls != 3 {
		t.Fatal("recently used artwork evicted")
	}
	read("b")
	if calls != 4 {
		t.Fatal("least recently used artwork retained beyond budget")
	}
	scope.TrackEpoch++
	read("b")
	if calls != 5 {
		t.Fatal("new track epoch reused old artwork")
	}
}

func TestArtworkCacheBoundsConcurrentWorkAndRejectsInvalidImage(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	source := artworkSource{read: func(ctx context.Context, _ platform.MediaScope, _ string) (platform.MediaArtwork, error) {
		entered <- struct{}{}
		<-release
		return platform.MediaArtwork{PNG: []byte("invalid PNG"), Status: "ready"}, ctx.Err()
	}}
	cache := NewArtworkCache(source)
	done := make(chan error, 2)
	for _, token := range []string{"one", "two"} {
		go func() {
			_, err := cache.Read(context.Background(), artworkScope(), token, true, func() error { return nil })
			done <- err
		}()
	}
	<-entered
	<-entered
	if _, err := cache.Read(context.Background(), artworkScope(), "three", true, func() error { return nil }); !errors.Is(err, ErrArtworkBusy) {
		t.Fatalf("excess artwork read admitted: %v", err)
	}
	close(release)
	for range 2 {
		if err := <-done; err == nil {
			t.Fatal("invalid PNG accepted into cache")
		}
	}
}
