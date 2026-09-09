package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type (
	launcherInteractionJob struct {
		ctx                 context.Context
		kind                string
		admission, sequence uint64
		text                string
		modifiers           uint64
		composing, enabled  bool
		done                chan error
	}
	launcherInteractionOwner struct {
		app               *App
		runtime           *appLauncherRuntime
		host              *appLauncherHost
		window            *dockWindow
		edge              string
		capabilityReason  string
		reorder           bool
		authority         launcher.InteractionAuthority
		config            config.LauncherInteractions
		ctx               context.Context
		cancel            context.CancelFunc
		done, predecessor <-chan struct{}
		finish            chan struct{}
		live              atomic.Bool
		admission         atomic.Uint64
		state             LauncherInteractionState // viewMu
		panel             platform.LauncherPanel
		token             uint64 // published under viewMu, immutable thereafter
		reducer           launcher.GestureReducer
		reducerAdmission  uint64
		letters           launcher.LetterCycle
		letterAdmission   uint64
		haptic            launcher.HapticLimiter
		inputSequence     uint64
		jobs              chan launcherInteractionJob
		eventsMu          sync.Mutex
		events            []platform.LauncherGestureEvent
		overflow          *platform.LauncherGestureEvent
		wake              chan struct{}
	}
)

func interactionError(code string) error { return errors.New("launcher interaction: " + code) }
func (a *App) GetLauncherInteractionCapabilities() LauncherInteractionCapabilities {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.launcherInteractionCapabilitiesLocked()
}

func (a *App) launcherInteractionCapabilitiesLocked() LauncherInteractionCapabilities {
	if a.launcher != nil && a.launcher.interactionCapabilities != nil {
		return a.launcher.interactionCapabilities()
	}
	_, source := a.platform.(platform.LauncherPanelHost)
	_, haptic := a.platform.(platform.HapticFeedback)
	return LauncherInteractionCapabilities{GestureAvailable: source, HapticsAvailable: haptic, Reason: "deliveryUnverified"}
}

func (a *App) syncLauncherInteractionsLocked() {
	if a.launcher == nil {
		return
	}
	for _, host := range a.launcher.hosts {
		current := host.interaction
		if current != nil {
			latest, err := a.launcher.core.ValidateInteraction(current.authority)
			if err == nil && current.live.Load() && a.launcherAllowedLocked() {
				current.state.PresentationRevision = latest.Revision
				a.publishLauncherInteractionLocked(current, current.state.Reason)
				continue
			}
			a.retireLauncherInteractionLocked(host)
		}
		if !a.launcherAllowedLocked() || !host.presentation.Visible || time.Now().Before(host.interactionRetryAt) || len(a.launcher.interactionOwners) >= 16 {
			continue
		}
		var cfg config.LauncherInteractions
		for _, p := range a.settingsSnapshot().ReplacementDock.Profiles {
			if p.ID == host.presentation.ProfileID && p.Interactions != nil {
				cfg = *p.Interactions
			}
		}
		if !cfg.Enabled {
			continue
		}
		authority, err := a.launcher.core.CaptureInteraction(host.presentation.Scope)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		finish := make(chan struct{})
		o := &launcherInteractionOwner{app: a, runtime: a.launcher, host: host, window: host.window, authority: authority, edge: host.presentation.Edge, reorder: host.presentation.RuntimeReorder, config: cfg, ctx: ctx, cancel: cancel, done: finish, finish: finish, predecessor: a.launcher.interactionDrains[host.presentation.DisplayUUID], jobs: make(chan launcherInteractionJob, 8), wake: make(chan struct{}, 1)}
		a.launcher.nextInteraction++
		o.admission.Store(a.launcher.nextInteraction)
		o.live.Store(true)
		p := host.presentation
		o.capabilityReason = a.launcherInteractionCapabilitiesLocked().Reason
		o.state = LauncherInteractionState{Epoch: p.Epoch, DisplayUUID: p.DisplayUUID, Session: p.Session, PresentationRevision: p.Revision, Admission: o.admission.Load(), Visible: true, Configured: cfg, LauncherInteractionCapabilities: a.launcherInteractionCapabilitiesLocked()}
		host.interaction = o
		if a.launcher.interactionOwners == nil {
			a.launcher.interactionOwners = make(map[*launcherInteractionOwner]struct{})
		}
		a.launcher.interactionOwners[o] = struct{}{}
		go o.run()
	}
}

func (a *App) retireLauncherInteractionLocked(host *appLauncherHost) {
	o := host.interaction
	if o == nil {
		return
	}
	host.interaction = nil
	host.interactionDrain = o.done
	if o.runtime.interactionDrains == nil {
		o.runtime.interactionDrains = make(map[string]<-chan struct{})
	}
	o.runtime.interactionDrains[o.authority.Scope.DisplayUUID] = o.done
	o.runtime.nextInteraction++
	o.admission.Store(o.runtime.nextInteraction)
	o.state.Admission = o.admission.Load()
	o.live.Store(false)
	o.cancel()
	o.state.Visible = false
	o.state.KeyboardMode = false
	a.publishLauncherInteractionLocked(o, "stale")
}

func (a *App) publishLauncherInteractionLocked(o *launcherInteractionOwner, reason string) {
	o.state.Sequence++
	if reason == "" {
		reason = o.capabilityReason
	}
	o.state.Reason = reason
	a.emit("launcher:interaction", o.state)
}

func (a *App) GetLauncherInteractionState(session uint64) LauncherInteractionState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcher == nil {
		return LauncherInteractionState{}
	}
	h := a.launcher.hosts[session]
	if h == nil || h.interaction == nil {
		return LauncherInteractionState{}
	}
	o := h.interaction
	if !o.live.Load() {
		return LauncherInteractionState{}
	}
	latest, err := a.launcher.core.ValidateInteraction(o.authority)
	if err != nil {
		return LauncherInteractionState{}
	}
	o.state.PresentationRevision = latest.Revision
	return o.state
}

func (o *launcherInteractionOwner) logical() bool {
	if !o.live.Load() || o.ctx.Err() != nil {
		return false
	}
	_, err := o.runtime.core.ValidateInteraction(o.authority)
	return err == nil
}

func (o *launcherInteractionOwner) guard(admission uint64, keyboard bool) error {
	if !o.logical() || o.admission.Load() != admission {
		return interactionError("stale")
	}
	a := o.app
	a.viewMu.Lock()
	current := a.launcher == o.runtime && a.launcherAllowedLocked() && o.host.interaction == o && o.runtime.hosts[o.authority.Scope.Session] == o.host && o.host.window == o.window
	panel, ok := o.window.wheelPanel().(platform.LauncherPanel)
	current = current && ok && panel.LauncherToken() == o.token && o.token != 0
	key := o.state.KeyboardMode
	a.viewMu.Unlock()
	if !current {
		return interactionError("stale")
	}
	validator, ok := panel.(platform.LauncherPanelValidator)
	if !ok || validator.ValidateLauncherPanel(o.ctx, o.authority.Scope.DisplayUUID) != nil {
		return interactionError("stale")
	}
	if keyboard {
		v, ok := panel.(platform.LauncherKeyboardValidator)
		s := o.authority.Scope
		if !key || !ok || !v.ValidateLauncherKeyboard(s.Epoch, s.Session, s.Revision, admission) {
			return interactionError("stale")
		}
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	latestPanel, ok := o.window.wheelPanel().(platform.LauncherPanel)
	if !o.logical() || o.admission.Load() != admission || a.launcher != o.runtime || !a.launcherAllowedLocked() || o.host.interaction != o || !ok || latestPanel.LauncherToken() != o.token {
		return interactionError("stale")
	}
	return nil
}

func (o *launcherInteractionOwner) queue(e platform.LauncherGestureEvent) {
	if !o.live.Load() || e.Admission != o.admission.Load() {
		return
	}
	o.eventsMu.Lock()
	if len(o.events) < 128 {
		o.events = append(o.events, e)
	} else {
		copy := e
		o.overflow = &copy
	}
	o.eventsMu.Unlock()
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func interactionAction(v string) string {
	switch v {
	case "showPreview":
		return "show"
	case "hidePreview":
		return "hide"
	}
	return v
}

func (o *launcherInteractionOwner) install() error {
	s := o.authority.Scope
	c := o.config
	o.reducer = launcher.GestureReducer{}
	o.reducerAdmission = o.reducer.SetScope(s, launcher.GesturePolicy{Enabled: c.Enabled, Edge: o.edge, Pinch: interactionAction(c.PinchAction), Primary: c.PrimaryAction, Toward: interactionAction(c.TowardAction)})
	o.letters = launcher.LetterCycle{}
	o.letterAdmission = o.letters.SetScope(s, c.Enabled && c.LetterNavigation)
	source, ok := o.panel.(platform.LauncherGestureSource)
	if !ok {
		return nil
	}
	o.app.viewMu.Lock()
	caps := o.state.LauncherInteractionCapabilities
	o.app.viewMu.Unlock()
	p := platform.LauncherGesturePolicy{Epoch: s.Epoch, Session: s.Session, Revision: s.Revision, Admission: o.admission.Load(), DisplayUUID: s.DisplayUUID, Enabled: c.Enabled && ((c.PreciseScroll && caps.GestureAvailable) || (c.Pinch && caps.PinchAvailable) || (c.Swipe && caps.SwipeAvailable)), Scroll: c.PreciseScroll && caps.GestureAvailable, Magnify: c.Pinch && caps.PinchAvailable, Swipe: c.Swipe && caps.SwipeAvailable, Bounds: domain.Bounds{W: o.authority.Bounds.W, H: o.authority.Bounds.H}}
	return source.SetLauncherGesturePolicy(p, o.queue)
}

func (o *launcherInteractionOwner) run() {
	defer func() {
		o.app.viewMu.Lock()
		if o.host.interaction == o {
			o.host.interactionRetryAt = time.Now().Add(time.Second)
			o.app.retireLauncherInteractionLocked(o.host)
		}
		delete(o.runtime.interactionOwners, o)
		if o.runtime.interactionDrains[o.authority.Scope.DisplayUUID] == o.done {
			delete(o.runtime.interactionDrains, o.authority.Scope.DisplayUUID)
		}
		close(o.finish)
		o.app.viewMu.Unlock()
	}()
	if o.predecessor != nil {
		select {
		case <-o.predecessor:
		case <-o.ctx.Done():
			<-o.predecessor
			return
		}
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		o.app.viewMu.Lock()
		p, ok := o.window.wheelPanel().(platform.LauncherPanel)
		if ok && o.token == 0 && o.logical() && o.host.interaction == o {
			o.panel = p
			o.token = p.LauncherToken()
		}
		o.app.viewMu.Unlock()
		if !o.logical() {
			return
		}
		if o.token != 0 && o.guard(o.admission.Load(), false) == nil {
			break
		}
		select {
		case <-o.ctx.Done():
			return
		case <-deadline.C:
			o.fail("unavailable")
			return
		case <-tick.C:
		}
	}
	tick.Reset(100 * time.Millisecond)
	defer o.disable()
	if err := o.guard(o.admission.Load(), false); err != nil {
		return
	}
	if err := o.install(); err != nil {
		o.fail("unavailable")
	}
	o.app.viewMu.Lock()
	if o.live.Load() {
		o.app.publishLauncherInteractionLocked(o, o.state.Reason)
	}
	o.app.viewMu.Unlock()
	for {
		select {
		case <-o.ctx.Done():
			return
		case job := <-o.jobs:
			err := o.job(job)
			if err == nil {
				o.fail("")
			}
			select {
			case job.done <- err:
			default:
			}
		case <-o.wake:
			o.eventsMu.Lock()
			events, overflow := o.events, o.overflow
			o.events = nil
			o.overflow = nil
			o.eventsMu.Unlock()
			if overflow != nil {
				for _, e := range events {
					o.ack(e)
				}
				o.ack(*overflow)
				o.fail("busy")
			} else {
				for _, e := range events {
					o.event(e)
				}
			}
		case <-tick.C:
			o.app.viewMu.Lock()
			keyboard := o.state.KeyboardMode
			o.app.viewMu.Unlock()
			if keyboard && o.guard(o.admission.Load(), true) != nil {
				o.exitKeyboard()
			}
		}
	}
}

func (o *launcherInteractionOwner) fail(reason string) {
	o.app.viewMu.Lock()
	defer o.app.viewMu.Unlock()
	if o.live.Load() && o.host.interaction == o {
		o.app.publishLauncherInteractionLocked(o, reason)
	}
}

func (o *launcherInteractionOwner) ack(e platform.LauncherGestureEvent) {
	if ack, ok := o.panel.(platform.LauncherGestureAcknowledger); ok {
		ack.CompleteLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID)
	}
}

func (o *launcherInteractionOwner) disable() {
	if o.panel == nil {
		return
	}
	s := o.authority.Scope
	if k, ok := o.panel.(platform.LauncherKeyboardSource); ok {
		_ = k.SetLauncherKeyboardPolicy(platform.LauncherKeyboardPolicy{Epoch: s.Epoch, Session: s.Session, Revision: s.Revision, Admission: o.admission.Load(), DisplayUUID: s.DisplayUUID}, nil)
	}
	if g, ok := o.panel.(platform.LauncherGestureSource); ok {
		_ = g.SetLauncherGesturePolicy(platform.LauncherGesturePolicy{Epoch: s.Epoch, Session: s.Session, Revision: s.Revision, Admission: o.admission.Load(), DisplayUUID: s.DisplayUUID, Bounds: domain.Bounds{W: o.authority.Bounds.W, H: o.authority.Bounds.H}}, nil)
	}
}
