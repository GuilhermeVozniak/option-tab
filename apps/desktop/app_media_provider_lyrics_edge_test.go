package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"option-tab/internal/platform"
)

func TestAppMediaProviderLyricsNeverReplacesLocalAssociation(t *testing.T) {
	for _, scenario := range []string{"ready", "broken", "invalid", "mismatched", "missing with document"} {
		t.Run(scenario, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			var providerReads atomic.Int32
			a.media.lyrics = appLyricsSource{load: func(_ context.Context, scope platform.MediaLyricsScope) (platform.MediaLyricsFile, error) {
				document := platform.MediaLyricsFile{Scope: scope, Status: "ready", DocumentID: "chosen-local", Data: []byte("[00:00]Chosen local line")}
				switch scenario {
				case "broken":
					return document, errors.New("chosen file is unreadable")
				case "invalid":
					document.Data = []byte("Chosen text without timestamps")
				case "mismatched":
					document.Scope.TrackID = "other-track"
				case "missing with document":
					document.Status, document.Data = "missing", nil
				}
				return document, nil
			}}
			a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, _ func() error) (platform.MediaProviderLyrics, error) {
				providerReads.Add(1)
				return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00]Unrequested provider line")}, nil
			}}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			mediaReceive(t, source.started)(appMediaSample())
			wantStatus := "unavailable"
			if scenario == "ready" {
				wantStatus = "ready"
			}
			state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Status == wantStatus && s.Sample.Status == "ready" })
			if state.Lyrics.Source != "local" || providerReads.Load() != 0 || source.permissionCalls.Load() != 0 {
				t.Fatalf("local association bypassed: source=%q providerReads=%d prompts=%d", state.Lyrics.Source, providerReads.Load(), source.permissionCalls.Load())
			}
			if scenario == "ready" {
				if len(state.Lyrics.Cues) != 1 || state.Lyrics.Cues[0].Text != "Chosen local line" || state.Lyrics.DocumentID != "chosen-local" {
					t.Fatalf("chosen local document changed: %+v", state.Lyrics)
				}
			} else if len(state.Lyrics.Cues) != 0 || state.ActiveCue != -1 {
				t.Fatalf("unusable local association created cues: %+v", state.Lyrics)
			}
		})
	}
}

func TestAppMediaProviderLyricsRejectsUnusableRepliesWithoutLeakingText(t *testing.T) {
	for _, scenario := range []string{"plain text", "empty ready", "missing", "error", "process mismatch", "epoch mismatch", "oversize"} {
		t.Run(scenario, func(t *testing.T) {
			a, source := newAppMediaFixture(t)
			a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
				if err := guard(); err != nil {
					return platform.MediaProviderLyrics{}, err
				}
				result := platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00]Private provider text"), Reason: "Private provider detail"}
				switch scenario {
				case "plain text":
					result.Data = []byte("Private provider text without timestamps")
				case "empty ready":
					result.Data = nil
				case "missing":
					result.Status = "missing"
				case "error":
					return result, errors.New("Private provider error text")
				case "process mismatch":
					result.Scope.Process.LaunchID = "replaced-player"
				case "epoch mismatch":
					result.Scope.TrackEpoch++
				case "oversize":
					result.Data = []byte(strings.Repeat("x", (1<<20)+1))
				}
				return result, nil
			}}
			a.showDock(mediaDockFixture(1), true)
			id := a.GetDockState().Media.Session
			mediaReceive(t, source.started)(appMediaSample())
			wantStatus, wantReason := "unavailable", "Music lyrics are unavailable"
			switch scenario {
			case "plain text", "empty ready", "oversize":
				wantReason = "Music does not supply synchronized lyrics for this track"
			case "missing":
				wantStatus, wantReason = "missing", "Music has no lyrics for this track"
			}
			state := waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Lyrics.Source == "music" && s.Lyrics.Status == wantStatus })
			if state.Lyrics.Reason != wantReason || len(state.Lyrics.Cues) != 0 || state.ActiveCue != -1 || state.Lyrics.DocumentID != "" || state.Lyrics.OffsetMS != 0 {
				t.Fatalf("unusable provider reply escaped: %+v activeCue=%d", state.Lyrics, state.ActiveCue)
			}
			if source.permissionCalls.Load() != 0 {
				t.Fatal("lyric read requested Automation consent")
			}
		})
	}
}

func TestAppMediaProviderLyricsDiscardsReplyAfterOwnerRetirement(t *testing.T) {
	for _, scenario := range []string{"track change", "disable", "last subscriber"} {
		t.Run(scenario, func(t *testing.T) {
			// Wait for actual App goroutines to become idle after the delayed reply,
			// rather than asserting before publication or sleeping for a guessed delay.
			synctest.Test(t, func(t *testing.T) {
				a, source := newAppMediaFixture(t)
				entered := make(chan func() error, 1)
				release := make(chan struct{})
				var once sync.Once
				t.Cleanup(func() { once.Do(func() { close(release) }) })
				var reads atomic.Int32
				a.media.source = appProviderLyricsSource{MediaProviderSource: source, read: func(_ context.Context, scope platform.MediaScope, guard func() error) (platform.MediaProviderLyrics, error) {
					if reads.Add(1) == 1 {
						entered <- guard
						<-release
						// Deliberately succeed despite cancellation to exercise the App's
						// result admission, independently of the platform final guard.
						return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00]Obsolete provider line")}, nil
					}
					return platform.MediaProviderLyrics{Scope: scope, Status: "ready", Data: []byte("[00:00]Current provider line")}, nil
				}}
				a.showDock(mediaDockFixture(1), true)
				id := a.GetDockState().Media.Session
				emit := mediaReceive(t, source.started)
				emit(appMediaSample())
				guard := mediaReceive(t, entered)
				a.viewMu.Lock()
				oldAsset := a.media.assets[platform.MediaMusic]
				a.viewMu.Unlock()
				switch scenario {
				case "track change":
					sample := appMediaSample()
					sample.Sequence, sample.TrackEpoch, sample.Track.ID = 2, 2, "second"
					emit(sample)
					waitAppMedia(t, a, id, func(s *MediaViewState) bool { return s.Scope.TrackID == "second" && s.Lyrics.Status == "ready" })
				case "disable":
					a.settingsMu.Lock()
					a.settings.Dock.Media.Enabled = false
					a.settingsMu.Unlock()
					a.viewMu.Lock()
					a.syncMediaLocked()
					a.viewMu.Unlock()
				case "last subscriber":
					a.hideDock(1)
				}
				if err := guard(); err == nil {
					t.Fatal("retired asset retained provider getter admission")
				}
				once.Do(func() { close(release) })
				synctest.Wait()
				a.viewMu.Lock()
				oldCues := len(oldAsset.lyrics.Cues)
				_, oldActive := oldAsset.timeline.PositionAndActive()
				a.viewMu.Unlock()
				if oldCues != 0 || oldActive != -1 {
					t.Fatalf("retired reply changed old timeline: cues=%d active=%d", oldCues, oldActive)
				}
				state := a.GetMediaState(id)
				if scenario == "track change" {
					if state == nil || state.Scope.TrackID != "second" || len(state.Lyrics.Cues) != 1 || state.Lyrics.Cues[0].Text != "Current provider line" {
						t.Fatalf("old reply changed current track: %+v", state)
					}
				} else if state != nil {
					t.Fatal("old reply restored retired presentation")
				}
				if source.permissionCalls.Load() != 0 {
					t.Fatal("retiring lyric read requested consent")
				}
			})
		})
	}
}
