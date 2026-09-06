package dock

import (
	"context"
	"errors"
	"sync"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

var ErrShakeRetired = errors.New("shake gesture is no longer current")

type ShakeControllerDeps struct {
	Source       platform.WindowDragObservationSource
	SelfAppID    domain.AppID
	Execute      func(ShakeIntent, func() error) error
	Failed       func(ShakeIntent, error)
	SourceFailed func(error)
}
type (
	shakeConfiguration struct {
		epoch   uint64
		enabled bool
		action  string
	}
	// Failed and SourceFailed callbacks must return promptly; they run without owner locks.
	// ShakeController separates the passive source from serial, potentially slow
	// actions. Execute must call its guard after external reads and before dispatch.
	// Run joins native observation; a blocked external Execute must return itself.
	ShakeController struct {
		deps               ShakeControllerDeps
		mu                 sync.Mutex
		state              shakeConfiguration
		closed             bool
		source, generation uint64
		cancel             context.CancelFunc
		wake               chan struct{}
		once               sync.Once
	}
)

func NewShakeController(deps ShakeControllerDeps) *ShakeController {
	return &ShakeController{deps: deps, wake: make(chan struct{}, 1)}
}

func (c *ShakeController) Configure(enabled bool, action string) {
	if action != "closeOthers" && action != "minimizeOthers" {
		action = "none"
	}
	enabled = enabled && action != "none"
	c.mu.Lock()
	if c.closed || (c.state.enabled == enabled && c.state.action == action) {
		c.mu.Unlock()
		return
	}
	c.state.epoch++
	c.state.enabled = enabled
	c.state.action = action
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *ShakeController) snapshot() shakeConfiguration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}
func (c *ShakeController) Run(ctx context.Context) { c.once.Do(func() { c.run(ctx) }) }

type (
	shakeDelivery struct {
		source, epoch uint64
		event         platform.WindowDragEvent
	}
	shakeJob struct {
		source, epoch uint64
		intent        ShakeIntent
		context       context.Context
	}
)

func (c *ShakeController) run(ctx context.Context) {
	events := make(chan shakeDelivery, 64)
	jobs := make(chan shakeJob, 1)
	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	go c.execute(workerCtx, jobs)
	var sourceID, epoch uint64
	var child context.Context
	var stop context.CancelFunc
	var done chan error
	var reducer *ShakeRecognizer
	var lastAttempt time.Time
	var lastError string
	var needsRecovery bool
	retry := time.NewTicker(time.Second)
	defer retry.Stop()
	report := func(err error) {
		state := c.snapshot()
		if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil || !state.enabled || state.epoch != epoch {
			return
		}
		needsRecovery = true
		if err.Error() != lastError {
			lastError = err.Error()
			if c.deps.SourceFailed != nil {
				c.deps.SourceFailed(err)
			}
		}
	}
	retire := func() {
		c.mu.Lock()
		c.source = 0
		c.generation = 0
		c.cancel = nil
		c.mu.Unlock()
		if stop != nil {
			stop()
			<-done // Obsolete source joins cannot publish errors for a new policy.
		}
		stop = nil
		done = nil
		child = nil
		reducer = nil
	}
	defer func() { c.mu.Lock(); c.closed = true; c.state.epoch++; c.mu.Unlock(); retire() }()
	apply := func() {
		state := c.snapshot()
		if state.epoch != epoch {
			retire()
			state = c.snapshot()
			epoch = state.epoch
			lastAttempt = time.Time{}
			lastError = ""
		}
		if !state.enabled {
			retire()
			return
		}
		if c.deps.Source == nil {
			report(errors.New("passive window drag observation unavailable"))
			return
		}
		if _, ok := c.deps.Source.(platform.WindowDragGestureValidator); !ok {
			report(errors.New("window drag gesture validation unavailable"))
			return
		}
		if stop != nil || time.Since(lastAttempt) < time.Second {
			return
		}
		lastAttempt = time.Now()
		sourceID++
		id := sourceID
		configuration := epoch
		child, stop = context.WithCancel(ctx)
		sourceCtx := child
		sourceCancel := stop
		done = make(chan error, 1)
		completion := done
		reducer = NewShakeRecognizer(state.action)
		c.mu.Lock()
		if c.state.epoch != epoch || c.closed {
			stop()
		} else {
			c.source = id
			c.generation = 0
			c.cancel = stop
		}
		c.mu.Unlock()
		go func() {
			err := c.deps.Source.ObserveWindowDrags(sourceCtx, func(event platform.WindowDragEvent) {
				if sourceCtx.Err() != nil {
					return
				}
				select {
				case events <- shakeDelivery{source: id, epoch: configuration, event: event}:
				case <-sourceCtx.Done():
				}
			})
			if err == nil && sourceCtx.Err() == nil {
				err = errors.New("passive window drag observation ended unexpectedly")
			}
			sourceCancel()
			completion <- err
		}()
	}
	apply()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
			apply()
		case <-retry.C:
			apply()
		case err := <-done:
			stop()
			stop = nil
			done = nil
			child = nil
			reducer = nil
			c.mu.Lock()
			c.source = 0
			c.generation = 0
			c.cancel = nil
			c.mu.Unlock()
			report(err)
		case delivery := <-events:
			if stop == nil || child.Err() != nil || reducer == nil || delivery.source != sourceID || delivery.epoch != epoch {
				continue
			}
			e := delivery.event
			c.mu.Lock()
			admitted := !c.closed && c.state.enabled && c.state.epoch == epoch && c.source == delivery.source && e.Generation != 0
			if admitted && c.generation == 0 {
				c.generation = e.Generation
			}
			admitted = admitted && c.generation == e.Generation
			c.mu.Unlock()
			if !admitted {
				continue
			}
			reducer.SetGeneration(e.Generation)
			if needsRecovery && (e.Kind == "candidate" || e.Kind == "moved") && validShakeSample(e) {
				needsRecovery = false
				lastError = ""
				if c.deps.SourceFailed != nil {
					c.deps.SourceFailed(nil)
				}
			}
			if e.Kind == "candidate" && e.Window.AppID == c.deps.SelfAppID {
				reducer.Reset()
				continue
			}
			if intent := reducer.Step(e); intent != nil {
				job := shakeJob{source: delivery.source, epoch: epoch, intent: *intent, context: child}
				select {
				case jobs <- job:
				default:
					if c.deps.Failed != nil {
						c.deps.Failed(*intent, errors.New("shake action queue is full"))
					}
				}
			}
		}
	}
}

func (c *ShakeController) admits(job shakeJob) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed && c.state.enabled && c.state.epoch == job.epoch && c.source == job.source && c.generation == job.intent.Generation
}

func (c *ShakeController) guard(ctx context.Context, job shakeJob) error {
	if ctx.Err() != nil || job.context.Err() != nil || !c.admits(job) {
		return ErrShakeRetired
	}
	validator, ok := c.deps.Source.(platform.WindowDragGestureValidator)
	if !ok {
		return ErrShakeRetired
	}
	i := job.intent
	current := validator.WindowDragGestureCurrent(platform.WindowDragValidation{Generation: i.Generation, GestureID: i.GestureID, WindowID: i.WindowID, AppID: i.AppID, WindowX: i.WindowX, WindowY: i.WindowY, ObservedAt: i.Timestamp})
	if !current || ctx.Err() != nil || job.context.Err() != nil || !c.admits(job) {
		return ErrShakeRetired
	}
	return nil
}

func (c *ShakeController) execute(ctx context.Context, jobs <-chan shakeJob) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			guard := func() error { return c.guard(ctx, job) }
			if c.deps.Execute == nil || guard() != nil {
				continue
			}
			if err := c.deps.Execute(job.intent, guard); err != nil && !errors.Is(err, ErrShakeRetired) && ctx.Err() == nil && c.admits(job) && c.deps.Failed != nil {
				c.deps.Failed(job.intent, err)
			}
		}
	}
}
