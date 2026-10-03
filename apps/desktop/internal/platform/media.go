package platform

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

type MediaProvider string

const (
	MediaMusic   MediaProvider = "music"
	MediaSpotify MediaProvider = "spotify"
)

type MediaProcess struct {
	PID      int    `json:"pid"`
	LaunchID string `json:"launchID"`
}
type MediaTrack struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	Album      string `json:"album"`
	DurationMS int64  `json:"durationMS"`
}
type MediaCapabilities struct {
	Play     bool `json:"play"`
	Pause    bool `json:"pause"`
	Previous bool `json:"previous"`
	Next     bool `json:"next"`
	Seek     bool `json:"seek"`
}
type MediaSample struct {
	Provider     MediaProvider     `json:"provider"`
	Process      MediaProcess      `json:"process"`
	Generation   uint64            `json:"generation"`
	Sequence     uint64            `json:"sequence"`
	TrackEpoch   uint64            `json:"trackEpoch"`
	Track        MediaTrack        `json:"track"`
	Playback     string            `json:"playback"`
	PositionMS   int64             `json:"positionMS"`
	ObservedAt   time.Time         `json:"observedAt"`
	Status       string            `json:"status"`
	Reason       string            `json:"reason"`
	Capabilities MediaCapabilities `json:"capabilities"`
	ArtworkToken string            `json:"artworkToken"`
}
type MediaScope struct {
	Provider   MediaProvider `json:"provider"`
	Process    MediaProcess  `json:"process"`
	Generation uint64        `json:"generation"`
	TrackEpoch uint64        `json:"trackEpoch"`
	TrackID    string        `json:"trackID"`
}
type MediaCommand struct {
	Scope      MediaScope
	Kind       string
	PositionMS int64
}
type MediaPermission struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type MediaArtwork struct {
	PNG    []byte
	Status string
	Reason string
}

// MediaProviderLyrics contains provider-supplied text, not verified timestamps.
// Data is transient; consumers must validate it before presenting timed cues.
type MediaProviderLyrics struct {
	Scope  MediaScope
	Data   []byte
	Status string
	Reason string
}

// MediaProviderLyricsSource is optional; local lyric associations are separate.
type MediaProviderLyricsSource interface {
	ReadMediaProviderLyricsGuarded(context.Context, MediaScope, func() error) (MediaProviderLyrics, error)
}

type MediaProviderSource interface {
	ObserveMedia(context.Context, MediaProvider, func(MediaSample)) error
	RequestMediaPermission(context.Context, MediaProvider) (MediaPermission, error)
	PerformMediaCommandGuarded(context.Context, MediaCommand, func() error) error
	ReadMediaArtwork(context.Context, MediaScope, string) (MediaArtwork, error)
}

// Each provider gate serializes bounded native calls without holding a Go mutex
// across an AppleEvent or the caller's final admission callback.
type mediaTransport interface {
	read(context.Context, MediaProvider) (MediaSample, string, error)
	permission(context.Context, MediaProvider, bool) (MediaPermission, error)
	command(context.Context, MediaCommand, func() error) error
	artwork(context.Context, MediaScope, string) ([]byte, error)
}
type mediaLyricsTransport interface {
	lyrics(context.Context, MediaScope, func() error) (MediaProviderLyrics, error)
}
type mediaOwner struct {
	observation context.Context
	gate        chan struct{}
	active      bool
	sample      MediaSample
	artwork     string
	token       uint64
}
type mediaSource struct {
	transport mediaTransport
	owners    map[MediaProvider]*mediaOwner
}

func newMediaSource(t mediaTransport) *mediaSource {
	return &mediaSource{transport: t, owners: map[MediaProvider]*mediaOwner{MediaMusic: {gate: make(chan struct{}, 1)}, MediaSpotify: {gate: make(chan struct{}, 1)}}}
}

func mediaScope(v MediaSample) MediaScope {
	return MediaScope{Provider: v.Provider, Process: v.Process, Generation: v.Generation, TrackEpoch: v.TrackEpoch, TrackID: v.Track.ID}
}

func (s *mediaSource) acquire(ctx context.Context, p MediaProvider) (*mediaOwner, error) {
	o := s.owners[p]
	if o == nil {
		return nil, errors.New("unsupported media provider")
	}
	select {
	case o.gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-o.gate
			return nil, err
		}
		return o, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func mediaRelease(o *mediaOwner) { <-o.gate }
func (s *mediaSource) ObserveMedia(ctx context.Context, p MediaProvider, emit func(MediaSample)) error {
	if emit == nil {
		return errors.New("media callback required")
	}
	o, err := s.acquire(ctx, p)
	if err != nil {
		return err
	}
	if o.active {
		mediaRelease(o)
		return errors.New("media provider already observed")
	}
	o.active = true
	o.observation = ctx
	o.sample.Generation++
	o.sample.TrackEpoch++
	o.sample.Sequence = 0
	mediaRelease(o)
	defer func() {
		owned, _ := s.acquire(context.Background(), p)
		owned.active = false
		owned.sample.Generation++
		owned.artwork = ""
		mediaRelease(owned)
	}()
	for {
		o, err = s.acquire(ctx, p)
		if err != nil {
			return err
		}
		next, art, readErr := s.transport.read(ctx, p)
		if ctx.Err() != nil {
			mediaRelease(o)
			return ctx.Err()
		}
		next.Provider = p
		next.Generation = o.sample.Generation
		next.TrackEpoch = o.sample.TrackEpoch
		next.Sequence = o.sample.Sequence + 1
		next.ObservedAt = time.Now()
		if next.Process != o.sample.Process {
			next.Generation++
			next.TrackEpoch++
		}
		if next.Track.ID != o.sample.Track.ID {
			next.TrackEpoch++
		}
		if readErr != nil {
			next.Status = "unavailable"
			next.Reason = readErr.Error()
		}
		if next.Status != "ready" {
			next.Capabilities = MediaCapabilities{}
			next.Track = MediaTrack{}
			art = ""
		}
		if p == MediaSpotify {
			next.Track.DurationMS = 0
			next.Capabilities.Seek = false
		}
		if art != "" {
			if art != o.artwork || mediaScope(next) != mediaScope(o.sample) {
				o.token++
			}
			next.ArtworkToken = fmt.Sprintf("%d:%d:%d", next.Generation, next.TrackEpoch, o.token)
		}
		o.sample = next
		o.artwork = art
		mediaRelease(o)
		emit(next)
		delay := time.Second
		if next.Playback == "playing" {
			delay = 500 * time.Millisecond
		}
		if next.Status != "ready" {
			delay = 5 * time.Second
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *mediaSource) RequestMediaPermission(ctx context.Context, p MediaProvider) (MediaPermission, error) {
	o, err := s.acquire(ctx, p)
	if err != nil {
		return MediaPermission{}, err
	}
	defer mediaRelease(o)
	return s.transport.permission(ctx, p, true)
}

func (s *mediaSource) PerformMediaCommandGuarded(ctx context.Context, c MediaCommand, guard func() error) error {
	o, err := s.acquire(ctx, c.Scope.Provider)
	if err != nil {
		return err
	}
	defer mediaRelease(o)
	if !o.active || (o.observation != nil && o.observation.Err() != nil) || o.sample.Status != "ready" || c.Scope.TrackID == "" || mediaScope(o.sample) != c.Scope {
		return errors.New("media track retired")
	}
	caps := o.sample.Capabilities
	allowed := false
	switch c.Kind {
	case "play":
		allowed = caps.Play
	case "pause":
		allowed = caps.Pause
	case "previous":
		allowed = caps.Previous
	case "next":
		allowed = caps.Next
	case "seek":
		allowed = caps.Seek && c.PositionMS >= 0 && c.PositionMS < o.sample.Track.DurationMS
	}
	if !allowed {
		return errors.New("media command unavailable")
	}
	if guard == nil {
		return errors.New("media command guard required")
	}
	return s.transport.command(ctx, c, func() error {
		if o.observation != nil && o.observation.Err() != nil {
			return errors.New("media observation retired")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return guard()
	})
}

func (s *mediaSource) ReadMediaArtwork(ctx context.Context, scope MediaScope, token string) (MediaArtwork, error) {
	o, err := s.acquire(ctx, scope.Provider)
	if err != nil {
		return MediaArtwork{}, err
	}
	valid := func() bool {
		return o.active && (o.observation == nil || o.observation.Err() == nil) && scope == mediaScope(o.sample) && token != "" && token == o.sample.ArtworkToken
	}
	if !valid() {
		mediaRelease(o)
		return MediaArtwork{Status: "missing"}, errors.New("media artwork retired")
	}
	raw := o.artwork
	remote := scope.Provider == MediaSpotify
	if remote {
		mediaRelease(o)
	}
	data, readErr := s.transport.artwork(ctx, scope, raw)
	if remote {
		o, err = s.acquire(ctx, scope.Provider)
		if err != nil {
			return MediaArtwork{}, err
		}
	}
	defer mediaRelease(o)
	if ctx.Err() != nil {
		return MediaArtwork{}, ctx.Err()
	}
	if !valid() {
		return MediaArtwork{Status: "missing"}, errors.New("media artwork retired")
	}
	if readErr != nil {
		return MediaArtwork{Status: "unavailable", Reason: readErr.Error()}, readErr
	}
	return MediaArtwork{PNG: data, Status: "ready"}, nil
}

func (s *mediaSource) ReadMediaProviderLyricsGuarded(ctx context.Context, scope MediaScope, guard func() error) (MediaProviderLyrics, error) {
	unavailable := func(err error) (MediaProviderLyrics, error) {
		return MediaProviderLyrics{Scope: scope, Status: "unavailable", Reason: "Music lyrics are unavailable"}, err
	}
	transport, supported := s.transport.(mediaLyricsTransport)
	if scope.Provider != MediaMusic || !supported {
		return MediaProviderLyrics{Scope: scope, Status: "unsupported", Reason: "Provider lyrics are unavailable"}, nil
	}
	if scope.Process.PID <= 0 || scope.Process.PID > math.MaxInt32 || scope.Process.LaunchID == "" || len(scope.Process.LaunchID) > 4096 ||
		scope.Generation == 0 || scope.TrackEpoch == 0 || scope.TrackID == "" || len(scope.TrackID) > 4096 ||
		!utf8.ValidString(scope.Process.LaunchID) || !utf8.ValidString(scope.TrackID) ||
		strings.ContainsRune(scope.Process.LaunchID, 0) || strings.ContainsRune(scope.TrackID, 0) || guard == nil {
		return unavailable(errors.New("media lyric scope or guard is invalid"))
	}
	o, err := s.acquire(ctx, scope.Provider)
	if err != nil {
		return unavailable(err)
	}
	defer mediaRelease(o)
	current := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !o.active || o.observation == nil || o.observation.Err() != nil || o.sample.Status != "ready" || mediaScope(o.sample) != scope {
			return errors.New("media lyrics retired")
		}
		return nil
	}
	admit := func() error {
		if err := current(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		return current()
	}
	if err := admit(); err != nil {
		return unavailable(err)
	}
	result, err := transport.lyrics(ctx, scope, admit)
	if admissionErr := admit(); admissionErr != nil {
		return unavailable(admissionErr)
	}
	if err != nil || result.Scope != scope || len(result.Data) > 1<<20 || !utf8.Valid(result.Data) {
		return unavailable(errors.New("music lyrics are unavailable"))
	}
	switch result.Status {
	case "ready":
		if len(result.Data) == 0 {
			return MediaProviderLyrics{Scope: scope, Status: "missing", Reason: "Music has no lyrics for this track"}, nil
		}
		return MediaProviderLyrics{Scope: scope, Status: "ready", Data: append([]byte(nil), result.Data...)}, nil
	case "missing":
		return MediaProviderLyrics{Scope: scope, Status: "missing", Reason: "Music has no lyrics for this track"}, nil
	case "unsupported":
		return MediaProviderLyrics{Scope: scope, Status: "unsupported", Reason: "Provider lyrics are unavailable"}, nil
	default:
		return unavailable(errors.New("music lyrics are unavailable"))
	}
}
