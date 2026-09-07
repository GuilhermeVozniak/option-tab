package launcher

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type (
	inventory struct {
		epoch      uint64
		items      []Item
		targets    map[string]platform.LauncherAppTarget
		references map[string]platform.LauncherReference
		icons      map[string]string
		ok         bool
	}
	hideDeadline struct{ enter, leave time.Time }
	Controller   struct {
		mu                                     sync.Mutex
		deps                                   Deps
		settings                               config.ReplacementDockSettings
		epoch, nextSession, nextItem           uint64
		suspended, closed, running, actionBusy bool
		state                                  State
		env                                    platform.LauncherEnvironment
		items                                  []Item
		targets                                map[string]platform.LauncherAppTarget
		references                             map[string]platform.LauncherReference
		itemIcons                              map[string]string
		inventoryReady                         bool
		sourceCancel                           context.CancelFunc
		inventoryCancel                        context.CancelFunc
		activeSourceEpoch                      uint64
		failed                                 map[string]time.Time
		spaces                                 map[string]uint64
		childAdmissions                        map[string]uint64
		nextChildAdmission                     uint64
		childSessions                          map[string]uint64
		childHolds                             map[string]childHold
		hide                                   map[string]hideDeadline
		wake                                   chan struct{}
	}
)

func New(d Deps) *Controller {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Controller{deps: d, settings: config.DefaultReplacementDock(), state: State{Status: "disabled", Displays: []DisplayState{}, Presentations: []Presentation{}}, targets: map[string]platform.LauncherAppTarget{}, failed: map[string]time.Time{}, spaces: map[string]uint64{}, hide: map[string]hideDeadline{}, wake: make(chan struct{}, 1)}
}

func (c *Controller) notify() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *Controller) retireLocked() {
	if c.inventoryCancel != nil {
		c.inventoryCancel()
	}
	c.epoch++
	c.state.Epoch = c.epoch
	c.state.PointerOwned = false
	c.state.Enabled = c.settings.Enabled
	c.state.Status = "preparing"
	if !c.settings.Enabled {
		c.state.Status = "disabled"
	}
	if c.suspended {
		c.state.Status = "suspended"
	}
	for i := range c.state.Presentations {
		p := &c.state.Presentations[i]
		p.Visible = false
		p.Epoch = c.epoch
		p.Revision++
		p.Reason = c.state.Status
		p.Widgets = nil
	}
	c.env = platform.LauncherEnvironment{}
	c.inventoryReady = false
	c.items = nil
	c.targets = map[string]platform.LauncherAppTarget{}
	c.references = nil
	c.itemIcons = nil
	c.failed = map[string]time.Time{}
	c.hide = map[string]hideDeadline{}
	c.childAdmissions = nil
	c.childSessions = nil
	c.childHolds = nil
}

func (c *Controller) Configure(s config.ReplacementDockSettings) error {
	if err := config.ValidateReplacementDock(s); err != nil {
		return err
	}
	s = config.CloneReplacementDock(s)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrRetired
	}
	if reflect.DeepEqual(c.settings, s) {
		c.mu.Unlock()
		return nil
	}
	c.settings = s
	c.retireLocked()
	cancel := c.sourceCancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.notify()
	return nil
}

func (c *Controller) Suspend(v bool) {
	c.mu.Lock()
	if c.closed || c.suspended == v {
		c.mu.Unlock()
		return
	}
	c.suspended = v
	c.retireLocked()
	cancel := c.sourceCancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.notify()
}

func (c *Controller) Snapshot() State { c.mu.Lock(); defer c.mu.Unlock(); return cloneState(c.state) }

func (c *Controller) FailDisplay(scope Scope, reason string) {
	c.mu.Lock()
	for i := range c.state.Presentations {
		p := &c.state.Presentations[i]
		if p.Scope == scope && p.Visible && p.ProfileID == c.profileForDisplayLocked(p.DisplayUUID) {
			c.failed[p.DisplayUUID] = c.deps.Now().Add(time.Second)
			c.state.PointerOwned = false
			p.Visible = false
			p.Revision++
			p.Reason = "hostUnavailable"
			p.Widgets = nil
		}
	}
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) acceptEnvironment(epoch uint64, e platform.LauncherEnvironment) {
	e.Displays = slices.Clone(e.Displays)
	normalizeFocusedEvidence(&e)
	if e.Generation == 0 || e.Sequence == 0 || len(e.Displays) > 32 {
		return
	}
	c.mu.Lock()
	if c.closed || c.epoch != epoch || c.suspended || !c.settings.Enabled || e.Generation < c.env.Generation || (e.Generation == c.env.Generation && e.Sequence <= c.env.Sequence) {
		c.mu.Unlock()
		return
	}
	previous := c.env
	c.env = e
	c.retireChangedChildDisplaysLocked(previous)
	c.retireChangedProfilesLocked()
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) environmentReadyLocked() bool {
	e := c.env
	age := c.deps.Now().Sub(e.ObservedAt)
	seen := map[string]bool{}
	mainCount := 0
	for _, d := range e.Displays {
		if d.UUID == "" || seen[d.UUID] {
			return false
		}
		seen[d.UUID] = true
		if d.Main {
			mainCount++
		}
	}
	if mainCount > 1 {
		return false
	}
	return e.Complete && e.Status == "ready" && e.PointerKnown && finite(e.PointerX) && finite(e.PointerY) && !e.ObservedAt.IsZero() && age >= -250*time.Millisecond && age <= 1500*time.Millisecond && e.NativeDock.Confidence == "known" && e.NativeDock.Process.PID > 0 && e.NativeDock.Process.StartSeconds > 0 && validBounds(e.NativeDock.Bounds) && (e.NativeDock.Edge == "bottom" || e.NativeDock.Edge == "left" || e.NativeDock.Edge == "right") && (e.NativeDock.Visibility == "visible" || e.NativeDock.Visibility == "hidden")
}

func (c *Controller) ordinaryLocked(uuid string) bool {
	if !c.environmentReadyLocked() {
		return false
	}
	for _, d := range c.env.Displays {
		if d.UUID == uuid {
			return d.SpaceID > 0 && d.SpaceKind == "ordinary" && d.SpaceStatus == "known" && d.MirrorGroup == "" && c.spaces[uuid] == d.SpaceID && !c.protectedPointer(d)
		}
	}
	return false
}

func (c *Controller) protectedPointer(d platform.LauncherDisplay) bool {
	e := c.env
	x, y := e.PointerX, e.PointerY
	edge := domain.Bounds{X: d.Frame.X, Y: d.Frame.Y + d.Frame.H - 32, W: d.Frame.W, H: 32}
	if contains(edge, x, y) {
		return true
	}
	return overlap(d.Frame, e.NativeDock.Bounds) && contains(expand(e.NativeDock.Bounds, 12), x, y)
}

func (c *Controller) currentLocked(scope Scope, id string) bool {
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) {
		return false
	}
	if target, ok := c.targets[id]; !ok || target.Process.PID == c.deps.SelfAppID || (c.deps.SelfBundleID != "" && target.BundleID == c.deps.SelfBundleID) {
		return false
	}
	for _, p := range c.state.Presentations {
		if p.Scope == scope && p.Visible && p.ProfileID == c.profileForDisplayLocked(p.DisplayUUID) {
			return true
		}
	}
	return false
}

func (c *Controller) Activate(ctx context.Context, scope Scope, id string) error {
	if strings.HasPrefix(id, "pin:") {
		return c.PerformConfigured(ctx, scope, id, "open")
	}
	if ctx == nil {
		return ErrRetired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if !c.currentLocked(scope, id) {
		c.mu.Unlock()
		return ErrRetired
	}
	if c.actionBusy {
		c.mu.Unlock()
		return ErrBusy
	}
	if c.deps.Activate == nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	target := c.targets[id]
	target.DisplayUUID = scope.DisplayUUID
	c.actionBusy = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.actionBusy = false; c.mu.Unlock() }()
	guard := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if !c.currentLocked(scope, id) {
			return ErrRetired
		}
		return nil
	}
	return c.deps.Activate(ctx, scope, target, guard)
}

// Apps cannot be cancelled. One worker serializes inventory, identity and optional icon reads.
func (c *Controller) inventoryWorker(ctx context.Context, requests <-chan uint64, results chan<- inventory) {
	for {
		select {
		case <-ctx.Done():
			return
		case epoch := <-requests:
			out := inventory{epoch: epoch, targets: map[string]platform.LauncherAppTarget{}}
			if c.deps.Applications != nil && c.deps.Identities != nil {
				apps, err := c.deps.Applications.Apps()
				if err == nil && len(apps) <= 128 {
					out.ok = true
					seen := map[domain.AppID]bool{}
					for _, a := range apps {
						c.mu.Lock()
						current := c.epoch == epoch && !c.closed && !c.suspended && c.settings.Enabled
						c.mu.Unlock()
						if ctx.Err() != nil || !current {
							break
						}
						if a.ID <= 0 || a.BundleID == "" || a.Name == "" || seen[a.ID] || a.ID == c.deps.SelfAppID || (c.deps.SelfBundleID != "" && a.BundleID == c.deps.SelfBundleID) {
							continue
						}
						seen[a.ID] = true
						if c.deps.Eligible != nil && !c.deps.Eligible(a) {
							continue
						}
						identity, err := c.deps.Identities.ProcessIdentity(a.ID)
						if err != nil || identity.PID != a.ID || identity.StartSeconds == 0 {
							out.ok = false
							continue
						}
						target := platform.LauncherAppTarget{Process: identity, BundleID: a.BundleID, Name: a.Name}
						c.mu.Lock()
						id := ""
						for key, previous := range c.targets {
							if previous == target {
								id = key
								break
							}
						}
						if id == "" {
							c.nextItem++
							id = fmt.Sprintf("item-%d", c.nextItem)
						}
						c.mu.Unlock()
						icon := ""
						if source, ok := c.deps.Applications.(interface{ AppIcon(int, int) string }); ok {
							icon = source.AppIcon(int(a.ID), 64)
							if len(icon) > 256*1024 || !strings.HasPrefix(icon, "data:image/png;base64,") {
								icon = ""
							}
						}
						out.items = append(out.items, Item{ID: id, Name: a.Name, Icon: icon})
						out.targets[id] = target
					}
				}
			}
			c.readConfiguredInventory(ctx, &out)
			select {
			case results <- out:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *Controller) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrRetired
	}
	c.mu.Lock()
	if c.running || c.closed {
		c.mu.Unlock()
		return ErrRetired
	}
	c.running = true
	c.mu.Unlock()
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()
	requests := make(chan uint64, 1)
	results := make(chan inventory, 1)
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); c.inventoryWorker(workCtx, requests, results) }()
	maintenance := time.NewTicker(100 * time.Millisecond)
	defer maintenance.Stop()
	var clock *time.Ticker
	defer func() {
		if clock != nil {
			clock.Stop()
		}
	}()
	var sourceDone chan error
	var retryAt, nextInventory time.Time
	busyInventory := false
	var published State
	publish := func() {
		c.mu.Lock()
		s := cloneState(c.state)
		c.mu.Unlock()
		if !reflect.DeepEqual(s, published) {
			published = cloneState(s)
			if c.deps.View != nil {
				c.deps.View.Publish(s)
			}
		}
	}
	for {
		c.mu.Lock()
		enabled := c.settings.Enabled && !c.suspended && !c.closed
		epoch := c.epoch
		now := c.deps.Now()
		if enabled && sourceDone == nil && !now.Before(retryAt) && c.deps.Environment != nil {
			sourceCtx, cancel := context.WithCancel(workCtx)
			c.sourceCancel = cancel
			c.activeSourceEpoch = epoch
			sourceDone = make(chan error, 1)
			ch := sourceDone
			go func() {
				defer close(ch)
				ch <- c.deps.Environment.ObserveLauncherEnvironment(sourceCtx, func(e platform.LauncherEnvironment) { c.acceptEnvironment(epoch, e) })
			}()
		}
		c.reconcileLocked()
		hasClock := false
		for _, p := range c.state.Presentations {
			if p.Visible {
				for _, w := range p.Widgets {
					hasClock = hasClock || w.Status == "ready"
				}
			}
		}
		if enabled && c.environmentReadyLocked() && !busyInventory && !now.Before(nextInventory) {
			busyInventory = true
			requests <- epoch
			nextInventory = now.Add(time.Second)
		}
		c.mu.Unlock()
		if hasClock && clock == nil {
			clock = time.NewTicker(time.Second)
		} else if !hasClock && clock != nil {
			clock.Stop()
			clock = nil
		}
		var clockC <-chan time.Time
		if clock != nil {
			clockC = clock.C
		}
		publish()
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.closed = true
			c.retireLocked()
			c.state.Status = "closed"
			cancel := c.sourceCancel
			c.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			cancelWork()
			publish()
			if sourceDone != nil {
				<-sourceDone
			}
			<-workerDone
			return ctx.Err()
		case <-c.wake:
		case <-maintenance.C:
		case <-clockC:
		case out := <-results:
			busyInventory = false
			c.mu.Lock()
			if out.epoch == c.epoch && !c.closed && c.settings.Enabled && !c.suspended {
				c.inventoryReady = out.ok
				c.items = out.items
				c.targets = out.targets
				c.references = out.references
				c.itemIcons = out.icons
			}
			c.mu.Unlock()
		case <-sourceDone:
			c.mu.Lock()
			oldEpoch := c.activeSourceEpoch
			c.sourceCancel = nil
			c.env = platform.LauncherEnvironment{}
			for uuid := range c.childAdmissions {
				c.nextChildAdmission++
				c.childAdmissions[uuid] = c.nextChildAdmission
			}
			c.childHolds = nil
			c.inventoryReady = false
			c.mu.Unlock()
			sourceDone = nil
			if oldEpoch == epoch {
				retryAt = c.deps.Now().Add(time.Second)
			} else {
				retryAt = time.Time{}
			}
		}
	}
}
