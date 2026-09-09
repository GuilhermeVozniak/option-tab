package widgetproviders

import (
	"context"
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"option-tab/internal/media"
	"option-tab/internal/platform"
	"option-tab/internal/widgets"
)

type mediaController interface {
	Subscribe(platform.MediaProvider) uint64
	Unsubscribe(uint64)
	Snapshot(platform.MediaProvider) media.State
	Command(context.Context, uint64, platform.MediaScope, string, int64, func() error) error
}
type mediaLease struct {
	ctx                             context.Context
	subscription, epoch, generation uint64
	scope                           platform.MediaScope
	control                         bool
}

// Media adapts shared controller reservations; it never creates a native owner.
// Multiple widget observations may share this adapter and controller safely.
type Media struct {
	controller mediaController
	provider   platform.MediaProvider
	mu         sync.Mutex
	leases     map[uint64]*mediaLease
	clock      func() (<-chan time.Time, func())
}

var widgetMediaGeneration atomic.Uint64

func NewMedia(controller *media.Controller, provider platform.MediaProvider) *Media {
	if controller == nil {
		return newMedia(nil, provider)
	}
	return newMedia(controller, provider)
}

func newMedia(controller mediaController, provider platform.MediaProvider) *Media {
	return &Media{controller: controller, provider: provider, leases: map[uint64]*mediaLease{}, clock: func() (<-chan time.Time, func()) {
		timer := time.NewTimer(time.Second)
		return timer.C, func() { timer.Stop() }
	}}
}

func widgetMediaScope(s platform.MediaSample) platform.MediaScope {
	return platform.MediaScope{Provider: s.Provider, Process: s.Process, Generation: s.Generation, TrackEpoch: s.TrackEpoch, TrackID: s.Track.ID}
}

func widgetMediaReady(s platform.MediaSample, p platform.MediaProvider) bool {
	return s.Status == "ready" && s.Provider == p && s.Process.PID > 0 && s.Process.LaunchID != "" && s.Generation > 0 && s.TrackEpoch > 0 && s.Track.ID != ""
}

func boundedMediaText(s string) string { return boundedMediaString(s, 1024, 4096) }
func boundedMediaString(s string, maxRunes, maxBytes int) string {
	var out strings.Builder
	count := 0
	for _, r := range s {
		if r == 0 {
			continue
		}
		if count >= maxRunes || out.Len()+utf8.RuneLen(r) > maxBytes {
			break
		}
		out.WriteRune(r)
		count++
	}
	return out.String()
}

func mediaActionSpecs(s platform.MediaSample) map[string]widgets.ActionSpec {
	a := map[string]widgets.ActionSpec{}
	for k, v := range map[string]bool{"play": s.Capabilities.Play, "pause": s.Capabilities.Pause, "next": s.Capabilities.Next, "previous": s.Capabilities.Previous} {
		if v {
			a[k] = widgets.ActionSpec{Enabled: true}
		}
	}
	if (s.Playback == "playing" && s.Capabilities.Pause) || ((s.Playback == "paused" || s.Playback == "stopped") && s.Capabilities.Play) {
		a["playPause"] = widgets.ActionSpec{Enabled: true}
	}
	if s.Capabilities.Seek && s.Track.DurationMS > 0 && s.Track.DurationMS <= 1e12 {
		a["seek"] = widgets.ActionSpec{Enabled: true, Range: &widgets.NumberRange{Min: 0, Max: float64(s.Track.DurationMS - 1), Step: 1}}
	}
	return a
}

func mediaProjection(s platform.MediaSample, read, control bool) widgets.Sample {
	out := widgets.Sample{Status: s.Status, Reason: boundedMediaString(s.Reason, 160, 160), ObservedAt: s.ObservedAt, Fields: map[string]widgets.Value{}, Actions: map[string]widgets.ActionSpec{}}
	switch out.Status {
	case "notRunning":
		out.Status = "unavailable"
	case "denied":
		out.Status = "permissionRequired"
	}
	if !widgetMediaReady(s, s.Provider) {
		return out
	}
	if read {
		for k, v := range map[string]string{"title": s.Track.Title, "artist": s.Track.Artist, "album": s.Track.Album, "playback": s.Playback} {
			out.Fields[k] = textValue(boundedMediaText(v))
		}
		position, duration := float64(s.PositionMS), float64(s.Track.DurationMS)
		putNumber(out.Fields, "position", &position, 1e12)
		if duration > 0 {
			putNumber(out.Fields, "duration", &duration, 1e12)
		}
	}
	if control {
		out.Actions = mediaActionSpecs(s)
	}
	return out
}

func (p *Media) Observe(ctx context.Context, caps []string, emit func(widgets.Sample)) error {
	prefix := "media." + string(p.provider)
	if err := validObservation(ctx, caps, []string{prefix + ".read", prefix + ".control"}, emit); err != nil {
		return err
	}
	if p.controller == nil || !slices.Contains([]platform.MediaProvider{platform.MediaMusic, platform.MediaSpotify}, p.provider) {
		return widgets.ErrUnavailable
	}
	id := p.controller.Subscribe(p.provider)
	if id == 0 {
		return widgets.ErrUnavailable
	}
	defer func() {
		p.mu.Lock()
		for g, l := range p.leases {
			if l.subscription == id {
				delete(p.leases, g)
			}
		}
		p.mu.Unlock()
		p.controller.Unsubscribe(id)
	}()
	read, control := slices.Contains(caps, prefix+".read"), slices.Contains(caps, prefix+".control")
	var lease *mediaLease
	var previous media.State
	var sequence uint64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		state := p.controller.Snapshot(p.provider)
		if err := ctx.Err(); err != nil {
			return err
		}
		scope := widgetMediaScope(state.Sample)
		if lease == nil || lease.epoch != state.Epoch || lease.scope != scope {
			p.mu.Lock()
			if lease != nil {
				delete(p.leases, lease.generation)
			}
			lease = &mediaLease{ctx: ctx, subscription: id, epoch: state.Epoch, scope: scope, generation: widgetMediaGeneration.Add(1), control: control}
			p.leases[lease.generation] = lease
			p.mu.Unlock()
		}
		if sequence == 0 || state != previous {
			previous = state
			sequence++
			s := mediaProjection(state.Sample, read, control)
			if state.Provider != p.provider || state.Sample.Provider != p.provider {
				s = widgets.Sample{Status: "unavailable"}
			}
			s.Generation = lease.generation
			s.Sequence = sequence
			if s.ObservedAt.IsZero() {
				s.ObservedAt = time.Now()
			}
			if ctx.Err() == nil {
				emit(s)
			}
		}
		ticks, stop := p.clock()
		select {
		case <-ctx.Done():
			stop()
			return ctx.Err()
		case <-ticks:
			stop()
		}
	}
}

func (p *Media) current(ctx context.Context, l *mediaLease) (media.State, error) {
	if err := ctx.Err(); err != nil {
		return media.State{}, err
	}
	if l.ctx.Err() != nil {
		return media.State{}, widgets.ErrRetired
	}
	p.mu.Lock()
	valid := p.leases[l.generation] == l
	p.mu.Unlock()
	if !valid || !l.control {
		return media.State{}, widgets.ErrRetired
	}
	state := p.controller.Snapshot(p.provider)
	if state.Epoch != l.epoch || state.Provider != p.provider || widgetMediaScope(state.Sample) != l.scope || !widgetMediaReady(state.Sample, p.provider) {
		return media.State{}, widgets.ErrRetired
	}
	p.mu.Lock()
	valid = p.leases[l.generation] == l
	p.mu.Unlock()
	if !valid || l.ctx.Err() != nil {
		return media.State{}, widgets.ErrRetired
	}
	return state, ctx.Err()
}

func (p *Media) Perform(ctx context.Context, a widgets.ProviderAction, guard func() error) error {
	if ctx == nil || guard == nil || a.Generation == 0 || a.OptionID != "" {
		return widgets.ErrInvalid
	}
	p.mu.Lock()
	l := p.leases[a.Generation]
	p.mu.Unlock()
	if l == nil {
		return widgets.ErrRetired
	}
	state, err := p.current(ctx, l)
	if err != nil {
		return err
	}
	spec := mediaActionSpecs(state.Sample)[a.Action]
	if !spec.Enabled {
		return widgets.ErrUnavailable
	}
	kind := a.Action
	var position int64
	if kind == "seek" {
		if a.Value == nil || math.IsNaN(*a.Value) || math.IsInf(*a.Value, 0) || *a.Value < spec.Range.Min || *a.Value > spec.Range.Max || math.Trunc(*a.Value) != *a.Value {
			return widgets.ErrInvalid
		}
		position = int64(*a.Value)
	} else if a.Value != nil {
		return widgets.ErrInvalid
	}
	if kind == "playPause" {
		kind = "play"
		if state.Sample.Playback == "playing" {
			kind = "pause"
		}
	}
	final := func() error {
		current, e := p.current(ctx, l)
		if e != nil {
			return e
		}
		if !mediaCommandAllowed(current.Sample, kind, position) {
			return widgets.ErrUnavailable
		}
		if e = guard(); e != nil {
			return e
		}
		current, e = p.current(ctx, l)
		if e != nil {
			return e
		}
		if !mediaCommandAllowed(current.Sample, kind, position) {
			return widgets.ErrUnavailable
		}
		return nil
	}
	if err := final(); err != nil {
		return err
	}
	return p.controller.Command(ctx, l.subscription, l.scope, kind, position, final)
}

func mediaCommandAllowed(s platform.MediaSample, kind string, position int64) bool {
	spec := mediaActionSpecs(s)[kind]
	if !spec.Enabled {
		return false
	}
	return kind != "seek" || (spec.Range != nil && position >= 0 && float64(position) <= spec.Range.Max)
}
