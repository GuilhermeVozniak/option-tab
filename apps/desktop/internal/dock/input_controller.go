package dock

import (
	"context"
	"errors"
	"sync"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

var ErrInputRetired = errors.New("dock input gesture is no longer current")

type InputAction struct {
	Intent Intent
	Item   platform.DockItem
}

// The source must hold at most one pending owned gesture until acknowledgement.
// Additional gestures pass through natively while an action is pending.
type InputControllerDeps struct {
	Source       platform.DockInputSource
	SelfAppID    domain.AppID
	Execute      func(InputAction, func() error) error
	Failed       func(InputAction, error)
	SourceFailed func(error)
}
type inputConfiguration struct {
	epoch   uint64
	enabled bool
	policy  platform.DockInputPolicy
	target  platform.DockInputTarget
}

// InputController keeps native source retirement separate from potentially slow
// application actions. Configure closes action admission synchronously; Run joins
// the old source before starting its replacement. Execute must invoke its guard
// again after any external lookup, immediately before native dispatch.
type InputController struct {
	deps   InputControllerDeps
	mu     sync.Mutex
	state  inputConfiguration
	closed bool
	cancel context.CancelFunc
	wake   chan struct{}
	once   sync.Once
}

func NewInputController(deps InputControllerDeps) *InputController {
	return &InputController{deps: deps, wake: make(chan struct{}, 1)}
}

func (c *InputController) Configure(enabled bool, policy platform.DockInputPolicy) {
	enabled = enabled && (policy.ClickToHide || policy.ScrollShowHide || policy.ModifiedRightClick)
	c.mu.Lock()
	if c.closed || (c.state.enabled == enabled && c.state.policy == policy) {
		c.mu.Unlock()
		return
	}
	c.state.epoch++
	c.state.enabled = enabled
	c.state.policy = policy
	cancel := c.cancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.notify()
}

func (c *InputController) Target(target platform.DockInputTarget) {
	if target.Item != nil {
		item := *target.Item
		target.Item = &item
	}
	c.mu.Lock()
	if c.closed || (target.Generation != 0 && target.Generation < c.state.target.Generation) {
		c.mu.Unlock()
		return
	}
	if target.Generation == c.state.target.Generation && target.ObservedAt.Before(c.state.target.ObservedAt) {
		c.mu.Unlock()
		return
	}
	var cancel context.CancelFunc
	if target.Generation == 0 {
		// Zero is explicit environment invalidation; an ordinary pointer exit
		// retains the observed generation and Dock PID with a nil item.
		c.state.epoch++
		cancel = c.cancel
		target.Generation = c.state.target.Generation
	}
	c.state.target = target
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.notify()
}

func (c *InputController) notify() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *InputController) snapshot() inputConfiguration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}
func (c *InputController) Run(ctx context.Context) { c.once.Do(func() { c.run(ctx) }) }

type (
	inputDelivery struct {
		source uint64
		event  platform.DockInputEvent
	}
	inputJob struct {
		action        InputAction
		configuration uint64
		generation    uint64
		dockPID       int
	}
)

func (c *InputController) run(ctx context.Context) {
	events := make(chan inputDelivery, 64)
	jobs := make(chan inputJob, 1)
	go c.execute(ctx, jobs)
	var sourceID, configuration uint64
	var targets chan platform.DockInputTarget
	var stop context.CancelFunc
	var done chan error
	var reducer *InputReducer
	var lastAttempt time.Time
	var pending uint64
	var lastSourceError string
	var sourceNeedsRecovery bool
	retry := time.NewTicker(time.Second)
	defer retry.Stop()
	retire := func() {
		if stop != nil {
			stop()
			<-done
		}
		stop = nil
		done = nil
		targets = nil
		c.mu.Lock()
		c.cancel = nil
		c.mu.Unlock()
	}
	defer func() { c.mu.Lock(); c.closed = true; c.state.epoch++; c.mu.Unlock(); retire() }()
	apply := func() {
		state := c.snapshot()
		if state.epoch != configuration {
			retire()
			state = c.snapshot()
			configuration = state.epoch
			reducer = nil
			lastSourceError = ""
			lastAttempt = time.Time{}
		}
		if !state.enabled || c.deps.Source == nil {
			retire()
			return
		}
		if stop == nil && time.Since(lastAttempt) >= time.Second {
			lastAttempt = time.Now()
			sourceID++
			id := sourceID
			child, cancel := context.WithCancel(ctx)
			stop = cancel
			done = make(chan error, 1)
			completion := done
			targets = make(chan platform.DockInputTarget, 1)
			in := targets
			reducer = NewInputReducer(state.policy, c.deps.SelfAppID)
			c.mu.Lock()
			if c.state.epoch == configuration && !c.closed {
				c.cancel = cancel
			} else {
				cancel()
			}
			c.mu.Unlock()
			go func() {
				completion <- c.deps.Source.ObserveDockInput(child, state.policy, in, func(event platform.DockInputEvent) {
					select {
					case events <- inputDelivery{source: id, event: event}:
					case <-child.Done():
					}
				})
			}()
		}
		if reducer != nil {
			reducer.SetGeneration(state.target.Generation)
		}
		if targets != nil {
			if state.target.Item != nil {
				item := *state.target.Item
				state.target.Item = &item
			}
			select {
			case targets <- state.target:
			default:
				select {
				case <-targets:
				default:
				}
				select {
				case targets <- state.target:
				default:
				}
			}
		}
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
		case sourceErr := <-done:
			// Observe has returned and joined its native lifetime. Do not read its
			// completion channel again; a later bounded retry may create a new source.
			stop = nil
			done = nil
			targets = nil
			c.mu.Lock()
			c.cancel = nil
			c.mu.Unlock()
			state := c.snapshot()
			if sourceErr != nil && !errors.Is(sourceErr, context.Canceled) && ctx.Err() == nil && state.enabled && state.epoch == configuration && sourceErr.Error() != lastSourceError {
				lastSourceError = sourceErr.Error()
				sourceNeedsRecovery = true
				if c.deps.SourceFailed != nil {
					c.deps.SourceFailed(sourceErr)
				}
			}
		case delivery := <-events:
			if delivery.source != sourceID || reducer == nil || stop == nil {
				continue
			}
			state := c.snapshot()
			if !state.enabled || state.epoch != configuration {
				continue
			}
			reducer.SetGeneration(state.target.Generation)
			if delivery.event.Owned && sourceNeedsRecovery {
				lastSourceError = ""
				sourceNeedsRecovery = false
				if c.deps.SourceFailed != nil {
					c.deps.SourceFailed(nil)
				}
			}
			for _, intent := range reducer.Step(delivery.event) {
				job := inputJob{action: InputAction{Intent: intent, Item: delivery.event.Item}, configuration: configuration, generation: delivery.event.Generation, dockPID: delivery.event.DockPID}
				select {
				case jobs <- job:
					pending = intent.GestureID
				default:
					c.complete(job.generation, intent.GestureID)
					if c.deps.Failed != nil {
						c.deps.Failed(job.action, errors.New("dock input source admitted another gesture before completion"))
					}
				}
			}
			event := delivery.event
			terminal := event.Kind == platform.DockInputCancelled || event.Kind == platform.DockInputLeftUp || event.Kind == platform.DockInputRightUp || event.Phase == "ended" || event.Phase == "cancelled" || event.MomentumPhase == "ended"
			if terminal && pending != event.GestureID {
				c.complete(event.Generation, event.GestureID)
			}

		}
	}
}

// Completed input keeps its original app target even when the pointer moves.
// Fresh icon-cache age governs initial native ownership, not an admitted action.
func (c *InputController) guard(ctx context.Context, job inputJob) error {
	if ctx.Err() != nil || !c.admits(job) {
		return ErrInputRetired
	}
	validator, ok := c.deps.Source.(platform.DockInputGestureValidator)
	if !ok || !validator.DockInputGestureCurrent(job.generation, job.action.Intent.GestureID) {
		return ErrInputRetired
	}
	// Native validation can query process identity. Reconfiguration must win
	// if it completes while that external check is in progress.
	if ctx.Err() != nil || !c.admits(job) {
		return ErrInputRetired
	}
	return nil
}

func (c *InputController) admits(job inputJob) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed && c.state.enabled && c.state.epoch == job.configuration && c.state.target.Generation == job.generation && c.state.target.DockPID == job.dockPID
}

func (c *InputController) execute(ctx context.Context, jobs <-chan inputJob) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			func() {
				defer c.complete(job.generation, job.action.Intent.GestureID)
				guard := func() error { return c.guard(ctx, job) }
				if c.deps.Execute == nil || guard() != nil {
					return
				}
				if err := c.deps.Execute(job.action, guard); err != nil && guard() == nil && c.deps.Failed != nil {
					c.deps.Failed(job.action, err)
				}
			}()
		}
	}
}

func (c *InputController) complete(generation, gesture uint64) {
	if source, ok := c.deps.Source.(platform.DockInputGestureAcknowledger); ok {
		source.CompleteDockInputGesture(generation, gesture)
	}
}
