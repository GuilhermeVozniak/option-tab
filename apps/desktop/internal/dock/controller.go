package dock

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/filter"
	"option-tab/internal/order"
	"option-tab/internal/platform"
)

type command struct {
	kind      string
	session   uint64
	settings  config.Settings
	suspended bool
	bounds    domain.Bounds
	window    domain.WindowID
}

type observation struct {
	epoch uint64
	value platform.DockObservation
}

type windowResult struct {
	session, generation uint64
	windows             []domain.Window
	screen              domain.Bounds
	emptyReason         string
	excluded            bool
	err                 error
}

// Controller admits observations and commands on one owner loop. Window and
// environment queries run separately; at most one query can be in flight even
// while the hovered app changes. Results never outlive their visible session.
type Controller struct {
	admission    atomic.Uint64
	deps         Deps
	initial      config.Settings
	commands     []command
	commandMu    sync.Mutex
	commandReady chan struct{}
	observations chan observation
	results      chan windowResult
	done         chan struct{}
	once         sync.Once
}

func NewController(deps Deps, settings config.Settings) *Controller {
	c := &Controller{deps: deps, initial: settings.Normalize(), commandReady: make(chan struct{}, 1), observations: make(chan observation, 1), results: make(chan windowResult, 1), done: make(chan struct{})}
	c.admission.Store(1)
	return c
}

func (c *Controller) Configure(s config.Settings) {
	c.admission.Add(1)
	c.send(command{kind: "configure", settings: s.Normalize()})
}

// AdmissionEpoch changes synchronously when prior work must be discarded, even
// if a later resume replaces the queued suspension command.
func (c *Controller) AdmissionEpoch() uint64 { return c.admission.Load() }

func (c *Controller) Suspend(suspended bool) {
	if suspended {
		c.admission.Add(1)
	}
	c.send(command{kind: "suspend", suspended: suspended})
}

func (c *Controller) SetPanelBounds(session uint64, bounds domain.Bounds) {
	c.send(command{kind: "bounds", session: session, bounds: bounds})
}

func (c *Controller) SelectWindow(session uint64, id domain.WindowID) {
	c.send(command{kind: "select", session: session, window: id})
}
func (c *Controller) Refresh(session uint64) { c.send(command{kind: "refresh", session: session}) }
func (c *Controller) Dismiss(session uint64) { c.send(command{kind: "dismiss", session: session}) }

func (c *Controller) send(cmd command) {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	select {
	case <-c.done:
		return
	default:
	}
	// One entry per kind, ordered by its last update. Older sessions cannot
	// displace a newer queued command, even when their callbacks arrive late.
	for i, queued := range c.commands {
		if queued.kind != cmd.kind {
			continue
		}
		if queued.session > cmd.session {
			return
		}
		c.commands = append(c.commands[:i], c.commands[i+1:]...)
		break
	}
	c.commands = append(c.commands, cmd)
	select {
	case c.commandReady <- struct{}{}:
	default:
	}
}

func (c *Controller) takeCommands() []command {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	batch := c.commands
	c.commands = nil
	return batch
}

func (c *Controller) finish() {
	c.commandMu.Lock()
	defer c.commandMu.Unlock()
	c.commands = nil
	close(c.done)
}

func (c *Controller) Run(ctx context.Context) {
	c.once.Do(func() {
		defer c.finish()
		loop := controllerLoop{controller: c, ctx: ctx, settings: c.initial}
		loop.run()
	})
}

type controllerLoop struct {
	admission                            uint64
	controller                           *Controller
	ctx                                  context.Context
	settings                             config.Settings
	suspended                            bool
	hover                                *Hover
	epoch, sequence, generation, session uint64
	stopObserver                         context.CancelFunc
	observerDone                         chan struct{}
	restartPending                       bool
	last                                 *platform.DockObservation
	candidate, blocked                   *Item
	state                                State
	shown, querying, dirty               bool
	screen                               domain.Bounds
	measured                             domain.Bounds
	pointer                              PointerState
	pointerBounds                        domain.Bounds
}

func (l *controllerLoop) run() {
	l.restartObserver()
	defer func() {
		l.stop()
		l.publishInputTarget(platform.DockInputTarget{})
		l.reset()
		if l.observerDone != nil {
			<-l.observerDone
		}
	}()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	refresh := time.NewTicker(500 * time.Millisecond)
	defer refresh.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-l.observerDone:
			l.observerDone = nil
			l.stopObserver = nil
			l.startObserver()
		case observed := <-l.controller.observations:
			l.observe(observed)
		case <-l.controller.commandReady:
			for _, cmd := range l.controller.takeCommands() {
				l.command(cmd)
			}
		case result := <-l.controller.results:
			l.querying = false
			l.accept(result)
			l.query()
		case at := <-tick.C:
			l.step(at)
		case <-refresh.C:
			if l.shown {
				l.request()
			}
		}
	}
}

func (l *controllerLoop) enabled() bool {
	return l.settings.Dock.Enabled && !l.settings.Behavior.Paused && !l.suspended
}

func (l *controllerLoop) stop() {
	if l.stopObserver != nil {
		l.stopObserver()
		l.stopObserver = nil
	}
	l.epoch++
}

func (l *controllerLoop) restartObserver() {
	l.admission = l.controller.AdmissionEpoch()
	l.stop()
	l.reset()
	l.publishInputTarget(platform.DockInputTarget{})
	l.hover = NewHover(time.Duration(l.settings.Dock.HoverDelayMs)*time.Millisecond, time.Duration(l.settings.Dock.DismissDelayMs)*time.Millisecond)
	l.sequence = 0
	l.generation = 0
	l.blocked = nil
	l.restartPending = true
	l.startObserver()
}

// Cancellation is cooperative: keep the retiring invocation until its native
// resources are gone, while continuing to process configuration commands.
func (l *controllerLoop) startObserver() {
	if l.observerDone != nil || !l.restartPending || !l.enabled() || l.ctx.Err() != nil || l.controller.deps.Observations == nil {
		return
	}
	l.restartPending = false
	done := make(chan struct{})
	l.observerDone = done
	ctx, cancel := context.WithCancel(l.ctx)
	l.stopObserver = cancel
	epoch := l.epoch
	c := l.controller
	go func() {
		defer close(done)
		emit := func(value platform.DockObservation) {
			if ctx.Err() != nil {
				return
			}
			value.Item = cloneDockItem(value.Item)
			event := observation{epoch: epoch, value: value}
			// A slow view cannot backlog pointer samples or block a native worker.
			select {
			case c.observations <- event:
				return
			default:
			}
			select {
			case <-c.observations:
			default:
			}
			select {
			case c.observations <- event:
			case <-ctx.Done():
			default:
			}
		}
		if err := c.deps.Observations.ObserveDock(ctx, emit); err != nil && ctx.Err() == nil {
			emit(platform.DockObservation{Status: "dockUnavailable"})
		}
	}()
}

func cloneDockItem(item *platform.DockItem) *platform.DockItem {
	if item == nil {
		return nil
	}
	copy := *item
	return &copy
}

func (l *controllerLoop) reset() {
	if l.shown && l.controller.deps.View != nil {
		l.controller.deps.View.Hide(l.state.Session)
	}
	l.session++
	l.shown = false
	l.candidate = nil
	l.state = State{}
	l.last = nil
	l.dirty = false
	l.measured = domain.Bounds{}
	if l.hover != nil {
		l.hover.Reset()
	}
}

func (l *controllerLoop) observe(event observation) {
	if event.epoch != l.epoch || !l.enabled() {
		return
	}
	value := event.value
	// Sequence zero is reserved for terminal observer startup/delivery errors.
	if value.Sequence != 0 && value.Sequence <= l.sequence {
		return
	}
	if value.Sequence != 0 {
		l.sequence = value.Sequence
	}
	if value.Status != "ready" {
		l.publishInputTarget(platform.DockInputTarget{})
		l.reset()
		l.blocked = nil
		return
	}
	if value.Sequence == 0 {
		return
	}
	if value.Generation != l.generation {
		l.reset()
		l.blocked = nil
		l.generation = value.Generation
	}
	target := platform.DockInputTarget{Generation: value.Generation, DockPID: value.DockPID, ObservedAt: value.ObservedAt, Item: cloneDockItem(value.Item)}
	l.publishInputTarget(target)
	l.last = &value
	l.step(time.Now())
	l.publishPointer()
}

func (l *controllerLoop) publishInputTarget(target platform.DockInputTarget) {
	if view, ok := l.controller.deps.View.(InputTargetView); ok {
		view.InputTarget(l.admission, target)
	}
}

func (l *controllerLoop) step(at time.Time) {
	if !l.enabled() || l.last == nil {
		return
	}
	observed := l.last
	var item *Item
	if native := observed.Item; native != nil {
		item = &Item{Kind: "app", AppID: native.AppID, BundleID: native.BundleID, Path: native.Path, Title: native.Title, Bounds: native.Bounds, ScreenID: native.ScreenID, Edge: native.Edge}
	}
	if l.blocked != nil {
		if item != nil && item.same(l.blocked) {
			return
		}
		l.blocked = nil
	}
	// Tolerate AX hit-test jitter only while the opening delay is pending.
	if item == nil && l.hover.shown == nil && l.hover.pending != nil && inflate(l.hover.pending.Bounds, float64(l.settings.Dock.HoverSlopPx)).ContainsPoint(observed.PointerX, observed.PointerY) {
		item = copyItem(l.hover.pending)
	}
	overPanel := l.shown && overDockSurface(observed.PointerX, observed.PointerY, l.state.Item.Bounds, l.state.Bounds, l.state.Item.Edge, float64(l.settings.Dock.BridgePaddingPx))
	change := l.hover.Step(at, item, overPanel)
	if change.Hide {
		l.reset()
		return
	}
	if change.Show != nil {
		if l.shown && l.controller.deps.View != nil {
			l.controller.deps.View.Hide(l.state.Session)
		}
		l.session++
		l.shown = false
		l.state = State{}
		l.measured = domain.Bounds{}
		l.candidate = copyItem(change.Show)
		l.request()
	}
	if change.Move != nil {
		if l.candidate != nil && l.candidate.ScreenID != change.Move.ScreenID {
			// Old queries carry old display bounds. Give the migrated preview a
			// fresh session and hide until its own display has been resolved.
			if l.shown && l.controller.deps.View != nil {
				l.controller.deps.View.Hide(l.state.Session)
			}
			l.session++
			l.shown = false
			l.state = State{}
			l.measured = domain.Bounds{}
		}
		l.candidate = copyItem(change.Move)
		if l.shown {
			l.state.Item = *change.Move
			l.place()
			l.publish(false)
		}
		l.request()
	}
}

func (l *controllerLoop) command(cmd command) {
	switch cmd.kind {
	case "configure":
		l.settings = cmd.settings
		l.restartObserver()
		return
	case "suspend":
		if l.suspended != cmd.suspended || l.admission != l.controller.AdmissionEpoch() {
			l.suspended = cmd.suspended
			l.restartObserver()
		}
		return
	}
	if !l.shown || cmd.session != l.state.Session {
		return
	}
	switch cmd.kind {
	case "dismiss":
		blocked := copyItem(&l.state.Item)
		l.reset()
		l.blocked = blocked
	case "refresh":
		l.request()
	case "select":
		for _, window := range l.state.Windows {
			if window.ID == cmd.window && window.ID != l.state.SelectedWindowID {
				l.state.SelectedWindowID = window.ID
				l.publish(false)
				break
			}
		}
	case "bounds":
		if !validSize(cmd.bounds.W, cmd.bounds.H) {
			return
		}
		measured := domain.Bounds{W: cmd.bounds.W, H: cmd.bounds.H}
		if measured == l.measured {
			return
		}
		l.measured = measured
		l.place()
		l.publish(false)
	}
}

func (l *controllerLoop) request() { l.dirty = true; l.query() }

func (l *controllerLoop) query() {
	if l.querying || !l.dirty || l.candidate == nil || !l.enabled() {
		return
	}
	l.querying = true
	l.dirty = false
	item, settings := *l.candidate, l.settings
	session, generation := l.session, l.generation
	c := l.controller
	go func() {
		result := queryWindows(c.deps, settings, item)
		result.session = session
		result.generation = generation
		select {
		case c.results <- result:
		case <-l.ctx.Done():
		}
	}()
}

func queryWindows(deps Deps, settings config.Settings, item Item) windowResult {
	result := windowResult{}
	if deps.Env == nil || deps.Windows == nil {
		result.err = errors.New("dock: window environment unavailable")
		return result
	}
	for _, screen := range deps.Env.Screens() {
		if screen.ID == item.ScreenID {
			result.screen = screen.Visible
			if result.screen.Area() == 0 {
				result.screen = screen.Bounds
			}
			break
		}
	}
	if result.screen.Area() == 0 {
		result.err = errors.New("dock: display unavailable")
		return result
	}
	ctx := filter.Context{ActiveAppID: deps.Env.ActiveApp(), ActiveSpaceID: deps.Env.ActiveSpace(), ActiveScreenID: deps.Env.ActiveScreen(), CursorScreenID: deps.Env.CursorScreen(), SelfBundleID: deps.SelfBundleID}
	app := domain.App{ID: item.AppID, Name: item.Title, BundleID: item.BundleID}
	if item.AppID <= 0 {
		result.excluded = excludedPinnedApp(item, settings.Filters, ctx.SelfBundleID)
		result.emptyReason = "notRunning"
		return result
	}
	if deps.Apps != nil {
		apps, err := deps.Apps.Apps()
		if err != nil {
			result.err = err
			return result
		}
		found := false
		for _, candidate := range apps {
			if candidate.ID == item.AppID && (item.BundleID == "" || candidate.BundleID == item.BundleID) {
				app = candidate
				found = true
				break
			}
		}
		if !found {
			result.emptyReason = "notRunning"
			result.excluded = excludedPinnedApp(item, settings.Filters, ctx.SelfBundleID)
			return result
		}
	}
	all, err := deps.Windows.Windows()
	if err != nil {
		result.err = err
		return result
	}
	raw := make([]domain.Window, 0)
	seen := make(map[domain.WindowID]bool)
	for _, window := range all {
		if window.AppID == item.AppID && window.ID != 0 && !seen[window.ID] {
			raw = append(raw, window)
			seen[window.ID] = true
			app.Hidden = app.Hidden || window.Hidden
		}
	}
	mode := settings.Order
	if settings.Dock.Scope.Order.Valid() {
		mode = settings.Dock.Scope.Order
	}
	result.windows = order.SendToBack(order.Sort(filter.Apply(raw, settings.Filters, settings.Dock.Scope, ctx), mode), settings.Filters)
	rawCount := len(raw)
	if len(result.windows) == 0 {
		result.emptyReason = "noWindows"
		if len(raw) > 0 {
			result.emptyReason = "filtered"
		}
		if deps.AppWindows != nil {
			switch deps.AppWindows.AppWindowPresence(item.AppID) {
			case platform.WindowsNone:
				rawCount = 0
				result.emptyReason = "noWindows"
			case platform.WindowsPresent:
				rawCount = max(1, rawCount)
				result.emptyReason = "filtered"
			default:
				rawCount = -1
				result.emptyReason = "unavailable"
			}
		}
	}
	result.excluded = !filter.AppAllowed(app, rawCount, settings.Filters, settings.Dock.Scope, ctx)
	return result
}

func excludedPinnedApp(item Item, filters config.Filters, self string) bool {
	if self != "" && strings.EqualFold(item.BundleID, self) {
		return true
	}
	for _, entry := range filters.AppBlacklist {
		if entry.Match != "" && (strings.EqualFold(entry.Match, item.BundleID) || strings.EqualFold(entry.Match, item.Title)) {
			return true
		}
	}
	return false
}

func (l *controllerLoop) accept(result windowResult) {
	if !l.enabled() || l.admission != l.controller.AdmissionEpoch() || l.candidate == nil || result.session != l.session || result.generation != l.generation {
		return
	}
	if result.err != nil || result.excluded {
		blocked := copyItem(l.candidate)
		l.reset()
		l.blocked = blocked
		return
	}
	selected := l.state.SelectedWindowID
	if !slices.ContainsFunc(result.windows, func(w domain.Window) bool { return w.ID == selected }) {
		selected = 0
		if len(result.windows) > 0 {
			selected = result.windows[0].ID
		}
	}
	l.screen = result.screen
	l.state = State{AdmissionEpoch: l.admission, Session: l.session, Item: *l.candidate, Windows: result.windows, SelectedWindowID: selected, Appearance: l.settings.Dock.Appearance, CardSpacingPx: l.settings.Dock.CardSpacingPx, EmptyReason: result.emptyReason}
	l.place()
	first := !l.shown
	l.shown = true
	l.publish(first)
}

func (l *controllerLoop) place() {
	w, h := panelSize(len(l.state.Windows), l.settings.Dock.Appearance, l.settings.Dock.CardSpacingPx)
	if validSize(l.measured.W, l.measured.H) {
		w, h = l.measured.W, l.measured.H
	}
	l.state.Bounds = PlacePanel(l.state.Item.Bounds, l.screen, l.state.Item.Edge, w, h, 8)
}

func (l *controllerLoop) publish(first bool) {
	if l.admission != l.controller.AdmissionEpoch() {
		return
	}
	if view := l.controller.deps.View; view != nil {
		state := l.state
		state.Windows = slices.Clone(state.Windows)
		if first {
			view.Show(state)
		} else {
			view.Update(state)
		}
	}
	l.publishPointer()
}

func validSize(w, h float64) bool {
	return w > 0 && h > 0 && !math.IsInf(w, 0) && !math.IsInf(h, 0) && !math.IsNaN(w) && !math.IsNaN(h)
}

func panelSize(count int, a config.Appearance, spacing int) (float64, float64) {
	if count == 0 {
		return 320, 120
	}
	columns := min(max(1, a.MaxColumns), count)
	rows := min(max(1, a.MaxRows), (count+columns-1)/columns)
	cardWidth := float64(a.ThumbnailMaxPx)
	cardHeight := cardWidth*0.625 + 48
	if a.Style == config.StyleTitles {
		columns = 1
		rows = min(count, max(1, a.MaxRows))
		cardWidth = max(240, float64(a.TitleMaxWidthPx))
		cardHeight = 52
	}
	return 24 + float64(columns)*(cardWidth+12) + float64(columns-1)*float64(spacing),
		56 + float64(rows)*cardHeight + float64(rows-1)*float64(spacing)
}

func inflate(b domain.Bounds, padding float64) domain.Bounds {
	return domain.Bounds{X: b.X - padding, Y: b.Y - padding, W: b.W + 2*padding, H: b.H + 2*padding}
}

// The bridge joins nearest edges at icon width. Unlike a bounding-box union it
// does not turn empty space beside a wide panel into a sticky hover region.
func overDockSurface(x, y float64, icon, panel domain.Bounds, edge string, padding float64) bool {
	if icon.ContainsPoint(x, y) || panel.ContainsPoint(x, y) {
		return true
	}
	var bridge domain.Bounds
	switch edge {
	case "left":
		top := max(icon.Y, panel.Y)
		bottom := min(icon.Y+icon.H, panel.Y+panel.H)
		bridge = domain.Bounds{X: icon.X + icon.W, Y: top, W: max(0, panel.X-icon.X-icon.W), H: max(0, bottom-top)}
	case "right":
		top := max(icon.Y, panel.Y)
		bottom := min(icon.Y+icon.H, panel.Y+panel.H)
		bridge = domain.Bounds{X: panel.X + panel.W, Y: top, W: max(0, icon.X-panel.X-panel.W), H: max(0, bottom-top)}
	default:
		left := max(icon.X, panel.X)
		right := min(icon.X+icon.W, panel.X+panel.W)
		bridge = domain.Bounds{X: left, Y: panel.Y + panel.H, W: max(0, right-left), H: max(0, icon.Y-panel.Y-panel.H)}
	}
	return bridge.Area() > 0 && inflate(bridge, max(0, padding)).ContainsPoint(x, y)
}

func (l *controllerLoop) publishPointer() {
	view, ok := l.controller.deps.View.(PointerView)
	if !ok || !l.shown || !l.enabled() || l.last == nil || l.state.Session != l.session || l.state.AdmissionEpoch != l.admission || l.admission != l.controller.AdmissionEpoch() {
		return
	}
	x, y := l.last.PointerX, l.last.PointerY
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return
	}
	bounds := l.state.Bounds
	next := PointerState{Session: l.state.Session, AdmissionEpoch: l.admission, X: x - bounds.X, Y: y - bounds.Y, Inside: bounds.ContainsPoint(x, y)}
	previous := l.pointer
	if previous.Session == next.Session && previous.AdmissionEpoch == next.AdmissionEpoch {
		if previous.X == next.X && previous.Y == next.Y && previous.Inside == next.Inside && l.pointerBounds == bounds {
			return
		}
		next.Sequence = previous.Sequence + 1
	} else {
		next.Sequence = 1
	}
	l.pointer = next
	l.pointerBounds = bounds
	view.Pointer(next)
}
