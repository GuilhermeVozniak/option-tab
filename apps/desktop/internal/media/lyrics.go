package media

import (
	"errors"
	"math"
	"slices"
	"time"

	"option-tab/internal/platform"
)

const (
	maxAssociationOffsetMS = int64(30_000)
	maxExtrapolation       = 2 * time.Second
)

type LyricsScope struct {
	Provider string `json:"provider"`
	TrackID  string `json:"trackId"`
}

type LyricsDocument struct {
	ID     string      `json:"id"`
	Scope  LyricsScope `json:"scope"`
	Cues   []Cue       `json:"cues"`
	Status string      `json:"status"`
	Reason string      `json:"reason"`
}

type Timeline struct {
	now       func() time.Time
	scope     LyricsScope
	epoch     uint64
	cues      []Cue
	offsetMS  int64
	sample    platform.MediaSample
	hasSample bool
}

func NewTimeline(now func() time.Time) *Timeline {
	if now == nil {
		now = time.Now
	}
	return &Timeline{now: now}
}

func (t *Timeline) Load(scope LyricsScope, trackEpoch uint64, cues []Cue, offsetMS int64) error {
	if scope.Provider == "" || scope.TrackID == "" || trackEpoch == 0 {
		return errors.New("lyrics scope is incomplete")
	}
	if offsetMS < -maxAssociationOffsetMS || offsetMS > maxAssociationOffsetMS {
		return errors.New("lyrics timing offset must be within 30 seconds")
	}
	if t.hasSample && !sampleMatches(t.sample, scope, trackEpoch) {
		return errors.New("lyrics scope does not match the current track")
	}
	t.scope = scope
	t.epoch = trackEpoch
	t.cues = slices.Clone(cues)
	t.offsetMS = offsetMS
	return nil
}

func (t *Timeline) Observe(sample platform.MediaSample) {
	nextScope := LyricsScope{Provider: string(sample.Provider), TrackID: sample.Track.ID}
	if t.scope.Provider != "" && !sampleMatches(sample, t.scope, t.epoch) {
		t.scope = nextScope
		t.epoch = sample.TrackEpoch
		t.cues = nil
		t.offsetMS = 0
	}
	t.sample = sample
	t.hasSample = true
}

func sampleMatches(sample platform.MediaSample, scope LyricsScope, epoch uint64) bool {
	return string(sample.Provider) == scope.Provider && sample.Track.ID == scope.TrackID && sample.TrackEpoch == epoch
}

func (t *Timeline) PositionMS() int64 {
	return t.positionMS(t.now())
}

func (t *Timeline) positionMS(now time.Time) int64 {
	if !t.hasSample {
		return 0
	}
	position := t.sample.PositionMS
	if position < 0 {
		position = 0
	}
	if t.sample.Playback == "playing" && !t.sample.ObservedAt.IsZero() {
		elapsed := now.Sub(t.sample.ObservedAt)
		if elapsed > 0 {
			if elapsed > maxExtrapolation {
				elapsed = maxExtrapolation
			}
			addition := elapsed.Milliseconds()
			if position > math.MaxInt64-addition {
				position = math.MaxInt64
			} else {
				position += addition
			}
		}
	}
	if duration := t.sample.Track.DurationMS; duration > 0 && position > duration {
		position = duration
	}
	return position
}

func (t *Timeline) Active() int {
	_, active := t.PositionAndActive()
	return active
}

func (t *Timeline) PositionAndActive() (int64, int) {
	position := t.positionMS(t.now())
	if len(t.cues) == 0 || !sampleMatches(t.sample, t.scope, t.epoch) {
		return position, -1
	}
	lookup := position
	if t.offsetMS > 0 {
		if lookup < t.offsetMS {
			return position, -1
		}
		lookup -= t.offsetMS
	} else if t.offsetMS < 0 {
		addition := -t.offsetMS
		if lookup > math.MaxInt64-addition {
			lookup = math.MaxInt64
		} else {
			lookup += addition
		}
	}
	return position, ActiveCue(t.cues, lookup)
}

func (t *Timeline) Cues() []Cue { return slices.Clone(t.cues) }

func (t *Timeline) Clear() {
	t.scope = LyricsScope{}
	t.epoch = 0
	t.cues = nil
	t.offsetMS = 0
	t.sample = platform.MediaSample{}
	t.hasSample = false
}
