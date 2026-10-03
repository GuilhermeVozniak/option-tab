package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"option-tab/internal/media"
	"option-tab/internal/platform"
)

type appProviderLyricsSource struct {
	platform.MediaProviderSource
	read func(context.Context, platform.MediaScope, func() error) (platform.MediaProviderLyrics, error)
}

func TestAppMediaProviderLyricsYieldToImportUntilChooserDrains(t *testing.T) {
	for _, cancelImport := range []bool{false, true} {
		t.Run(map[bool]string{false: "local override", true: "cancel resumes provider"}[cancelImport], func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			providerEntered := make(chan func() error, 1)
			providerRelease, chooserRelease := make(chan struct{}), make(chan struct{})
			chooserEntered, chooserCancelled := make(chan struct{}), make(chan struct{})
			var providerOnce, chooserOnce sync.Once
			t.Cleanup(func() {
				providerOnce.Do(func() { close(providerRelease) })
				chooserOnce.Do(func() { close(chooserRelease) })
			})
			var imported atomic.Bool
			var reads atomic.Int32
			a.media.lyrics = appLyricsSource{
				load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
					if imported.Load() {
						return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "chosen", Data: []byte("[00:00.00]Local override")}, nil
					}
					return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
				},
				choose: func(ctx context.Context, scope platform.MediaLyricsScope, _ func([]byte) error) (platform.MediaLyricsFile, error) {
					close(chooserEntered)
					if cancelImport {
						<-ctx.Done()
						close(chooserCancelled)
					}
					<-chooserRelease
					if cancelImport {
						return platform.MediaLyricsFile{}, context.Canceled
					}
					imported.Store(true)
					return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "chosen"}, nil
				},
			}
			a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
				if reads.Add(1) == 1 {
					providerEntered <- guard
					<-providerRelease
					// Exercise App publication retirement even if an in-flight reply succeeds.
					return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00.00]Obsolete provider")}, nil
				}
				if err := guard(); err != nil {
					return platform.MediaProviderLyrics{}, err
				}
				return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00.00]Fresh provider")}, nil
			}}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			mediaReceive(t, source.started)(appMediaSample())
			guard := mediaReceive(t, providerEntered)
			st := a.GetMediaState(id)
			done := make(chan error, 1)
			go func() { done <- a.ImportMediaLyrics(id, st.Revision) }()
			mediaReceive(t, chooserEntered)
			if err := guard(); err == nil {
				t.Fatal("accepted import left old provider admission active")
			}
			st = a.GetMediaState(id)
			if err := a.ReloadMediaLyrics(id, st.Revision); !errors.Is(err, media.ErrBusy) {
				t.Fatalf("reload during import: %v", err)
			}
			if cancelImport {
				if err := a.CancelMediaLyricsImport(id, st.Revision); err != nil {
					t.Fatal(err)
				}
				mediaReceive(t, chooserCancelled)
				if err := a.ReloadMediaLyrics(id, st.Revision); !errors.Is(err, media.ErrBusy) {
					t.Fatalf("reload before cancelled chooser drained: %v", err)
				}
			}
			chooserOnce.Do(func() { close(chooserRelease) })
			if err := mediaReceive(t, done); err != nil {
				t.Fatal(err)
			}
			wantSource, wantText := "local", "Local override"
			if cancelImport {
				wantSource, wantText = "music", "Fresh provider"
			}
			st = waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
			if st.Lyrics.Source != wantSource || len(st.Lyrics.Cues) != 1 || st.Lyrics.Cues[0].Text != wantText {
				t.Fatalf("wrong source after chooser drain: %+v", st.Lyrics)
			}
			providerOnce.Do(func() { close(providerRelease) })
			if err := guard(); err == nil {
				t.Fatal("old provider read revived after import")
			}
		})
	}
}

func TestAppMediaProviderLyricsLocalMutationSerializesReloadAndRejectsProviderMutation(t *testing.T) {
	a, source := newAppMediaFixture(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var removed atomic.Bool
	var removals, offsets, imports atomic.Int32
	a.media.lyrics = appLyricsSource{
		load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
			if removed.Load() {
				return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
			}
			return platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "local", Data: []byte("[00:00.00]Local")}, nil
		},
		remove: func(context.Context, platform.MediaLyricsScope) error {
			removals.Add(1)
			close(entered)
			<-release
			removed.Store(true)
			return nil
		},
		offset: func(context.Context, platform.MediaLyricsScope, int64) error { offsets.Add(1); return nil },
		choose: func(context.Context, platform.MediaLyricsScope, func([]byte) error) (platform.MediaLyricsFile, error) {
			imports.Add(1)
			return platform.MediaLyricsFile{}, errors.New("unexpected chooser")
		},
	}
	a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
		if err := guard(); err != nil {
			return platform.MediaProviderLyrics{}, err
		}
		return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00.00]Provider")}, nil
	}}
	a.showDock(mediaDockFixture(1), true)
	id := a.GetDockState().Media.Session
	mediaReceive(t, source.started)(appMediaSample())
	st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
	done := make(chan error, 1)
	go func() { done <- a.RemoveMediaLyrics(id, st.Revision) }()
	mediaReceive(t, entered)
	if err := a.ReloadMediaLyrics(id, st.Revision); !errors.Is(err, media.ErrBusy) {
		t.Fatalf("reload raced removal: %v", err)
	}
	if err := a.SetMediaLyricsOffset(id, st.Revision, 100); !errors.Is(err, media.ErrBusy) {
		t.Fatalf("offset raced removal: %v", err)
	}
	if err := a.ImportMediaLyrics(id, st.Revision); !errors.Is(err, media.ErrBusy) {
		t.Fatalf("import raced removal: %v", err)
	}
	once.Do(func() { close(release) })
	if err := mediaReceive(t, done); err != nil {
		t.Fatal(err)
	}
	st = waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Source == "music" && s.Lyrics.Status == "ready" })
	if err := a.RemoveMediaLyrics(id, st.Revision); err == nil {
		t.Fatal("provider data admitted local removal")
	}
	if err := a.SetMediaLyricsOffset(id, st.Revision, 100); err == nil {
		t.Fatal("provider data admitted local offset")
	}
	if removals.Load() != 1 || offsets.Load() != 0 || imports.Load() != 0 {
		t.Fatal("unadmitted mutation reached local store")
	}
}

func (s appProviderLyricsSource) ReadMediaProviderLyricsGuarded(ctx context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
	return s.read(ctx, scope, guard)
}

func TestAppMediaProviderLyricsFeedExistingTimelineAndReload(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(map[bool]string{false: "no local adapter", true: "no local association"}[local], func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			if local {
				a.media.lyrics = appLyricsSource{load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
					return platform.MediaLyricsFile{Scope: scope, Status: "missing"}, nil
				}}
			}
			var reads atomic.Int32
			a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
				if err := guard(); err != nil {
					return platform.MediaProviderLyrics{}, err
				}
				reads.Add(1)
				return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00.00]First\n[00:02.00]Second")}, nil
			}}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			emit := mediaReceive(t, source.started)
			sample := appMediaSample()
			emit(sample)
			st := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" })
			if st.Lyrics.Source != "music" || st.Lyrics.DocumentID != "" || st.Lyrics.OffsetMS != 0 || len(st.Lyrics.Cues) != 2 || st.Lyrics.Cues[1].AtMS != 2000 || st.Lyrics.Cues[1].Text != "Second" || st.ActiveCue != 0 {
				t.Fatalf("provider cues did not reach the existing timeline: %+v", st)
			}
			sample.Sequence++
			sample.PositionMS = 2500
			emit(sample)
			st = waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.ActiveCue == 1 })
			if reads.Load() != 1 {
				t.Fatal("position update reread lyrics")
			}
			if err := a.ReloadMediaLyrics(id, st.Revision); err != nil {
				t.Fatal(err)
			}
			waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == "ready" && reads.Load() == 2 })
		})
	}
}
