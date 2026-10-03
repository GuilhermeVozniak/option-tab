package main

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/dock"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

var errSwitcherGestureRetired = errors.New("switcher gesture presentation retired")

type SwitcherGestureCapabilities struct {
	Available bool `json:"available"`
}
type switcherGestureError struct {
	Session  uint64 `json:"session"`
	Revision uint64 `json:"revision"`
	Message  string `json:"message"`
}

// No state lock crosses AppKit or AX calls. publish serializes native policy
// writes; the single retirement worker coalesces lifecycle invalidations.
// In particular, final native action guards never acquire App.viewMu, since
// Show can hold it while synchronously fitting the native window.
type appSwitcherGestures struct {
	mu                                                             sync.Mutex
	attach                                                         sync.Mutex
	publish                                                        sync.Mutex
	source                                                         platform.SwitcherWheel
	key                                                            switcher.State
	session, stateRevision, sequence, epoch, revision, policyEpoch uint64
	presented, closed                                              bool
	regions                                                        []platform.PreviewRegion
	identities                                                     map[domain.WindowID]platform.AutomationWindowIdentity
	recognizer                                                     *dock.PanelGestureRecognizer
	pending                                                        struct{ session, revision, gesture uint64 }
	cancel                                                         context.CancelFunc
	wake                                                           chan struct{}
	done                                                           chan struct{}
	workers                                                        sync.WaitGroup
}

func (a *App) GetSwitcherGestureCapabilities() SwitcherGestureCapabilities {
	g := &a.switcherGestures
	g.attach.Lock()
	defer g.attach.Unlock()
	g.mu.Lock()
	source, closed := g.source, g.closed
	g.mu.Unlock()
	if closed {
		return SwitcherGestureCapabilities{}
	}
	if source != nil {
		select {
		case <-source.Done():
			return SwitcherGestureCapabilities{}
		default:
			return SwitcherGestureCapabilities{true}
		}
	}
	host, ok := a.platform.(platform.SwitcherWheelHost)
	if !ok {
		return SwitcherGestureCapabilities{}
	}
	if _, ok := a.platform.(platform.GuardedPointerPerformer); !ok {
		return SwitcherGestureCapabilities{}
	}
	if _, ok := a.platform.(platform.AutomationIdentitySource); !ok {
		return SwitcherGestureCapabilities{}
	}
	native := a.overlay.native()
	if native == nil {
		return SwitcherGestureCapabilities{}
	}
	source, err := host.CreateSwitcherWheel(native)
	if err != nil || source == nil {
		return SwitcherGestureCapabilities{}
	}
	if !a.installSwitcherGestureSource(source) {
		_ = source.Close()
		return SwitcherGestureCapabilities{}
	}
	return SwitcherGestureCapabilities{true}
}

func (a *App) installSwitcherGestureSource(source platform.SwitcherWheel) bool {
	g := &a.switcherGestures
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.source != nil || source == nil {
		return false
	}
	g.source = source
	g.wake = make(chan struct{}, 1)
	g.done = make(chan struct{})
	go a.runSwitcherGestureRetirement(source, g.wake, g.done)
	return true
}

func (g *appSwitcherGestures) retireLocked() {
	g.epoch++
	g.policyEpoch = 0
	g.regions = nil
	g.identities = nil
	if g.recognizer != nil {
		g.recognizer.Reset()
	}
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	if g.wake != nil {
		select {
		case g.wake <- struct{}{}:
		default:
		}
	}
}

func (a *App) invalidateSwitcherGestures() {
	g := &a.switcherGestures
	g.mu.Lock()
	defer g.mu.Unlock()
	g.retireLocked()
	g.presented = false
}

// Geometry invalidation permits a fresh report for the same displayed state.
func (a *App) retireSwitcherGestureGeometry() {
	g := &a.switcherGestures
	g.mu.Lock()
	g.retireLocked()
	g.mu.Unlock()
}

func (a *App) stopSwitcherGestures() {
	g := &a.switcherGestures
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		g.closed = true
		g.presented = false
		g.retireLocked()
	}
}

func (a *App) runSwitcherGestureRetirement(source platform.SwitcherWheel, wake <-chan struct{}, done chan struct{}) {
	defer close(done)
	for range wake {
		g := &a.switcherGestures
		g.publish.Lock()
		g.mu.Lock()
		closed, retired := g.closed, g.policyEpoch == 0
		g.mu.Unlock()
		if closed {
			_ = source.Close()
			g.publish.Unlock()
			// Close does not block AppKit callbacks; joining lives on this worker.
			<-source.Done()
			g.workers.Wait()
			return
		}
		if retired {
			_ = source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{}, nil)
		}
		g.publish.Unlock()
	}
}

// Called under viewMu immediately before publishing a Show/Update packet.
// Selection alone does not change input authority; entries, search, layout,
// settings and gallery changes do. Geometry changes are admitted separately.
func (a *App) presentSwitcherGestures(st switcher.State) {
	if !a.switcherVisible || a.visibleSwitcherSession != st.Session || a.prefsOpen || a.sessionInactive || a.settingsSnapshot().Behavior.Paused {
		a.invalidateSwitcherGestures()
		return
	}
	key := st
	key.Revision = 0
	key.Selected = 0
	key.SelectedWindowID = 0
	key.ActionBindings = nil
	key.Apps = nil
	key.Entries = slices.Clone(st.Entries)
	g := &a.switcherGestures
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	if !g.presented || !reflect.DeepEqual(g.key, key) {
		g.retireLocked()
	}
	if g.session != st.Session {
		g.sequence = 0
	}
	g.key = key
	g.session = st.Session
	g.stateRevision = st.Revision
	g.presented = true
}

func (a *App) SetSwitcherGestureRegions(session, stateRevision, sequence uint64, regions []DockPreviewRegion) error {
	g := &a.switcherGestures
	g.mu.Lock()
	needsSource := g.source == nil && !g.closed
	g.mu.Unlock()
	if needsSource && !a.GetSwitcherGestureCapabilities().Available {
		return errors.New("switcher gestures are unavailable")
	}
	if session == 0 || stateRevision == 0 || sequence == 0 || len(regions) > 256 {
		return errors.New("invalid switcher gesture regions")
	}
	g.mu.Lock()
	if g.closed || !g.presented || g.session != session || g.stateRevision != stateRevision || sequence <= g.sequence || g.source == nil {
		g.mu.Unlock()
		return errSwitcherGestureRetired
	}
	g.sequence = sequence
	source, key := g.source, g.key
	valid := make(map[domain.WindowID]domain.AppID, len(key.Entries))
	for _, entry := range key.Entries {
		valid[entry.WindowID] = entry.AppID
	}
	copied := make([]platform.PreviewRegion, len(regions))
	invalid := false
	for i, region := range regions {
		b := region.Bounds
		copied[i] = platform.PreviewRegion{WindowID: region.WindowID, AppID: region.AppID, Bounds: domain.Bounds{X: b.X, Y: b.Y, W: b.W, H: b.H}}
		invalid = invalid || region.WindowID == 0 || region.AppID <= 0 || valid[region.WindowID] != region.AppID || b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 || nonfinite(b.X, b.Y, b.W, b.H, b.X+b.W, b.Y+b.H)
	}
	unchanged := !invalid && g.policyEpoch != 0 && reflect.DeepEqual(g.regions, copied)
	identities := g.identities
	if !unchanged {
		g.retireLocked()
	}
	epoch := g.epoch
	g.mu.Unlock()
	current := func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return !g.closed && g.presented && g.epoch == epoch && g.session == session && g.stateRevision == stateRevision && g.sequence == sequence
	}
	if !unchanged {
		// A moved/removed card retires old hit areas BEFORE preparing identities.
		// Preparation never holds the native publication lock or delays shutdown.
		g.publish.Lock()
		if current() {
			_ = source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{}, nil)
		}
		g.publish.Unlock()
		if invalid {
			return errors.New("invalid switcher gesture region")
		}
		identitySource, ok := a.platform.(platform.AutomationIdentitySource)
		if !ok {
			return errors.New("switcher gesture identity unavailable")
		}
		prior := identities
		identities = make(map[domain.WindowID]platform.AutomationWindowIdentity)
		deadline := time.Now().Add(time.Second)
		for _, region := range copied {
			if !current() {
				return errSwitcherGestureRetired
			}
			if time.Now().After(deadline) {
				return errors.New("switcher gesture identity capture timed out")
			}
			if _, exists := identities[region.WindowID]; exists {
				continue
			}
			// WindowIdentity captures only CG owner/PID/start time. Full AX validation
			// is bounded to the one selected action, never repeated for a whole grid.
			id, err := identitySource.WindowIdentity(region.WindowID)
			if err != nil {
				return err
			}
			if id.ID != region.WindowID || id.Process.PID != region.AppID || id.Process.StartSeconds == 0 || id.Process.StartMicros >= 1_000_000 {
				return errors.New("switcher gesture window identity changed")
			}
			if previous, ok := prior[region.WindowID]; ok && previous != id {
				return errors.New("switcher gesture window identity changed")
			}
			identities[region.WindowID] = id
		}
		if time.Now().After(deadline) {
			return errors.New("switcher gesture identity capture timed out")
		}
	}
	g.publish.Lock()
	defer g.publish.Unlock()
	g.mu.Lock()
	if g.closed || !g.presented || g.epoch != epoch || g.session != session || g.stateRevision != stateRevision || g.sequence != sequence {
		g.mu.Unlock()
		return errSwitcherGestureRetired
	}
	if !unchanged {
		g.policyEpoch = epoch
		g.revision++
		g.regions = copied
		g.identities = identities
		input := config.DockInputSettings{SwipeAwayFromDock: key.SwipeUpAction, SwipeTowardDock: key.SwipeDownAction, SwipePrevious: config.PointerNone, SwipeNext: config.PointerNone}
		g.recognizer = dock.NewPanelGestureRecognizer("bottom", input)
		g.recognizer.SetPresentation(session, g.revision)
	}
	revision := g.revision
	enabled := len(copied) > 0 && (key.SwipeUpAction != config.PointerNone || key.SwipeDownAction != config.PointerNone)
	g.mu.Unlock()
	err := source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{Session: session, Revision: revision, Enabled: enabled, Regions: copied}, func(e platform.DockPanelWheelEvent) { a.handleSwitcherWheel(source, e) })
	// A newer identical report may already be waiting for publication. It owns
	// the same authority; an older successful reply must not retire that owner.
	g.mu.Lock()
	authorityCurrent := !g.closed && g.presented && g.session == session && g.epoch == epoch && g.policyEpoch == epoch
	g.mu.Unlock()
	if err != nil || !authorityCurrent {
		_ = source.SetDockPanelWheelPolicy(platform.DockPanelWheelPolicy{}, nil)
		g.mu.Lock()
		if g.policyEpoch == epoch {
			g.retireLocked()
		}
		g.mu.Unlock()
		if err != nil {
			return err
		}
		return errSwitcherGestureRetired
	}
	return nil
}

func (a *App) handleSwitcherWheel(source platform.SwitcherWheel, event platform.DockPanelWheelEvent) {
	g := &a.switcherGestures
	g.mu.Lock()
	current := !g.closed && g.presented && g.source == source && g.policyEpoch != 0 && event.Session == g.session && event.Revision == g.revision
	var intent *dock.PanelGestureIntent
	if current && g.recognizer != nil {
		intent = g.recognizer.Step(event)
	}
	pending := g.pending.session == event.Session && g.pending.revision == event.Revision && g.pending.gesture == event.GestureID
	if intent == nil || g.pending.gesture != 0 {
		g.mu.Unlock()
		if wheelTerminal(event) && !pending {
			source.CompleteDockPanelWheelGesture(event.Session, event.Revision, event.GestureID)
		}
		if current && event.Reason == "native wheel queue overflow" {
			a.emitSwitcherGestureError(event.Session, event.Revision, event.Reason)
		}
		return
	}
	id, found := g.identities[intent.WindowID]
	if !found || id.Process.PID != intent.AppID {
		g.mu.Unlock()
		source.CompleteDockPanelWheelGesture(event.Session, event.Revision, event.GestureID)
		return
	}
	epoch := g.policyEpoch
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	g.cancel = cancel
	g.pending = struct{ session, revision, gesture uint64 }{intent.Session, intent.Revision, intent.GestureID}
	g.workers.Add(1)
	g.mu.Unlock()
	go func() {
		defer g.workers.Done()
		defer func() {
			cancel()
			g.mu.Lock()
			if g.pending.session == intent.Session && g.pending.revision == intent.Revision && g.pending.gesture == intent.GestureID {
				g.pending = struct{ session, revision, gesture uint64 }{}
				g.cancel = nil
			}
			g.mu.Unlock()
			source.CompleteDockPanelWheelGesture(intent.Session, intent.Revision, intent.GestureID)
		}()
		err := a.executeSwitcherGesture(ctx, source, *intent, epoch, id)
		if err != nil && !errors.Is(err, errSwitcherGestureRetired) && !errors.Is(err, context.Canceled) {
			a.emitSwitcherGestureError(intent.Session, intent.Revision, err.Error())
		}
		if err == nil && a.currentSwitcherGesture(source, *intent, epoch) && a.controller != nil {
			a.controller.Refresh()
		}
	}()
}

func (a *App) currentSwitcherGesture(source platform.SwitcherWheel, intent dock.PanelGestureIntent, epoch uint64) bool {
	g := &a.switcherGestures
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.closed && g.presented && g.source == source && g.policyEpoch == epoch && g.epoch == epoch && g.session == intent.Session && g.revision == intent.Revision
}

func (a *App) executeSwitcherGesture(ctx context.Context, source platform.SwitcherWheel, intent dock.PanelGestureIntent, epoch uint64, id platform.AutomationWindowIdentity) error {
	guard := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !a.currentSwitcherGesture(source, intent, epoch) {
			return errSwitcherGestureRetired
		}
		if !source.ValidateDockPanelWheelGesture(intent.Session, intent.Revision, intent.GestureID) {
			return errSwitcherGestureRetired
		}
		if !a.currentSwitcherGesture(source, intent, epoch) {
			return errSwitcherGestureRetired
		}
		return ctx.Err()
	}
	if err := guard(); err != nil {
		return err
	}
	identities, ok := a.platform.(platform.AutomationIdentitySource)
	if !ok || !identities.WindowIdentityCurrent(id) {
		return errors.New("switcher gesture window identity changed")
	}
	performer, ok := a.platform.(platform.GuardedPointerPerformer)
	if !ok {
		return errors.New("switcher gesture action unavailable")
	}
	return performer.PerformPointerAction(ctx, string(intent.Action), id, guard)
}

func (a *App) emitSwitcherGestureError(session, revision uint64, message string) {
	g := &a.switcherGestures
	g.mu.Lock()
	current := !g.closed && g.presented && g.policyEpoch != 0 && g.session == session && g.revision == revision
	frontendRevision := g.stateRevision
	g.mu.Unlock()
	if current {
		a.emit("switcher:gestureError", switcherGestureError{session, frontendRevision, message})
	}
}
