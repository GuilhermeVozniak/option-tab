package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"option-tab/internal/platform"
)

type appLyricsSource struct {
	platform.MediaLyricsSource
	load   func(context.Context, platform.MediaLyricsScope) (platform.MediaLyricsFile, error)
	choose func(context.Context, platform.MediaLyricsScope, func([]byte) error) (platform.MediaLyricsFile, error)
}

func TestAppMediaRemoteArtworkPolicyPreservesLocalLyrics(t *testing.T) {
	a, source := newAppMediaFixture(t)
	a.settingsMu.Lock()
	a.settings.Dock.Media.SpotifyEnabled = true
	a.settings.Dock.Media.RemoteArtwork = true
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	var loads atomic.Int32
	a.media.lyrics = appLyricsSource{load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
		loads.Add(1)
		return platform.MediaLyricsFile{Scope: scope, DocumentID: "same-local-document", Status: "ready", Data: []byte("[00:00.00]Original local line")}, nil
	}}
	st := mediaDockFixture(1)
	st.Item.BundleID = "com.spotify.client"
	a.showDock(st, true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	sample := appMediaSample()
	sample.Provider = platform.MediaSpotify
	emit(sample)
	waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
	a.settingsMu.Lock()
	a.settings.Dock.Media.RemoteArtwork = false
	a.settingsMu.Unlock()
	a.configureDock(a.settingsSnapshot())
	got := a.GetMediaState(id)
	if got == nil || got.Lyrics.DocumentID != "same-local-document" || loads.Load() != 1 || got.Artwork.Status != "networkDisabled" {
		t.Fatalf("artwork policy discarded or reloaded local lyrics: %+v loads=%d", got, loads.Load())
	}
}

func (s appLyricsSource) LoadMediaLyrics(ctx context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
	return s.load(ctx, scope)
}

func (s appLyricsSource) ChooseMediaLyrics(ctx context.Context, scope platform.MediaLyricsScope, validate func([]byte) error) (platform.MediaLyricsFile, error) {
	return s.choose(ctx, scope, validate)
}

func TestAppMediaImportKeepsOriginalTrackAcrossReplacement(t *testing.T) {
	a, source := newAppMediaFixture(t)
	entered := make(chan platform.MediaLyricsScope, 1)
	release := make(chan struct{})
	var mu sync.Mutex
	imported := false
	a.media.lyrics = appLyricsSource{
		load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
			mu.Lock()
			ready := imported && scope.TrackID == "first"
			mu.Unlock()
			if ready {
				return platform.MediaLyricsFile{DocumentID: "original-doc", Scope: scope, Status: "ready", Data: []byte("[00:00.00]Original fixture line\n[00:02.00]Another fixture line")}, nil
			}
			return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
		},
		choose: func(_ context.Context, scope platform.MediaLyricsScope, validate func([]byte) error) (platform.MediaLyricsFile, error) {
			entered <- scope
			<-release
			if err := validate([]byte("[00:00.00]Original fixture line")); err != nil {
				return platform.MediaLyricsFile{}, err
			}
			mu.Lock()
			imported = true
			mu.Unlock()
			return platform.MediaLyricsFile{DocumentID: "original-doc", Scope: scope, Status: "ready"}, nil
		},
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	emit(appMediaSample())
	st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" && s.Lyrics.Status == "missing" })
	done := make(chan error, 1)
	go func() { done <- a.ImportMediaLyrics(id, st.Revision) }()
	if scope := mediaReceive(t, entered); scope.TrackID != "first" || scope.Provider != platform.MediaMusic {
		t.Fatalf("wrong original lyric scope: %+v", scope)
	}
	next := appMediaSample()
	next.Sequence = 2
	next.TrackEpoch = 2
	next.Track.ID = "second"
	emit(next)
	waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Scope.TrackID == "second" && s.Lyrics.Status == "missing" })
	close(release)
	if err := mediaReceive(t, done); err != nil {
		t.Fatal(err)
	}
	if got := a.GetMediaState(id); got.Scope.TrackID != "second" || len(got.Lyrics.Cues) != 0 {
		t.Fatal("old import was remapped onto replacement track")
	}
	next.Sequence = 3
	next.TrackEpoch = 3
	next.Track.ID = "first"
	emit(next)
	loaded := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Scope.TrackID == "first" && s.Lyrics.Status == "ready" })
	if loaded.Lyrics.DocumentID != "original-doc" || loaded.Lyrics.Cues[0].Text != "Original fixture line" {
		t.Fatal("return to associated track did not reload original document")
	}
	loaded.Lyrics.Cues[0].Text = "caller mutation"
	if a.GetMediaState(id).Lyrics.Cues[0].Text != "Original fixture line" {
		t.Fatal("lyric DTO aliases private timeline data")
	}
}

func TestAppMediaLateLyricReadCannotReplaceCurrentTrack(t *testing.T) {
	a, source := newAppMediaFixture(t)
	entered, release, returned := make(chan struct{}), make(chan struct{}), make(chan struct{})
	a.media.lyrics = appLyricsSource{load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
		if scope.TrackID == "first" {
			close(entered)
			<-release
			defer close(returned)
			return platform.MediaLyricsFile{Scope: scope, DocumentID: "old", Status: "ready", Data: []byte("[00:00.00]Retired original line")}, nil
		}
		return platform.MediaLyricsFile{Scope: scope, DocumentID: "current", Status: "ready", Data: []byte("[00:00.00]Current original line")}, nil
	}}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	emit(appMediaSample())
	mediaReceive(t, entered)
	next := appMediaSample()
	next.Sequence = 2
	next.TrackEpoch = 2
	next.Track.ID = "second"
	emit(next)
	waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.DocumentID == "current" })
	close(release)
	mediaReceive(t, returned)
	// Capture a fresh sample as a barrier after the late source read. No delayed
	// old completion has authority over this provider's replacement assets.
	next.Sequence = 3
	emit(next)
	got := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Sequence == 3 })
	if got.Lyrics.DocumentID != "current" || got.Lyrics.Cues[0].Text != "Current original line" {
		t.Fatal("retired lyric read contaminated current track")
	}
}
