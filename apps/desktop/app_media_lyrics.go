package main

import (
	"context"
	"errors"

	"option-tab/internal/media"
	"option-tab/internal/platform"
)

type mediaLyricsChange struct {
	scope platform.MediaLyricsScope
}

func localMediaLyricsScope(scope platform.MediaScope) platform.MediaLyricsScope {
	return platform.MediaLyricsScope{Provider: scope.Provider, TrackID: scope.TrackID}
}

func (a *App) mediaLyricsBusyLocked(scope platform.MediaLyricsScope) bool {
	if job := a.media.importJob; job != nil && job.scope == scope {
		return true
	}
	change := a.media.lyricsChanges[scope.Provider]
	return change != nil && change.scope == scope
}

func suspendMediaLyricsRead(asset *mediaTrackAssets) {
	if asset.lyricsCancel != nil {
		asset.lyricsCancel()
	}
	asset.lyricsRevision++
	if asset.lyrics.Status == "loading" {
		asset.lyrics = MediaLyricsView{Status: "missing", Cues: []media.Cue{}}
	}
}

func (a *App) resumeMediaLyricsLocked(scope platform.MediaLyricsScope, reload bool) {
	asset := a.media.assets[scope.Provider]
	if asset != nil && localMediaLyricsScope(asset.scope) == scope && a.mediaAssetsCurrentLocked(asset) &&
		(reload || asset.lyrics.Status == "missing" || asset.lyrics.Status == "loading") {
		a.queueMediaLyricsLocked(asset)
		a.publishMediaAssetsLocked(asset)
	}
}

func (a *App) mediaLyricsAvailableLocked(provider platform.MediaProvider) bool {
	_, supplied := a.media.source.(platform.MediaProviderLyricsSource)
	return a.media.lyrics != nil || (provider == platform.MediaMusic && supplied)
}

func (a *App) queueMediaLyricsLocked(asset *mediaTrackAssets) {
	if !a.mediaLyricsAvailableLocked(asset.scope.Provider) || a.mediaLyricsBusyLocked(localMediaLyricsScope(asset.scope)) {
		return
	}
	if asset.lyricsCancel != nil {
		asset.lyricsCancel()
	}
	ctx, cancel := context.WithCancel(asset.ctx)
	asset.lyricsCancel = cancel
	asset.lyricsRevision++
	revision := asset.lyricsRevision
	asset.lyrics = MediaLyricsView{Status: "loading", Cues: []media.Cue{}}
	_ = asset.timeline.Load(media.LyricsScope{Provider: string(asset.scope.Provider), TrackID: asset.scope.TrackID}, asset.scope.TrackEpoch, nil, 0)
	local := a.media.lyrics
	provider, _ := a.media.source.(platform.MediaProviderLyricsSource)
	current := func() bool {
		return ctx.Err() == nil && a.mediaAssetsCurrentLocked(asset) && asset.lyricsRevision == revision &&
			!a.mediaLyricsBusyLocked(localMediaLyricsScope(asset.scope))
	}
	guard := func() error {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !current() {
			return media.ErrRetired
		}
		return nil
	}
	go func() {
		defer cancel()
		result := readMediaLyrics(ctx, asset.scope, local, provider, guard)
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !current() {
			return
		}
		if result.Status == "ready" {
			if err := asset.timeline.Load(media.LyricsScope{Provider: string(asset.scope.Provider), TrackID: asset.scope.TrackID}, asset.scope.TrackEpoch, result.Cues, result.OffsetMS); err != nil {
				result.Status, result.Reason, result.Cues = "unavailable", err.Error(), nil
			}
		}
		asset.lyrics = result
		a.publishMediaAssetsLocked(asset)
	}()
}

func readMediaLyrics(ctx context.Context, scope platform.MediaScope, local platform.MediaLyricsSource, provider platform.MediaProviderLyricsSource, guard func() error) MediaLyricsView {
	if local != nil {
		expected := platform.MediaLyricsScope{Provider: scope.Provider, TrackID: scope.TrackID}
		document, err := local.LoadMediaLyrics(ctx, expected)
		result := MediaLyricsView{Source: "local", DocumentID: document.DocumentID, Status: document.Status, Reason: document.Reason, OffsetMS: document.OffsetMS}
		if err == nil && (document.Scope != expected || (document.Status == "missing" && (document.DocumentID != "" || len(document.Data) != 0))) {
			err = errors.New("lyric document does not match the current track")
			result.DocumentID = ""
		}
		if err == nil && document.Status == "ready" {
			result.Cues, err = media.ParseLRC(document.Data)
		}
		if err != nil {
			result.Status, result.Reason, result.Cues = "unavailable", err.Error(), nil
			return result
		}
		if document.Status != "missing" {
			return result
		}
	}
	result := MediaLyricsView{Status: "missing", Cues: []media.Cue{}}
	if provider == nil || scope.Provider != platform.MediaMusic {
		return result
	}
	result.Source = "music"
	if err := guard(); err != nil {
		return MediaLyricsView{Source: "music", Status: "unavailable", Reason: "Music lyrics are unavailable"}
	}
	supplied, err := provider.ReadMediaProviderLyricsGuarded(ctx, scope, guard)
	if err != nil || supplied.Scope != scope {
		result.Status, result.Reason = "unavailable", "Music lyrics are unavailable"
		return result
	}
	switch supplied.Status {
	case "ready":
		result.Cues, err = media.ParseLRC(supplied.Data)
		if err != nil {
			result.Status, result.Reason, result.Cues = "unavailable", "Music does not supply synchronized lyrics for this track", nil
		} else {
			result.Status = "ready"
		}
	case "missing":
		result.Reason = "Music has no lyrics for this track"
	case "unsupported":
		result.Status, result.Reason = "unavailable", "Provider lyrics are unavailable"
	default:
		result.Status, result.Reason = "unavailable", "Music lyrics are unavailable"
	}
	return result
}
