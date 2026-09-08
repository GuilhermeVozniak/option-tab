package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"option-tab/internal/platform"
)

type appLyricsSource struct {
	platform.MediaLyricsSource
	load   func(context.Context, platform.MediaLyricsScope) (platform.MediaLyricsFile, error)
	choose func(context.Context, platform.MediaLyricsScope, func([]byte) error) (platform.MediaLyricsFile, error)
	remove func(context.Context, platform.MediaLyricsScope) error
	offset func(context.Context, platform.MediaLyricsScope, int64) error
}

func TestAppMediaMissingAndInvalidLyricsKeepCueArrayInJSON(t *testing.T) {
	for _, kind := range []string{"missing", "unreadable", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			a.media.lyrics = appLyricsSource{load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
				switch kind {
				case "unreadable":
					return platform.MediaLyricsFile{Scope: scope, Status: "unavailable"}, errors.New("lyric file unavailable")
				case "invalid":
					return platform.MediaLyricsFile{Scope: scope, Status: "ready", Data: []byte("no timestamps")}, nil
				default:
					return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
				}
			}}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			emit := mediaReceive(t, source.started)
			emit(appMediaSample())
			wantStatus := "unavailable"
			if kind == "missing" {
				wantStatus = "missing"
			}
			st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == wantStatus && s.Sample.Status == "ready" })
			data, err := json.Marshal(st)
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Lyrics struct {
					Cues json.RawMessage `json:"cues"`
				} `json:"lyrics"`
			}
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			if string(wire.Lyrics.Cues) != "[]" {
				t.Fatalf("missing lyrics must send an empty cue array to the renderer, got %s", wire.Lyrics.Cues)
			}
		})
	}
}

func TestAppMediaLyricsChooserCancelKeepsExistingDocumentWithoutError(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		name := "native chooser cancel"
		if explicit {
			name = "Cancel import button"
		}
		t.Run(name, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			entered := make(chan struct{})
			a.media.lyrics = appLyricsSource{
				load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
					return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "existing", Data: []byte("[00:00.00]Existing lyric")}, nil
				},
				choose: func(ctx context.Context, _ platform.MediaLyricsScope, _ func([]byte) error) (platform.MediaLyricsFile, error) {
					close(entered)
					if explicit {
						<-ctx.Done()
					}
					return platform.MediaLyricsFile{}, context.Canceled
				},
			}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			emit := mediaReceive(t, source.started)
			emit(appMediaSample())
			st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
			done := make(chan error, 1)
			go func() { done <- a.ImportMediaLyrics(id, st.Revision) }()
			mediaReceive(t, entered)
			if explicit {
				if err := a.CancelMediaLyricsImport(id, st.Revision); err != nil {
					t.Fatal(err)
				}
			}
			if err := mediaReceive(t, done); err != nil {
				t.Fatalf("intentional cancellation was reported as a failure: %v", err)
			}
			got := a.GetMediaState(id)
			if got.Error != "" || got.Lyrics.DocumentID != "existing" || len(got.Lyrics.Cues) != 1 || got.Lyrics.Cues[0].Text != "Existing lyric" {
				t.Fatalf("cancellation changed the existing document or reported an error: %+v", got.Lyrics)
			}
		})
	}
}

func TestAppMediaSuccessfulLyricReplacementClearsPriorImportError(t *testing.T) {
	a, source := newAppMediaFixture(t)
	var imported atomic.Bool
	a.media.lyrics = appLyricsSource{
		load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
			if imported.Load() {
				return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "valid", Data: []byte("[00:00.00]Valid lyric")}, nil
			}
			return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
		},
		choose: func(_ context.Context, scope platform.MediaLyricsScope, _ func([]byte) error) (platform.MediaLyricsFile, error) {
			if !imported.Swap(true) {
				return platform.MediaLyricsFile{}, errors.New("LRC file has no usable timestamps")
			}
			return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "valid"}, nil
		},
	}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	emit := mediaReceive(t, source.started)
	emit(appMediaSample())
	st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Sample.Status == "ready" && s.Lyrics.Status == "missing" })
	if err := a.ImportMediaLyrics(id, st.Revision); err == nil {
		t.Fatal("invalid import was accepted")
	}
	st = a.GetMediaState(id)
	if st.Error == "" {
		t.Fatal("invalid import error was lost")
	}
	if err := a.ImportMediaLyrics(id, st.Revision); err != nil {
		t.Fatal(err)
	}
	st = waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
	if st.Error != "" {
		t.Fatalf("successful replacement retained the previous import error: %s", st.Error)
	}
}

func TestAppMediaSuccessfulLyricChangesClearPriorImportError(t *testing.T) {
	for _, kind := range []string{"reload", "remove", "offset"} {
		t.Run(kind, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			var removed atomic.Bool
			var offset atomic.Int64
			a.media.lyrics = appLyricsSource{
				load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
					if removed.Load() {
						return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
					}
					return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "existing", OffsetMS: offset.Load(), Data: []byte("[00:00.00]Existing lyric")}, nil
				},
				choose: func(context.Context, platform.MediaLyricsScope, func([]byte) error) (platform.MediaLyricsFile, error) {
					return platform.MediaLyricsFile{}, errors.New("LRC file has no usable timestamps")
				},
				remove: func(context.Context, platform.MediaLyricsScope) error { removed.Store(true); return nil },
				offset: func(_ context.Context, _ platform.MediaLyricsScope, value int64) error {
					offset.Store(value)
					return nil
				},
			}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			emit := mediaReceive(t, source.started)
			emit(appMediaSample())
			st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
			if err := a.ImportMediaLyrics(id, st.Revision); err == nil {
				t.Fatal("invalid replacement was accepted")
			}
			st = a.GetMediaState(id)
			if st.Error == "" {
				t.Fatal("invalid replacement error was lost")
			}
			var err error
			wantStatus := "ready"
			switch kind {
			case "remove":
				err = a.RemoveMediaLyrics(id, st.Revision)
				wantStatus = "missing"
			case "offset":
				err = a.SetMediaLyricsOffset(id, st.Revision, 500)
			default:
				err = a.ReloadMediaLyrics(id, st.Revision)
			}
			if err != nil {
				t.Fatal(err)
			}
			st = waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == wantStatus })
			if st.Error != "" {
				t.Fatalf("successful lyric change retained the old replacement failure: %s", st.Error)
			}
			if kind == "offset" && st.Lyrics.OffsetMS != 500 {
				t.Fatal("successful offset was not reflected in the document")
			}
		})
	}
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

func (s appLyricsSource) RemoveMediaLyrics(ctx context.Context, scope platform.MediaLyricsScope) error {
	return s.remove(ctx, scope)
}

func (s appLyricsSource) SetMediaLyricsOffset(ctx context.Context, scope platform.MediaLyricsScope, offset int64) error {
	return s.offset(ctx, scope, offset)
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
