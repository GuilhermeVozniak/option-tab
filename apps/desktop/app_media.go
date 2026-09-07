package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/media"
	"option-tab/internal/platform"
)

type MediaArtworkView struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Image  string `json:"image"`
}
type MediaLyricsView struct {
	DocumentID string      `json:"documentID"`
	Status     string      `json:"status"`
	Reason     string      `json:"reason"`
	Cues       []media.Cue `json:"cues"`
	OffsetMS   int64       `json:"offsetMS"`
}
type MediaViewState struct {
	InteractionEpoch uint64                 `json:"interactionEpoch"`
	Session          uint64                 `json:"session"`
	Revision         uint64                 `json:"revision"`
	Open             bool                   `json:"open"`
	Pinned           bool                   `json:"pinned"`
	Pinnable         bool                   `json:"pinnable"`
	Provider         platform.MediaProvider `json:"provider"`
	Scope            platform.MediaScope    `json:"scope"`
	Sample           platform.MediaSample   `json:"sample"`
	Appearance       config.Appearance      `json:"appearance"`
	Artwork          MediaArtworkView       `json:"artwork"`
	Lyrics           MediaLyricsView        `json:"lyrics"`
	PositionMS       int64                  `json:"positionMS"`
	ActiveCue        int                    `json:"activeCue"`
	Error            string                 `json:"error"`
}
type mediaPresentation struct {
	state          MediaViewState
	subscription   uint64
	progress       uint64
	ctx            context.Context
	cancel         context.CancelFunc
	host           *dockWindow
	bounds         domain.Bounds
	displayUUID    string
	nativeSequence uint64
}
type mediaPermissionOwner struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Every presentation/asset map below is protected by App.viewMu. Native calls
// and source joins run outside it. Controller callbacks only deliver snapshots.
type appMediaRuntime struct {
	source         platform.MediaProviderSource
	lyrics         platform.MediaLyricsSource
	controller     *media.Controller
	artwork        *media.ArtworkCache
	ctx            context.Context
	cancel         context.CancelFunc
	once           sync.Once
	next, hover    uint64
	panels         map[uint64]*mediaPresentation
	assets         map[platform.MediaProvider]*mediaTrackAssets
	permissions    map[platform.MediaProvider]platform.MediaPermission
	permissionJobs map[platform.MediaProvider]*mediaPermissionOwner
	importJob      *mediaImportOwner
	remoteArtwork  bool
}

func (a *App) wireMedia(source platform.MediaProviderSource, lyrics platform.MediaLyricsSource) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &appMediaRuntime{source: source, lyrics: lyrics, ctx: ctx, cancel: cancel, panels: map[uint64]*mediaPresentation{}, assets: map[platform.MediaProvider]*mediaTrackAssets{}, permissions: map[platform.MediaProvider]platform.MediaPermission{}, permissionJobs: map[platform.MediaProvider]*mediaPermissionOwner{}}
	r.controller = media.NewController(media.Deps{Source: source, Changed: a.acceptMedia})
	r.artwork = media.NewArtworkCache(source)
	r.remoteArtwork = a.settingsSnapshot().Dock.Media.RemoteArtwork
	a.media = r
	r.controller.Configure(a.settingsSnapshot().Dock.Media)
}

func (a *App) startMedia() {
	if a.media == nil {
		return
	}
	r := a.media
	r.once.Do(func() {
		go r.controller.Run(r.ctx)
		go func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
					a.publishMediaProgress()
				}
			}
		}()
	})
}

func mediaProviderEnabled(s config.DockMediaSettings, p platform.MediaProvider) bool {
	return s.Enabled && ((p == platform.MediaMusic && s.MusicEnabled) || (p == platform.MediaSpotify && s.SpotifyEnabled))
}

func mediaScope(s platform.MediaSample) platform.MediaScope {
	return platform.MediaScope{Provider: s.Provider, Process: s.Process, Generation: s.Generation, TrackEpoch: s.TrackEpoch, TrackID: s.Track.ID}
}

func (a *App) mediaAllowedLocked(p platform.MediaProvider) bool {
	if a.media == nil || a.media.ctx.Err() != nil || a.sessionInactive {
		return false
	}
	s := a.settingsSnapshot()
	return !s.Behavior.Paused && mediaProviderEnabled(s.Dock.Media, p)
}

func (a *App) mediaPresentationLocked(id uint64) *mediaPresentation {
	if a.media == nil {
		return nil
	}
	return a.media.panels[id]
}

func (a *App) mediaTargetLocked(id, revision uint64) (*mediaPresentation, error) {
	p := a.mediaPresentationLocked(id)
	if p == nil || p.state.Revision != revision || !a.mediaPanelAllowedLocked(p) {
		return nil, media.ErrRetired
	}
	return p, nil
}

func (a *App) mediaPanelAllowedLocked(p *mediaPresentation) bool {
	if p == nil || !p.state.Open || p.ctx.Err() != nil || !a.mediaAllowedLocked(p.state.Provider) {
		return false
	}
	if p.state.Pinned {
		return true
	}
	if a.dockState.ContentKind != "media" || a.media.hover != p.state.Session || !a.dockItemAllowedLocked(a.dockState.Item) {
		return false
	}
	return a.dockController == nil || a.dockState.AdmissionEpoch == 0 || a.dockState.AdmissionEpoch == a.dockController.AdmissionEpoch()
}

func (a *App) newMediaPresentationLocked(provider platform.MediaProvider, pinned bool) *mediaPresentation {
	r := a.media
	r.next++
	ctx, cancel := context.WithCancel(r.ctx)
	p := &mediaPresentation{state: MediaViewState{InteractionEpoch: 1, Session: r.next, Open: true, Pinned: pinned, Pinnable: !pinned && a.mediaPinFactory != nil, Provider: provider, Appearance: a.settingsSnapshot().Dock.Appearance, ActiveCue: -1}, ctx: ctx, cancel: cancel}
	p.subscription = r.controller.Subscribe(provider)
	r.panels[p.state.Session] = p
	a.applyMediaSampleLocked(p, r.controller.Snapshot(provider))
	return p
}

func (a *App) syncMediaHoverLocked(st dock.State) {
	if a.media == nil {
		return
	}
	provider := dock.MediaProviderForItem(st.Item, a.settingsSnapshot().Dock.Media)
	if st.ContentKind != "media" || provider == "" {
		a.retireMediaHoverLocked()
		return
	}
	if p := a.media.panels[a.media.hover]; p != nil && p.state.Provider == provider {
		if p.state.Appearance != st.Appearance {
			p.state.Appearance = st.Appearance
			a.publishMediaLocked(p)
		}
		return
	}
	a.retireMediaHoverLocked()
	p := a.newMediaPresentationLocked(provider, false)
	p.state.Appearance = st.Appearance
	a.media.hover = p.state.Session
}

func (a *App) retireMediaHoverLocked() {
	if a.media != nil && a.media.hover != 0 {
		a.retireMediaPresentationLocked(a.media.hover)
		a.media.hover = 0
	}
}

func (a *App) retireMediaPresentationLocked(id uint64) {
	p := a.mediaPresentationLocked(id)
	if p == nil {
		return
	}
	p.cancel()
	if p.host != nil {
		p.host.close()
	}
	a.media.controller.Unsubscribe(p.subscription)
	delete(a.media.panels, id)
	p.state.Open = false
	p.state.Revision++
	a.emit("media:hide", dockSessionEvent{Session: id, Revision: p.state.Revision})
	for _, other := range a.media.panels {
		if other.state.Provider == p.state.Provider {
			return
		}
	}
	if asset := a.media.assets[p.state.Provider]; asset != nil {
		asset.cancel()
		delete(a.media.assets, p.state.Provider)
	}
}

func cloneMediaView(s MediaViewState) MediaViewState {
	s.Lyrics.Cues = slices.Clone(s.Lyrics.Cues)
	return s
}

func (a *App) mediaViewLocked(p *mediaPresentation) MediaViewState {
	s := cloneMediaView(p.state)
	if asset := a.media.assets[s.Provider]; asset != nil && asset.scope == s.Scope {
		s.Artwork = asset.artwork
		s.Lyrics = asset.lyrics
		s.Lyrics.Cues = slices.Clone(asset.lyrics.Cues)
		s.PositionMS, s.ActiveCue = asset.timeline.PositionAndActive()
	}
	return s
}

func (a *App) GetMediaState(session uint64) *MediaViewState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	p := a.mediaPresentationLocked(session)
	if p == nil {
		return nil
	}
	s := a.mediaViewLocked(p)
	return &s
}

func (a *App) publishMediaLocked(p *mediaPresentation) {
	p.state.Revision++
	s := a.mediaViewLocked(p)
	if a.media.hover == p.state.Session {
		a.dockViewState.Media = &s
	}
	a.emit("media:update", s)
}

func sameMediaMetadata(a, b platform.MediaSample) bool {
	a.Sequence = 0
	b.Sequence = 0
	a.PositionMS = 0
	b.PositionMS = 0
	a.ObservedAt = time.Time{}
	b.ObservedAt = time.Time{}
	return a == b
}

func (a *App) applyMediaSampleLocked(p *mediaPresentation, st media.State) {
	old := p.state.Sample
	p.state.Sample = st.Sample
	p.state.Scope = mediaScope(st.Sample)
	a.ensureMediaAssetsLocked(st)
	if !sameMediaMetadata(old, st.Sample) || p.state.Revision == 0 {
		p.state.Error = ""
		a.publishMediaLocked(p)
	}
}

func (a *App) acceptMedia(st media.State) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.media == nil || a.media.ctx.Err() != nil {
		return
	}
	current := a.media.controller.Snapshot(st.Provider)
	if current.Revision != st.Revision || current.Epoch != st.Epoch {
		return
	}
	for _, p := range a.media.panels {
		if p.state.Provider == st.Provider && p.state.Open && a.mediaAllowedLocked(st.Provider) {
			a.applyMediaSampleLocked(p, st)
		}
	}
	if st.Sample.Status == "ready" || st.Sample.Status == "permissionRequired" || st.Sample.Status == "denied" {
		a.media.permissions[st.Provider] = platform.MediaPermission{Status: st.Sample.Status, Reason: st.Sample.Reason}
	}
}

func (a *App) publishMediaProgress() {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.media == nil || a.media.ctx.Err() != nil {
		return
	}
	for _, p := range a.media.panels {
		if !p.state.Open || !a.mediaAllowedLocked(p.state.Provider) {
			continue
		}
		asset := a.media.assets[p.state.Provider]
		if asset == nil || asset.scope != p.state.Scope {
			continue
		}
		p.progress++
		position, active := asset.timeline.PositionAndActive()
		a.emit("media:progress", struct {
			Session    uint64 `json:"session"`
			Revision   uint64 `json:"revision"`
			Sequence   uint64 `json:"sequence"`
			PositionMS int64  `json:"positionMS"`
			ActiveCue  int    `json:"activeCue"`
		}{p.state.Session, p.state.Revision, p.progress, position, active})
	}
}

// Initial RPC admission uses the rendered revision. Once accepted, native
// preparation keeps exact panel and track authority while progress/artwork can
// update the view independently. Controller also checks its current owner epoch.
func (a *App) PerformMediaAction(session, revision uint64, kind string, positionMS int64) error {
	a.viewMu.Lock()
	p, err := a.mediaTargetLocked(session, revision)
	if err != nil {
		a.viewMu.Unlock()
		return err
	}
	scope, subscription, ctx := p.state.Scope, p.subscription, p.ctx
	interaction := p.state.InteractionEpoch
	a.viewMu.Unlock()
	guard := func() error {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		current := a.mediaPresentationLocked(session)
		if current != p || p.state.Scope != scope || p.state.InteractionEpoch != interaction || !a.mediaPanelAllowedLocked(p) {
			return media.ErrRetired
		}
		return nil
	}
	err = a.media.controller.Command(ctx, subscription, scope, kind, positionMS, guard)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.mediaPresentationLocked(session) == p && p.state.Scope == scope && p.state.InteractionEpoch == interaction && a.mediaPanelAllowedLocked(p) {
		p.state.Error = ""
		if err != nil {
			p.state.Error = err.Error()
		}
		a.publishMediaLocked(p)
	}
	return err
}

func (a *App) GetMediaPermissions() map[string]platform.MediaPermission {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	out := map[string]platform.MediaPermission{}
	for _, p := range []platform.MediaProvider{platform.MediaMusic, platform.MediaSpotify} {
		status := platform.MediaPermission{Status: "permissionRequired"}
		if a.media == nil {
			status.Status = "unsupported"
		} else if known, ok := a.media.permissions[p]; ok {
			status = known
		}
		out[string(p)] = status
	}
	return out
}

func (a *App) ConnectMediaProvider(provider string) (platform.MediaPermission, error) {
	p := platform.MediaProvider(provider)
	a.viewMu.Lock()
	if !a.mediaAllowedLocked(p) || a.media.source == nil {
		a.viewMu.Unlock()
		return platform.MediaPermission{}, media.ErrRetired
	}
	if a.media.permissionJobs[p] != nil {
		a.viewMu.Unlock()
		return platform.MediaPermission{}, media.ErrBusy
	}
	ctx, cancel := context.WithCancel(a.media.ctx)
	owner := &mediaPermissionOwner{ctx, cancel}
	a.media.permissionJobs[p] = owner
	a.viewMu.Unlock()
	result, err := a.media.source.RequestMediaPermission(ctx, p)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	defer cancel()
	if a.media.permissionJobs[p] == owner {
		delete(a.media.permissionJobs, p)
	}
	if ctx.Err() != nil || !a.mediaAllowedLocked(p) {
		return platform.MediaPermission{}, media.ErrRetired
	}
	if err != nil && result.Status == "" {
		result = platform.MediaPermission{Status: "unavailable", Reason: err.Error()}
	}
	a.media.permissions[p] = result
	a.emit("media:permission", struct {
		Provider platform.MediaProvider `json:"provider"`
		Status   string                 `json:"status"`
		Reason   string                 `json:"reason"`
	}{p, result.Status, result.Reason})
	return result, err
}

func (a *App) syncMediaLocked() {
	if a.media == nil {
		return
	}
	r := a.media
	s := a.settingsSnapshot()
	effective := s.Dock.Media
	select {
	case <-a.captureStop:
		r.cancel()
		effective.Enabled = false
	default:
	}
	if s.Behavior.Paused || a.sessionInactive {
		effective.Enabled = false
	}
	r.controller.Configure(effective)
	for provider, job := range r.permissionJobs {
		if !a.mediaAllowedLocked(provider) {
			job.cancel()
		}
	}
	if r.importJob != nil && !a.mediaAllowedLocked(r.importJob.scope.Provider) {
		r.importJob.cancel()
	}
	for id, p := range r.panels {
		if !mediaProviderEnabled(s.Dock.Media, p.state.Provider) || r.ctx.Err() != nil {
			a.retireMediaPresentationLocked(id)
			continue
		}
		if p.state.Pinned {
			changed := p.state.Appearance != s.Dock.Appearance
			p.state.Appearance = s.Dock.Appearance
			open := a.mediaAllowedLocked(p.state.Provider)
			if p.state.Open != open {
				p.state.InteractionEpoch++
				p.state.Open = open
				p.state.Sample = r.controller.Snapshot(p.state.Provider).Sample
				p.state.Scope = mediaScope(p.state.Sample)
				p.state.Error = ""
				if p.host != nil {
					if open {
						p.host.show(p.bounds)
					} else {
						p.host.hide()
					}
				}
				// Temporary suspension retains the pin session. Terminal close is
				// the only path emitting media:hide.
				changed = true
			}
			if changed {
				a.publishMediaLocked(p)
			}
		}
	}
	for provider, asset := range r.assets {
		if !a.mediaAllowedLocked(provider) {
			asset.cancel()
			delete(r.assets, provider)
		}
	}
	// Remote artwork policy owns only Spotify's artwork request. Local lyrics,
	// provider observations, and Music artwork retain their independent lifetime.
	if r.remoteArtwork != s.Dock.Media.RemoteArtwork {
		r.remoteArtwork = s.Dock.Media.RemoteArtwork
		if asset := r.assets[platform.MediaSpotify]; asset != nil {
			a.queueMediaArtworkLocked(asset, r.remoteArtwork)
			a.publishMediaAssetsLocked(asset)
		}
	}
	if !effective.Enabled {
		r.artwork.Clear()
	}
}

var errMediaLyricsUnavailable = errors.New("local lyric import is unavailable")
