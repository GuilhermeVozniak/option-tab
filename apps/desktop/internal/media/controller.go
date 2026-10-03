// Package media shares bounded provider observations across independent panels.
package media

import (
	"context"
	"errors"
	"sync"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

var (
	ErrBusy    = errors.New("media provider command is busy")
	ErrRetired = errors.New("media subscription or track is no longer current")
)

type (
	State struct {
		Provider        platform.MediaProvider
		Revision, Epoch uint64
		Sample          platform.MediaSample
	}
	Deps struct {
		Source  platform.MediaProviderSource
		Changed func(State)
	}
	owner struct {
		epoch  uint64
		ctx    context.Context
		cancel context.CancelFunc
	}
	commandJob struct {
		subscription, epoch uint64
		cancel              context.CancelFunc
		done                chan struct{}
	}
	providerState struct {
		epoch   uint64
		state   State
		owner   *owner
		command *commandJob
		wake    chan struct{}
	}
	// Changed must return promptly. It runs outside every controller/native lock;
	// callbacks from native observations only coalesce immutable state notifications.
	Controller struct {
		deps           Deps
		mu             sync.Mutex
		settings       config.DockMediaSettings
		subscribers    map[uint64]platform.MediaProvider
		providers      map[platform.MediaProvider]*providerState
		next, revision uint64
		closed         bool
		once           sync.Once
		notify         chan struct{}
	}
)

func NewController(d Deps) *Controller {
	c := &Controller{deps: d, subscribers: map[uint64]platform.MediaProvider{}, providers: map[platform.MediaProvider]*providerState{}, notify: make(chan struct{}, 1)}
	for _, p := range []platform.MediaProvider{platform.MediaMusic, platform.MediaSpotify} {
		c.providers[p] = &providerState{epoch: 1, state: State{Provider: p, Epoch: 1, Sample: platform.MediaSample{Provider: p, Status: "unavailable", Reason: "media is disabled"}}, wake: make(chan struct{}, 1)}
	}
	return c
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Controller) enabled(p platform.MediaProvider) bool {
	return c.settings.Enabled && ((p == platform.MediaMusic && c.settings.MusicEnabled) || (p == platform.MediaSpotify && c.settings.SpotifyEnabled))
}

func (c *Controller) subscribed(p platform.MediaProvider) bool {
	for _, v := range c.subscribers {
		if v == p {
			return true
		}
	}
	return false
}

func (c *Controller) publish(p platform.MediaProvider, s platform.MediaSample) {
	v := c.providers[p]
	c.revision++
	v.state = State{Provider: p, Epoch: v.epoch, Revision: c.revision, Sample: s}
	signal(c.notify)
}

func (c *Controller) invalidate(p platform.MediaProvider) {
	v := c.providers[p]
	v.epoch++
	if v.owner != nil {
		v.owner.cancel()
	}
	if v.command != nil {
		v.command.cancel()
	}
	c.publish(p, platform.MediaSample{Provider: p, Status: "unavailable", Reason: "media observation is inactive"})
	signal(v.wake)
}

func (c *Controller) Configure(s config.DockMediaSettings) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.settings == s {
		return
	}
	previous := c.settings
	c.settings = s
	for p := range c.providers {
		wasEnabled := previous.Enabled && ((p == platform.MediaMusic && previous.MusicEnabled) || (p == platform.MediaSpotify && previous.SpotifyEnabled))
		if wasEnabled != c.enabled(p) {
			c.invalidate(p)
		}
	}
}

func (c *Controller) Subscribe(p platform.MediaProvider) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.providers[p] == nil {
		return 0
	}
	first := !c.subscribed(p)
	c.next++
	id := c.next
	c.subscribers[id] = p
	if first {
		c.invalidate(p)
	}
	return id
}

func (c *Controller) Unsubscribe(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.subscribers[id]
	if !ok {
		return
	}
	delete(c.subscribers, id)
	if job := c.providers[p].command; job != nil && job.subscription == id {
		job.cancel()
	}
	if !c.subscribed(p) {
		c.invalidate(p)
	}
}

func (c *Controller) Snapshot(p platform.MediaProvider) State {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v := c.providers[p]; v != nil {
		return v.state
	}
	return State{Provider: p, Sample: platform.MediaSample{Provider: p, Status: "unsupported", Reason: "unsupported media provider"}}
}

func (c *Controller) Run(ctx context.Context) {
	if ctx == nil {
		return
	}
	c.once.Do(func() { c.run(ctx) })
}

func (c *Controller) run(ctx context.Context) {
	var work sync.WaitGroup
	for p := range c.providers {
		work.Add(1)
		go func(p platform.MediaProvider) { defer work.Done(); c.observe(ctx, p) }(p)
	}
	delivery := make(chan struct{})
	go func() {
		defer close(delivery)
		last := map[platform.MediaProvider]uint64{}
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.notify:
			}
			for p := range c.providers {
				c.mu.Lock()
				v := c.providers[p]
				s := v.state
				current := !c.closed && ctx.Err() == nil && s.Revision > last[p]
				c.mu.Unlock()
				if current {
					last[p] = s.Revision
					if c.deps.Changed != nil {
						c.deps.Changed(s)
					}
				}
			}
		}
	}()
	<-ctx.Done()
	c.mu.Lock()
	c.closed = true
	for p := range c.providers {
		c.invalidate(p)
	}
	c.mu.Unlock()
	work.Wait()
	<-delivery
}

func (c *Controller) observe(ctx context.Context, p platform.MediaProvider) {
	v := c.providers[p]
	var retry time.Time
	var retryEpoch uint64
	for {
		c.mu.Lock()
		epoch := v.epoch
		wanted := !c.closed && ctx.Err() == nil && c.enabled(p) && c.subscribed(p)
		c.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		if !wanted {
			select {
			case <-ctx.Done():
				return
			case <-v.wake:
			}
			continue
		}
		if retryEpoch == epoch && time.Now().Before(retry) {
			timer := time.NewTimer(time.Until(retry))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-v.wake:
				timer.Stop()
			case <-timer.C:
			}
			continue
		}
		child, cancel := context.WithCancel(ctx)
		o := &owner{epoch: epoch, ctx: child, cancel: cancel}
		c.mu.Lock()
		if c.closed || v.epoch != epoch || !c.enabled(p) || !c.subscribed(p) {
			c.mu.Unlock()
			cancel()
			continue
		}
		v.owner = o
		c.mu.Unlock()
		done := make(chan error, 1)
		go func() {
			if c.deps.Source == nil {
				done <- errors.New("media source unavailable")
				return
			}
			done <- c.deps.Source.ObserveMedia(child, p, func(s platform.MediaSample) { c.accept(p, o, s) })
		}()
		var err error
		finished := false
	active:
		for {
			select {
			case err = <-done:
				finished = true
				break active
			case <-ctx.Done():
				break active
			case <-child.Done():
				break active
			case <-v.wake:
				c.mu.Lock()
				current := v.epoch == epoch && c.enabled(p) && c.subscribed(p)
				c.mu.Unlock()
				if !current {
					break active
				}
			}
		}
		cancel()
		if !finished {
			err = <-done
		}
		c.mu.Lock()
		job := v.command
		if job != nil && job.epoch == epoch {
			job.cancel()
		}
		if v.owner == o {
			v.owner = nil
		}
		current := !c.closed && ctx.Err() == nil && v.epoch == epoch && c.enabled(p) && c.subscribed(p)
		if current {
			v.epoch++ // Retrying a source is a new presentation-admission lifetime.
			reason := "media observation ended"
			if err != nil {
				reason = err.Error()
			}
			c.publish(p, platform.MediaSample{Provider: p, Status: "unavailable", Reason: reason})
			retry = time.Now().Add(time.Second)
			retryEpoch = v.epoch
		}
		c.mu.Unlock()
		if job != nil && job.epoch == epoch {
			<-job.done
		}
	}
}

func (c *Controller) accept(p platform.MediaProvider, o *owner, s platform.MediaSample) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v := c.providers[p]
	if c.closed || o.ctx.Err() != nil || v.owner != o || v.epoch != o.epoch || !c.enabled(p) || !c.subscribed(p) || s.Provider != p || s.Sequence == 0 || s.Generation == 0 {
		return
	}
	old := v.state.Sample
	if old.Sequence != 0 && (s.Sequence <= old.Sequence || s.Generation < old.Generation) {
		return
	}
	switch s.Status {
	case "ready", "notRunning", "permissionRequired", "denied", "unsupported", "unavailable":
	default:
		s.Status = "unavailable"
		s.Reason = "unsupported media source status"
	}
	c.publish(p, s)
}

func sampleScope(s platform.MediaSample) platform.MediaScope {
	return platform.MediaScope{Provider: s.Provider, Process: s.Process, Generation: s.Generation, TrackEpoch: s.TrackEpoch, TrackID: s.Track.ID}
}

func (c *Controller) current(id uint64, scope platform.MediaScope, epoch uint64) error {
	p, ok := c.subscribers[id]
	v := c.providers[p]
	if c.closed || !ok || p != scope.Provider || v == nil || v.epoch != epoch || !c.enabled(p) || v.owner == nil || v.owner.ctx.Err() != nil || v.owner.epoch != epoch || v.state.Sample.Status != "ready" || sampleScope(v.state.Sample) != scope {
		return ErrRetired
	}
	return nil
}

func (c *Controller) Command(ctx context.Context, id uint64, scope platform.MediaScope, kind string, positionMS int64, guard func() error) error {
	if ctx == nil || guard == nil {
		return errors.New("media command requires context and guard")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if scope.Process.PID <= 0 || scope.Process.LaunchID == "" || scope.Generation == 0 || scope.TrackEpoch == 0 || scope.TrackID == "" {
		return ErrRetired
	}
	c.mu.Lock()
	v := c.providers[scope.Provider]
	if v == nil {
		c.mu.Unlock()
		return ErrRetired
	}
	epoch := v.epoch
	if err := c.current(id, scope, epoch); err != nil {
		c.mu.Unlock()
		return err
	}
	s := v.state.Sample
	allowed := false
	switch kind {
	case "play":
		allowed = s.Capabilities.Play
	case "pause":
		allowed = s.Capabilities.Pause
	case "previous":
		allowed = s.Capabilities.Previous
	case "next":
		allowed = s.Capabilities.Next
	case "seek":
		allowed = s.Capabilities.Seek && positionMS >= 0 && s.Track.DurationMS > 0 && positionMS < s.Track.DurationMS
	}
	if !allowed {
		c.mu.Unlock()
		return errors.New("unsupported media command or position")
	}
	if v.command != nil {
		c.mu.Unlock()
		return ErrBusy
	}
	commandCtx, cancel := context.WithCancel(v.owner.ctx)
	stopCaller := context.AfterFunc(ctx, cancel)
	job := &commandJob{subscription: id, epoch: epoch, cancel: cancel, done: make(chan struct{})}
	v.command = job
	c.mu.Unlock()
	defer func() {
		stopCaller()
		cancel()
		c.mu.Lock()
		if v.command == job {
			v.command = nil
		}
		close(job.done)
		c.mu.Unlock()
	}()
	check := func() error {
		if e := commandCtx.Err(); e != nil {
			return e
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		c.mu.Lock()
		e := c.current(id, scope, epoch)
		c.mu.Unlock()
		return e
	}
	final := func() error {
		if e := check(); e != nil {
			return e
		}
		if e := guard(); e != nil {
			return e
		}
		return check()
	}
	if e := final(); e != nil {
		return e
	}
	return c.deps.Source.PerformMediaCommandGuarded(commandCtx, platform.MediaCommand{Scope: scope, Kind: kind, PositionMS: positionMS}, final)
}
