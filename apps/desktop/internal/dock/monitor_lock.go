package dock

import (
	"context"
	"errors"
	"sync"
	"time"

	"option-tab/internal/platform"
)

var ErrMonitorLockRetired = errors.New("dock monitor lock operation is no longer current")

type MonitorLockControllerDeps struct {
	Source platform.DockMonitorLockSource
	// Changed runs serially on Run, without controller locks. It must return
	// promptly. Consumers must admit snapshots by session/revision/sequence.
	Changed func(platform.DockMonitorLockState)
}

type MonitorLockController struct {
	mu                               sync.Mutex
	deps                             MonitorLockControllerDeps
	once                             sync.Once
	wake                             chan struct{}
	enabled, closed                  bool
	policy                           platform.DockMonitorLockPolicy
	state                            platform.DockMonitorLockState
	session, nativeSequence, request uint64
	active                           context.Context
	stop                             context.CancelFunc
	placement                        context.CancelFunc
	placementDone                    chan struct{}
	sourceDone                       <-chan struct{}
	retirement                       <-chan struct{}
}

func NewMonitorLockController(deps MonitorLockControllerDeps) *MonitorLockController {
	return &MonitorLockController{deps: deps, wake: make(chan struct{}, 1), state: platform.DockMonitorLockState{Status: "disabled"}}
}

func copyMonitorState(s platform.DockMonitorLockState) platform.DockMonitorLockState {
	s.Displays = append([]platform.DockLockDisplay(nil), s.Displays...)
	return s
}

func (c *MonitorLockController) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *MonitorLockController) Configure(enabled bool, p platform.DockMonitorLockPolicy) {
	c.mu.Lock()
	if c.closed || (c.enabled == enabled && c.policy.Target == p.Target && c.policy.DisplayUUID == p.DisplayUUID && c.policy.Bypass == p.Bypass) {
		c.mu.Unlock()
		return
	}
	p.Session = c.session
	p.Revision = c.policy.Revision + 1
	c.policy = p
	c.enabled = enabled
	if c.stop != nil {
		c.stop()
	}
	if c.placement != nil {
		c.placement()
	}
	status := "disabled"
	if enabled {
		status = "starting"
	}
	c.state = platform.DockMonitorLockState{Session: c.session, Revision: p.Revision, Sequence: 1, Status: status}
	c.mu.Unlock()
	c.signal()
}

func (c *MonitorLockController) Snapshot() platform.DockMonitorLockState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return copyMonitorState(c.state)
}

// RetireSource includes both the native observer and any admitted placement.
// Repeated retirement shares one receipt until a fresh source is started.
func (c *MonitorLockController) RetireSource() <-chan struct{} {
	c.mu.Lock()
	c.enabled = false
	c.policy.Revision++
	c.state = platform.DockMonitorLockState{Session: c.session, Revision: c.policy.Revision, Sequence: 1, Status: "disabled"}
	if c.stop != nil {
		c.stop()
	}
	if c.placement != nil {
		c.placement()
	}
	if c.retirement == nil {
		joined := make(chan struct{})
		c.retirement = joined
		source, placement := c.sourceDone, c.placementDone
		if source == nil && placement == nil {
			close(joined)
		} else {
			go func() {
				if source != nil {
					<-source
				}
				if placement != nil {
					<-placement
				}
				close(joined)
			}()
		}
	}
	receipt := c.retirement
	c.mu.Unlock()
	c.signal()
	return receipt
}

func (c *MonitorLockController) CancelPlacement() {
	c.mu.Lock()
	if c.placement != nil {
		c.placement()
	}
	c.mu.Unlock()
}
func (c *MonitorLockController) Run(ctx context.Context) { c.once.Do(func() { c.run(ctx) }) }

func (c *MonitorLockController) observe(ctx context.Context, p platform.DockMonitorLockPolicy, s platform.DockMonitorLockState) {
	c.mu.Lock()
	if c.closed || ctx.Err() != nil || c.active != ctx || !c.enabled || c.policy.Revision != p.Revision || s.Session != p.Session || s.Revision != p.Revision || s.Sequence <= c.nativeSequence || s.Generation == 0 || s.Generation < c.state.Generation {
		c.mu.Unlock()
		return
	}
	if s.Generation != c.state.Generation && c.placement != nil {
		c.placement()
	}
	c.nativeSequence = s.Sequence
	s.Sequence = c.state.Sequence + 1
	c.state = copyMonitorState(s)
	c.mu.Unlock()
	c.signal()
}

func (c *MonitorLockController) run(ctx context.Context) {
	var done chan error
	var revision uint64
	var lastAttempt time.Time
	var delivered platform.DockMonitorLockState
	notify := func() {
		s := c.Snapshot()
		if s.Session == delivered.Session && s.Revision == delivered.Revision && s.Sequence == delivered.Sequence && s.Status == delivered.Status {
			return
		}
		delivered = s
		if c.deps.Changed != nil {
			c.deps.Changed(s)
		}
	}
	retire := func() {
		c.mu.Lock()
		if c.stop != nil {
			c.stop()
		}
		if c.placement != nil {
			c.placement()
		}
		waitPlacement := c.placementDone
		c.active = nil
		c.stop = nil
		c.mu.Unlock()
		if done != nil {
			<-done
			done = nil
		}
		if waitPlacement != nil {
			<-waitPlacement
		}
		c.mu.Lock()
		c.sourceDone = nil
		c.mu.Unlock()
	}
	defer func() {
		c.mu.Lock()
		c.closed = true
		c.enabled = false
		c.policy.Revision++
		c.state = platform.DockMonitorLockState{Session: c.session, Revision: c.policy.Revision, Sequence: 1, Status: "disabled"}
		c.mu.Unlock()
		retire()
		notify()
	}()
	retry := time.NewTicker(time.Second)
	defer retry.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		c.mu.Lock()
		p := c.policy
		enabled := c.enabled
		c.mu.Unlock()
		if revision != p.Revision {
			retire()
			revision = p.Revision
		}
		c.mu.Lock()
		if done == nil && enabled && c.enabled && c.policy.Revision == revision && time.Since(lastAttempt) >= time.Second {
			lastAttempt = time.Now()
			if c.deps.Source == nil {
				c.state.Status = "unavailable"
				c.state.Reason = "Dock monitor locking is unsupported"
				c.state.Sequence++
			} else {
				c.session++
				c.policy.Session = c.session
				p = c.policy
				child, stop := context.WithCancel(ctx)
				c.active = child
				c.stop = stop
				joined := make(chan struct{})
				c.sourceDone = joined
				c.retirement = nil
				c.nativeSequence = 0
				c.state = platform.DockMonitorLockState{Session: p.Session, Revision: p.Revision, Sequence: 1, Status: "starting"}
				done = make(chan error, 1)
				completion := done
				go func() {
					err := c.deps.Source.ObserveDockMonitorLock(child, p, func(s platform.DockMonitorLockState) { c.observe(child, p, s) })
					stop()
					close(joined)
					completion <- err
				}()
			}
		}
		c.mu.Unlock()
		notify()
		select {
		case <-ctx.Done():
			return
		case <-c.wake:
		case <-retry.C:
		case err := <-done:
			done = nil
			c.mu.Lock()
			if c.stop != nil {
				c.stop()
			}
			if c.placement != nil {
				c.placement()
			}
			placementDone := c.placementDone
			c.active = nil
			c.stop = nil
			if c.enabled && c.policy.Revision == revision {
				if err == nil {
					err = errors.New("dock monitor observation ended unexpectedly")
				}
				c.state.Status = "unavailable"
				c.state.Reason = err.Error()
				c.state.Generation = 0
				c.state.Sequence++
			}
			c.mu.Unlock()
			if placementDone != nil {
				<-placementDone
			}
		}
	}
}

// Place waits for native completion, including cancellation cleanup. A source
// that ignores cancellation cannot be replaced by another placement operation.
func (c *MonitorLockController) Place(ctx context.Context) (platform.DockPlacementResult, error) {
	state := c.Snapshot()
	return c.PlaceScoped(ctx, state.Session, state.Revision, state.Generation)
}

// PlaceScoped admits only the exact state observed by the explicit caller.
// Settings changes or source recovery cannot redirect an older placement intent.
func (c *MonitorLockController) PlaceScoped(ctx context.Context, session, revision, generation uint64) (platform.DockPlacementResult, error) {
	if ctx == nil {
		return platform.DockPlacementResult{}, errors.New("placement requires context")
	}
	c.mu.Lock()
	if session != c.state.Session || revision != c.state.Revision || generation != c.state.Generation || ctx.Err() != nil || c.closed || !c.enabled || c.active == nil || c.active.Err() != nil || c.state.Generation == 0 || c.placementDone != nil || (c.state.Status != "awaitingPlacement" && c.state.Status != "protected") {
		c.mu.Unlock()
		return platform.DockPlacementResult{}, ErrMonitorLockRetired
	}
	c.request++
	req := platform.DockPlacementRequest{Session: c.state.Session, Revision: c.state.Revision, Generation: c.state.Generation, RequestID: c.request}
	child, cancel := context.WithCancel(ctx)
	unlink := context.AfterFunc(c.active, cancel)
	finished := make(chan struct{})
	c.placement = cancel
	c.placementDone = finished
	c.mu.Unlock()
	result, err := c.deps.Source.PlaceDock(child, req)
	c.mu.Lock()
	current := !c.closed && c.enabled && child.Err() == nil && c.active != nil && c.active.Err() == nil && c.state.Session == req.Session && c.state.Revision == req.Revision && c.state.Generation == req.Generation && (result.RequestID == req.RequestID || (err != nil && result.RequestID == 0))
	c.placement = nil
	c.placementDone = nil
	close(finished)
	c.mu.Unlock()
	unlink()
	cancel()
	c.signal()
	if !current {
		return platform.DockPlacementResult{RequestID: req.RequestID, Status: "cancelled", Reason: ErrMonitorLockRetired.Error()}, ErrMonitorLockRetired
	}
	return result, err
}
