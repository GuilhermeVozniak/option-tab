package main

import (
	"context"
	"encoding/base64"
	"errors"

	"option-tab/internal/media"
	"option-tab/internal/platform"
)

type mediaTrackAssets struct {
	scope           platform.MediaScope
	epoch           uint64
	token           string
	ctx             context.Context
	cancel          context.CancelFunc
	timeline        *media.Timeline
	artwork         MediaArtworkView
	artworkRevision uint64
	artworkCancel   context.CancelFunc
	lyrics          MediaLyricsView
	lyricsRevision  uint64
	lyricsCancel    context.CancelFunc
}
type mediaImportOwner struct {
	session, revision uint64
	scope             platform.MediaLyricsScope
	ctx               context.Context
	cancel            context.CancelFunc
	userCancelled     bool
}

func (a *App) mediaAssetsCurrentLocked(asset *mediaTrackAssets) bool {
	return a.media != nil && a.media.assets[asset.scope.Provider] == asset && asset.ctx.Err() == nil && a.mediaAllowedLocked(asset.scope.Provider)
}

func (a *App) ensureMediaAssetsLocked(st media.State) {
	r := a.media
	scope := mediaScope(st.Sample)
	asset := r.assets[st.Provider]
	if asset != nil && (asset.scope != scope || asset.epoch != st.Epoch || asset.token != st.Sample.ArtworkToken) {
		asset.cancel()
		delete(r.assets, st.Provider)
		asset = nil
	}
	if asset != nil {
		asset.timeline.Observe(st.Sample)
		return
	}
	ctx, cancel := context.WithCancel(r.ctx)
	asset = &mediaTrackAssets{scope: scope, epoch: st.Epoch, token: st.Sample.ArtworkToken, ctx: ctx, cancel: cancel, timeline: media.NewTimeline(nil), artwork: MediaArtworkView{Status: "missing"}, lyrics: MediaLyricsView{Status: "missing", Cues: []media.Cue{}}}
	asset.timeline.Observe(st.Sample)
	r.assets[st.Provider] = asset
	if st.Sample.Status != "ready" || scope.TrackID == "" || scope.TrackEpoch == 0 {
		return
	}
	a.queueMediaArtworkLocked(asset, a.settingsSnapshot().Dock.Media.RemoteArtwork)
	if r.lyrics != nil {
		a.queueMediaLyricsLocked(asset)
	}
}

func (a *App) publishMediaAssetsLocked(asset *mediaTrackAssets) {
	for _, p := range a.media.panels {
		if p.state.Open && p.state.Scope == asset.scope {
			a.publishMediaLocked(p)
		}
	}
}

func (a *App) queueMediaArtworkLocked(asset *mediaTrackAssets, remote bool) {
	if asset.artworkCancel != nil {
		asset.artworkCancel()
	}
	asset.artworkRevision++
	asset.artwork = MediaArtworkView{Status: "missing"}
	if asset.scope.Provider == platform.MediaSpotify && !remote {
		asset.artwork.Status = "networkDisabled"
		return
	}
	if asset.token == "" {
		return
	}
	ctx, cancel := context.WithCancel(asset.ctx)
	asset.artworkCancel = cancel
	asset.artwork.Status = "loading"
	go a.readMediaArtwork(asset, ctx, asset.artworkRevision, remote)
}

func (a *App) readMediaArtwork(asset *mediaTrackAssets, ctx context.Context, revision uint64, remote bool) {
	current := func() bool {
		return ctx.Err() == nil && a.mediaAssetsCurrentLocked(asset) && asset.artworkRevision == revision &&
			(asset.scope.Provider != platform.MediaSpotify || a.settingsSnapshot().Dock.Media.RemoteArtwork)
	}
	guard := func() error {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if !current() {
			return media.ErrRetired
		}
		return nil
	}
	result, err := a.media.artwork.Read(ctx, asset.scope, asset.token, remote, guard)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if !current() {
		return
	}
	asset.artwork = MediaArtworkView{Status: result.Status, Reason: result.Reason}
	if err != nil {
		asset.artwork.Status = "unavailable"
		asset.artwork.Reason = err.Error()
	} else if result.Status == "ready" {
		asset.artwork.Image = "data:image/png;base64," + base64.StdEncoding.EncodeToString(result.PNG)
	}
	a.publishMediaAssetsLocked(asset)
}

func (a *App) queueMediaLyricsLocked(asset *mediaTrackAssets) {
	if a.media.lyrics == nil {
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
	go func() {
		defer cancel()
		document, err := a.media.lyrics.LoadMediaLyrics(ctx, platform.MediaLyricsScope{Provider: asset.scope.Provider, TrackID: asset.scope.TrackID})
		var cues []media.Cue
		if err == nil && document.Status == "ready" {
			cues, err = media.ParseLRC(document.Data)
		}
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if ctx.Err() != nil || !a.mediaAssetsCurrentLocked(asset) || asset.lyricsRevision != revision {
			return
		}
		asset.lyrics = MediaLyricsView{DocumentID: document.DocumentID, Status: document.Status, Reason: document.Reason, Cues: cues, OffsetMS: document.OffsetMS}
		if err == nil && document.Status == "ready" {
			expected := platform.MediaLyricsScope{Provider: asset.scope.Provider, TrackID: asset.scope.TrackID}
			if document.Scope != expected {
				err = errors.New("lyric document does not match the current track")
			} else {
				err = asset.timeline.Load(media.LyricsScope{Provider: string(asset.scope.Provider), TrackID: asset.scope.TrackID}, asset.scope.TrackEpoch, cues, document.OffsetMS)
			}
		}
		if err != nil {
			asset.lyrics.Status = "unavailable"
			asset.lyrics.Reason = err.Error()
			asset.lyrics.Cues = nil
		}
		a.publishMediaAssetsLocked(asset)
	}()
}

func (a *App) ImportMediaLyrics(session, revision uint64) error {
	a.viewMu.Lock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	if a.media.lyrics == nil || p.state.Sample.Status != "ready" || p.state.Scope.TrackID == "" {
		a.viewMu.Unlock()
		return errMediaLyricsUnavailable
	}
	if a.media.importJob != nil {
		a.viewMu.Unlock()
		return media.ErrBusy
	}
	ctx, cancel := context.WithCancel(a.media.ctx)
	owner := &mediaImportOwner{session: session, revision: revision, scope: platform.MediaLyricsScope{Provider: p.state.Provider, TrackID: p.state.Scope.TrackID}, ctx: ctx, cancel: cancel}
	a.media.importJob = owner
	a.viewMu.Unlock()
	_, err = a.media.lyrics.ChooseMediaLyrics(ctx, owner.scope, func(data []byte) error { _, err := media.ParseLRC(data); return err })
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	defer cancel()
	if a.media.importJob == owner {
		a.media.importJob = nil
	}
	if owner.userCancelled {
		return nil
	}
	if ctx.Err() != nil || !a.mediaAllowedLocked(owner.scope.Provider) {
		return media.ErrRetired
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	asset := a.media.assets[owner.scope.Provider]
	if err == nil && asset != nil && asset.scope.TrackID == owner.scope.TrackID && a.mediaAssetsCurrentLocked(asset) {
		if a.mediaPresentationLocked(session) == p && p.state.Scope.TrackID == owner.scope.TrackID {
			p.state.Error = ""
		}
		a.queueMediaLyricsLocked(asset)
		a.publishMediaAssetsLocked(asset)
	}
	if err != nil && a.mediaPresentationLocked(session) == p && p.state.Scope.TrackID == owner.scope.TrackID {
		p.state.Error = err.Error()
		a.publishMediaLocked(p)
	}
	return err
}

func (a *App) CancelMediaLyricsImport(session, revision uint64) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.media == nil || a.media.importJob == nil {
		return nil
	}
	owner := a.media.importJob
	if owner.session != session || owner.revision != revision {
		return media.ErrRetired
	}
	owner.userCancelled = true
	owner.cancel()
	return nil
}

func (a *App) ReloadMediaLyrics(session, revision uint64) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		return err
	}
	asset := a.media.assets[p.state.Provider]
	if a.media.lyrics == nil || asset == nil || asset.scope != p.state.Scope || asset.scope.TrackID == "" {
		return errMediaLyricsUnavailable
	}
	p.state.Error = ""
	a.queueMediaLyricsLocked(asset)
	a.publishMediaAssetsLocked(asset)
	return nil
}

func (a *App) changeMediaLyrics(session, revision uint64, change func(context.Context, platform.MediaLyricsScope) error) error {
	a.viewMu.Lock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	asset := a.media.assets[p.state.Provider]
	if a.media.lyrics == nil || asset == nil || asset.scope != p.state.Scope || asset.scope.TrackID == "" {
		a.viewMu.Unlock()
		return errMediaLyricsUnavailable
	}
	scope := platform.MediaLyricsScope{Provider: asset.scope.Provider, TrackID: asset.scope.TrackID}
	ctx, cancel := context.WithCancel(asset.ctx)
	stop := context.AfterFunc(p.ctx, cancel)
	a.viewMu.Unlock()
	defer cancel()
	defer stop()
	err = change(ctx, scope)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if ctx.Err() != nil || !a.mediaAssetsCurrentLocked(asset) || a.mediaPresentationLocked(session) != p {
		return media.ErrRetired
	}
	if err != nil {
		p.state.Error = err.Error()
		a.publishMediaLocked(p)
	} else {
		p.state.Error = ""
		a.queueMediaLyricsLocked(asset)
		a.publishMediaAssetsLocked(asset)
	}
	return err
}

func (a *App) RemoveMediaLyrics(session, revision uint64) error {
	return a.changeMediaLyrics(session, revision, func(ctx context.Context, scope platform.MediaLyricsScope) error {
		return a.media.lyrics.RemoveMediaLyrics(ctx, scope)
	})
}

func (a *App) SetMediaLyricsOffset(session, revision uint64, offsetMS int64) error {
	return a.changeMediaLyrics(session, revision, func(ctx context.Context, scope platform.MediaLyricsScope) error {
		return a.media.lyrics.SetMediaLyricsOffset(ctx, scope, offsetMS)
	})
}
