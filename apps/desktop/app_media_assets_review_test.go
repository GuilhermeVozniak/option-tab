package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync/atomic"
	"testing"

	"option-tab/internal/domain"
	"option-tab/internal/media"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type reviewArtworkSource struct {
	platform.MediaProviderSource
	entered, release chan struct{}
	png              []byte
	fresh            []byte
	calls            atomic.Int32
}

func (s *reviewArtworkSource) ReadMediaArtwork(context.Context, platform.MediaScope, string) (platform.MediaArtwork, error) {
	if s.calls.Add(1) > 1 {
		return platform.MediaArtwork{Status: "ready", PNG: s.fresh}, nil
	}
	close(s.entered)
	<-s.release
	return platform.MediaArtwork{Status: "ready", PNG: s.png}, nil
}

func TestAppMediaReviewLateSuccessfulArtworkCannotPublish(t *testing.T) {
	for _, optOut := range []bool{false, true} {
		name := "track replacement"
		if optOut {
			name = "remote opt out"
		}
		t.Run(name, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			a.settingsMu.Lock()
			a.settings.Dock.Media.SpotifyEnabled = true
			a.settings.Dock.Media.RemoteArtwork = true
			a.settingsMu.Unlock()
			a.configureDock(a.settingsSnapshot())
			st := mediaDockFixture(1)
			if optOut {
				st.Item.BundleID = "com.spotify.client"
			}
			a.showDock(st, true)
			id := a.GetDockState().Media.Session
			emit := mediaReceive(t, source.started)
			sample := appMediaSample()
			if optOut {
				sample.Provider = platform.MediaSpotify
			}
			emit(sample)
			waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
				t.Fatal(err)
			}
			art := &reviewArtworkSource{entered: make(chan struct{}), release: make(chan struct{}), png: encoded.Bytes()}
			a.viewMu.Lock()
			asset := a.media.assets[sample.Provider]
			asset.token = "review-token"
			a.media.artwork = media.NewArtworkCache(art)
			a.viewMu.Unlock()
			done := make(chan struct{})
			go func() { defer close(done); a.readMediaArtwork(asset, asset.ctx, asset.artworkRevision, true) }()
			mediaReceive(t, art.entered)
			if optOut {
				a.settingsMu.Lock()
				a.settings.Dock.Media.RemoteArtwork = false
				a.settingsMu.Unlock()
				a.configureDock(a.settingsSnapshot())
			} else {
				sample.Sequence = 2
				sample.TrackEpoch = 2
				sample.Track.ID = "replacement"
				emit(sample)
				waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Scope.TrackID == "replacement" })
			}
			close(art.release)
			mediaReceive(t, done)
			got := a.GetMediaState(id)
			if got == nil || got.Artwork.Image != "" || got.Artwork.Status == "ready" {
				t.Fatalf("late success published: %+v", got)
			}
		})
	}
}

func TestAppMediaReviewImportHideDisableAndDrainOwnership(t *testing.T) {
	a, source := newAppMediaFixture(t)
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	a.media.lyrics = appLyricsSource{
		load: func(_ context.Context, s platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
			return platform.MediaLyricsFile{Scope: s, Status: "missing"}, nil
		},
		choose: func(ctx context.Context, s platform.MediaLyricsScope, _ func([]byte) error) (platform.MediaLyricsFile, error) {
			close(entered)
			<-ctx.Done()
			close(cancelled)
			<-release
			return platform.MediaLyricsFile{Scope: s, Status: "ready"}, nil
		},
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	emit(appMediaSample())
	st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" && s.Lyrics.Status == "missing" })
	done := make(chan error, 1)
	go func() { done <- a.ImportMediaLyrics(id, st.Revision) }()
	mediaReceive(t, entered)
	a.hideDock(1)
	mediaReceive(t, source.stopped)
	select {
	case <-cancelled:
		t.Fatal("hover hide cancelled accepted import")
	default:
	}
	a.settingsMu.Lock()
	a.settings.Dock.Media.Enabled = false
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	mediaReceive(t, cancelled)
	a.settingsMu.Lock()
	a.settings.Dock.Media.Enabled = true
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	a.showDock(mediaDockFixture(2), true)
	next := a.GetDockState().Media.Session
	emit = mediaReceive(t, source.started)
	emit(appMediaSample())
	current := waitAppMedia(t, a, next, func(s *MediaViewState) bool { return s.Sample.Status == "ready" && s.Lyrics.Status == "missing" })
	if err := a.ImportMediaLyrics(next, current.Revision); !errors.Is(err, media.ErrBusy) {
		t.Fatalf("undrained chooser slot reused: %v", err)
	}
	close(release)
	if err := mediaReceive(t, done); !errors.Is(err, media.ErrRetired) {
		t.Fatalf("disabled import result accepted: %v", err)
	}
	a.viewMu.Lock()
	occupied := a.media.importJob != nil
	a.viewMu.Unlock()
	if occupied {
		t.Fatal("drained chooser retained slot")
	}
}

type (
	reviewProviderStart struct {
		provider platform.MediaProvider
		emit     func(platform.MediaSample)
	}
	reviewProviderSource struct {
		appMediaSource
		starts chan reviewProviderStart
		stops  chan platform.MediaProvider
	}
)

func (s *reviewProviderSource) ObserveMedia(ctx context.Context, p platform.MediaProvider, emit func(platform.MediaSample)) error {
	s.starts <- reviewProviderStart{p, emit}
	<-ctx.Done()
	s.stops <- p
	return ctx.Err()
}

func TestAppMediaReviewPinsKeepProvidersIndependent(t *testing.T) {
	for _, disable := range []bool{false, true} {
		name := "close"
		if disable {
			name = "disable"
		}
		t.Run(name, func(t *testing.T) {
			settings := mediaSettings()
			settings.Dock.Media.SpotifyEnabled = true
			p := fake.New()
			p.ScreenList = []domain.Screen{{ID: 1, Main: true, Visible: domain.Bounds{W: 1440, H: 900}}}
			a := newApp(p, settings, "")
			source := &reviewProviderSource{starts: make(chan reviewProviderStart, 4), stops: make(chan platform.MediaProvider, 4)}
			a.wireMedia(source, nil)
			a.startMedia()
			t.Cleanup(a.stopCapture)
			a.mediaPinFactory = func(uint64, platform.MediaProvider, func(platform.MediaPanelEvent)) *dockWindow {
				d, _, _, _ := dockHarness()
				return d
			}
			pins := map[platform.MediaProvider]uint64{}
			for i, provider := range []platform.MediaProvider{platform.MediaMusic, platform.MediaSpotify} {
				dockID := uint64(i + 1)
				st := mediaDockFixture(dockID)
				if provider == platform.MediaSpotify {
					st.Item.BundleID = "com.spotify.client"
				}
				a.showDock(st, true)
				id := a.GetDockState().Media.Session
				started := mediaReceive(t, source.starts)
				if started.provider != provider {
					t.Fatal("wrong provider observed")
				}
				sample := appMediaSample()
				sample.Provider = provider
				started.emit(sample)
				current := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
				pin, err := a.PinMediaPanel(id, current.Revision)
				if err != nil {
					t.Fatal(err)
				}
				pins[provider] = pin
				a.hideDock(dockID)
			}
			if pins[platform.MediaMusic] == pins[platform.MediaSpotify] {
				t.Fatal("providers shared pin identity")
			}
			if disable {
				a.settingsMu.Lock()
				a.settings.Dock.Media.MusicEnabled = false
				a.settingsMu.Unlock()
				a.configureDock(a.settingsSnapshot())
			} else {
				st := a.GetMediaState(pins[platform.MediaMusic])
				if err := a.CloseMediaPanel(st.Session, st.Revision); err != nil {
					t.Fatal(err)
				}
			}
			if got := mediaReceive(t, source.stops); got != platform.MediaMusic {
				t.Fatalf("wrong provider stopped: %s", got)
			}
			other := a.GetMediaState(pins[platform.MediaSpotify])
			if other == nil || !other.Open || other.Provider != platform.MediaSpotify {
				t.Fatal("Music retirement closed Spotify pin")
			}
			select {
			case p := <-source.stops:
				t.Fatalf("unrelated provider stopped: %s", p)
			case <-source.starts:
				t.Fatal("pin started duplicate observer")
			default:
			}
		})
	}
}

func TestAppMediaReviewArtworkOffOnRejectsOldCompletion(t *testing.T) {
	a, source := newAppMediaFixture(t)
	a.settingsMu.Lock()
	a.settings.Dock.Media.SpotifyEnabled = true
	a.settings.Dock.Media.RemoteArtwork = true
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	dockState := mediaDockFixture(1)
	dockState.Item.BundleID = "com.spotify.client"
	a.showDock(dockState, true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	sample := appMediaSample()
	sample.Provider = platform.MediaSpotify
	emit(sample)
	waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" })
	var old, fresh bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if err := png.Encode(&old, img); err != nil {
		t.Fatal(err)
	}
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&fresh, img); err != nil {
		t.Fatal(err)
	}
	art := &reviewArtworkSource{entered: make(chan struct{}), release: make(chan struct{}), png: old.Bytes(), fresh: fresh.Bytes()}
	t.Cleanup(func() {
		select {
		case <-art.release:
		default:
			close(art.release)
		}
	})
	a.viewMu.Lock()
	asset := a.media.assets[platform.MediaSpotify]
	asset.token = "toggle-token"
	a.media.artwork = media.NewArtworkCache(art)
	ctx, cancel := context.WithCancel(asset.ctx)
	asset.artworkCancel = cancel
	asset.artworkRevision++
	revision := asset.artworkRevision
	a.viewMu.Unlock()
	done := make(chan struct{})
	go func() { defer close(done); a.readMediaArtwork(asset, ctx, revision, true) }()
	mediaReceive(t, art.entered)
	for _, enabled := range []bool{false, true} {
		a.settingsMu.Lock()
		a.settings.Dock.Media.RemoteArtwork = enabled
		a.settingsMu.Unlock()
		a.configureDock(a.settingsSnapshot())
	}
	current := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Artwork.Status == "ready" })
	expected := "data:image/png;base64," + base64.StdEncoding.EncodeToString(fresh.Bytes())
	if current.Artwork.Image != expected {
		t.Fatal("new policy read did not publish fresh image")
	}
	close(art.release)
	mediaReceive(t, done)
	if got := a.GetMediaState(id); got.Artwork.Status != "ready" || got.Artwork.Image != expected {
		t.Fatal("old off/on completion overwrote current artwork")
	}
}
