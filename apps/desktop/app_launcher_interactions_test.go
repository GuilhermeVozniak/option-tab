package main

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type interactionTestPanel struct {
	physicalChecks atomic.Int32
	clock          atomic.Int64
	haptics        atomic.Int32
	*launcherIntegrationPanel
	mu           sync.Mutex
	policy       platform.LauncherGesturePolicy
	emit         func(platform.LauncherGestureEvent)
	acks         int
	key          platform.LauncherKeyboardPolicy
	rejectKey    bool
	keyInstalled func()
}

func (p *interactionTestPanel) SetLauncherGesturePolicy(v platform.LauncherGesturePolicy, emit func(platform.LauncherGestureEvent)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policy, p.emit = v, emit
	return nil
}

func (p *interactionTestPanel) ValidateLauncherGesture(e, s, r, a, g uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.policy
	return v.Enabled && v.Epoch == e && v.Session == s && v.Revision == r && v.Admission == a && g != 0
}

func (p *interactionTestPanel) CompleteLauncherGesture(_, _, _, _, _ uint64) {
	p.mu.Lock()
	p.acks++
	p.mu.Unlock()
}

func (p *interactionTestPanel) SetLauncherKeyboardPolicy(v platform.LauncherKeyboardPolicy, guard func() bool) error {
	if v.Enabled && (guard == nil || !guard()) {
		return context.Canceled
	}
	p.mu.Lock()
	p.key = v
	hook := p.keyInstalled
	p.mu.Unlock()
	if hook != nil && v.Enabled {
		hook()
	}
	return nil
}

func (p *interactionTestPanel) ValidateLauncherKeyboard(e, s, r, a uint64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.rejectKey && p.key.Enabled && p.key.Epoch == e && p.key.Session == s && p.key.Revision == r && p.key.Admission == a
}

func (p *interactionTestPanel) send(kind string, id uint64, delta float64) {
	p.mu.Lock()
	v, emit := p.policy, p.emit
	p.mu.Unlock()
	if emit != nil {
		emit(platform.LauncherGestureEvent{Epoch: v.Epoch, Session: v.Session, Revision: v.Revision, Admission: v.Admission, DisplayUUID: v.DisplayUUID, Sequence: id, GestureID: id, Timestamp: time.Now(), Kind: kind, Phase: "began", DeltaX: delta, Owned: true, Precise: true})
	}
}

type interactionTestHost struct{ panel *interactionTestPanel }

func (h interactionTestHost) CreateLauncherPanel(unsafe.Pointer, string) (platform.LauncherPanel, error) {
	return h.panel, nil
}

func interactionFixture(t *testing.T, cfg func(*config.LauncherInteractions), prepare ...func(*interactionTestPanel)) (*App, *launcherIntegrationPlatform, *interactionTestPanel, launcher.Presentation) {
	t.Helper()
	a, p, q, base := launcherIntegrationApp(t)
	panel := &interactionTestPanel{launcherIntegrationPanel: base}
	for _, preparePanel := range prepare {
		preparePanel(panel)
	}
	a.platform = interactionTestPlatform{p, panel}
	a.launcher.core = launcher.New(launcher.Deps{Environment: interactionTestEnvironment{p, panel}, Applications: p, Identities: p, View: appLauncherView{a}, Activate: a.activateLauncherTarget, Eligible: a.launcherAppEligible, Now: func() time.Time { return time.Now().Add(time.Duration(panel.clock.Load()) * time.Second) }})
	a.launcher.core.Suspend(true)
	a.launcherFactory = func(_ uint64, uuid string, style platform.LauncherPanelStyle, failed func()) *dockWindow {
		w := &dockFakeWindow{}
		w.native = unsafe.Pointer(new(int))
		d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, launcherPanelHostAdapter{source: interactionTestHost{panel}, uuid: uuid, style: style})
		d.onFailure = failed
		return d
	}
	a.launcher.interactionCapabilities = func() LauncherInteractionCapabilities {
		return LauncherInteractionCapabilities{GestureAvailable: true, PinchAvailable: true, SwipeAvailable: true, LetterInputAvailable: true}
	}
	s := a.settingsSnapshot()
	c := config.DefaultLauncherInteractions()
	c.Enabled = true
	c.PreciseScroll = true
	c.Swipe = true
	c.LetterNavigation = true
	cfg(c)
	interactionsEnabled := c.Enabled
	s.ReplacementDock.Profiles[0].Interactions = c
	s.ReplacementDock.Profiles[0].Widgets[0].Enabled = true
	s.ReplacementDock.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	s.ReplacementDock.Profiles[0].Items = []config.LauncherItem{{ID: "alpha", Kind: "link", Label: "Alpha", URL: "https://example.com/a"}, {ID: "beta", Kind: "link", Label: "Beta", URL: "https://example.com/b"}}
	a.saveMu.Lock()
	err := a.saveSettingsLocked(s)
	a.saveMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	pres := launcherIntegrationVisible(t, a, q)
	if interactionsEnabled {
		launcherEventually(t, func() bool { panel.mu.Lock(); defer panel.mu.Unlock(); return panel.policy.Admission != 0 })
	}
	t.Cleanup(func() {
		a.viewMu.Lock()
		a.stopLauncherAdmissionLocked()
		var receipts []<-chan struct{}
		for _, h := range a.launcher.hosts {
			receipts = append(receipts, h.interactionDrain)
		}
		a.viewMu.Unlock()
		for _, r := range receipts {
			if r != nil {
				select {
				case <-r:
				case <-time.After(time.Second):
					t.Error("interaction did not drain")
				}
			}
		}
	})
	return a, p, panel, pres
}

func keyboardInteraction(a *App, s LauncherInteractionState, on bool) error {
	return a.SetLauncherKeyboardMode(s.Epoch, s.DisplayUUID, s.Session, s.PresentationRevision, s.Admission, on)
}

func TestLauncherInteractionDiscreteNoActionAcknowledged(t *testing.T) {
	a, _, panel, p := interactionFixture(t, func(c *config.LauncherInteractions) { c.PrimaryAction = "none" })
	before := a.GetLauncherInteractionState(p.Session)
	panel.send("swipe", 1, 1)
	launcherEventually(t, func() bool { panel.mu.Lock(); defer panel.mu.Unlock(); return panel.acks == 1 })
	if a.GetLauncherInteractionState(p.Session).SelectedItemID != before.SelectedItemID {
		t.Fatal("no-action swipe changed selection")
	}
}

func TestLauncherInteractionFailedKeyAdmissionReleasesNativeKey(t *testing.T) {
	a, _, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {})
	panel.mu.Lock()
	panel.rejectKey = true
	panel.mu.Unlock()
	if err := keyboardInteraction(a, a.GetLauncherInteractionState(p.Session), true); err == nil {
		t.Fatal("accepted refused key")
	}
	panel.mu.Lock()
	enabled := panel.key.Enabled
	panel.mu.Unlock()
	if enabled {
		t.Fatal("failed keyboard admission left native keyboard enabled")
	}
}

func TestLauncherInteractionSelectionClockAndKeyboardReplay(t *testing.T) {
	a, backend, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {})
	initial := a.GetLauncherInteractionState(p.Session)
	panel.send("scroll", 1, 80)
	launcherEventually(t, func() bool { return a.GetLauncherInteractionState(p.Session).SelectedItemID != initial.SelectedItemID })
	selected := a.GetLauncherInteractionState(p.Session)
	// An ordinary clock reconciliation preserves the exact interaction authority.
	panel.clock.Add(60)
	backend.send(100, 100)
	launcherEventually(t, func() bool {
		return a.GetLauncherInteractionState(p.Session).PresentationRevision > selected.PresentationRevision
	})
	latest := a.GetLauncherInteractionState(p.Session)
	if latest.Admission != selected.Admission || latest.SelectedItemID != selected.SelectedItemID {
		t.Fatal("clock retired selection")
	}
	if err := keyboardInteraction(a, latest, true); err != nil {
		t.Fatal(err)
	}
	s := a.GetLauncherInteractionState(p.Session)
	commit := func(seq uint64) error {
		return a.CommitLauncherLetter(s.Epoch, s.DisplayUUID, s.Session, s.PresentationRevision, s.Admission, seq, "b", 0, false)
	}
	if err := commit(1); err != nil {
		t.Fatal(err)
	}
	if err := commit(1); err == nil {
		t.Fatal("accepted replay")
	}
	if got := a.GetLauncherInteractionState(p.Session).SelectedItemID; got != "pin:beta" {
		t.Fatal(got)
	}
	panel.mu.Lock()
	panel.rejectKey = true
	panel.mu.Unlock()
	launcherEventually(t, func() bool { return !a.GetLauncherInteractionState(p.Session).KeyboardMode })
	if err := commit(2); err == nil {
		t.Fatal("old keyboard admission survived blur")
	}
}

func TestLauncherInteractionEnterFinalNativeGuardAfterPreparation(t *testing.T) {
	a, backend, panel, p := interactionFixture(t, func(c *config.LauncherInteractions) { c.EnterActivates = true })
	backend.release = make(chan struct{})
	if err := keyboardInteraction(a, a.GetLauncherInteractionState(p.Session), true); err != nil {
		t.Fatal(err)
	}
	s := a.GetLauncherInteractionState(p.Session)
	if err := a.CommitLauncherLetter(s.Epoch, s.DisplayUUID, s.Session, s.PresentationRevision, s.Admission, 1, "o", 0, false); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- a.ActivateLauncherSelection(s.Epoch, s.DisplayUUID, s.Session, s.PresentationRevision, s.Admission, 2)
	}()
	select {
	case <-backend.entered:
	case <-time.After(time.Second):
		t.Fatal("activation not prepared")
	}
	panel.mu.Lock()
	panel.rejectKey = true
	panel.mu.Unlock()
	close(backend.release)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("activated after key authority lost")
		}
	case <-time.After(time.Second):
		t.Fatal("activation did not join")
	}
	if backend.mutations.Load() != 0 {
		t.Fatal("native action dispatched after key loss")
	}
}

func TestLauncherInteractionDisableRetiresBlockedKeyboardCompletion(t *testing.T) {
	a, _, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {})
	entered, release := make(chan struct{}), make(chan struct{})
	panel.mu.Lock()
	panel.keyInstalled = func() { close(entered); <-release }
	panel.mu.Unlock()
	result := make(chan error, 1)
	before := a.GetLauncherInteractionState(p.Session)
	go func() { result <- keyboardInteraction(a, before, true) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("keyboard preparation not reached")
	}
	a.viewMu.Lock()
	h := a.launcher.hosts[p.Session]
	owner := h.interaction
	a.retireLauncherInteractionLocked(h)
	a.viewMu.Unlock()
	select {
	case <-owner.done:
		t.Fatal("owner drained before native preparation returned")
	default:
	}
	close(release)
	select {
	case <-owner.done:
	case <-time.After(time.Second):
		t.Fatal("owner not joined")
	}
	if err := <-result; err == nil {
		t.Fatal("accepted retired keyboard result")
	}
	panel.mu.Lock()
	key := panel.key.Enabled
	gesture := panel.policy.Enabled
	panel.mu.Unlock()
	if key || gesture {
		t.Fatal("native ownership survived retirement")
	}
	if a.GetLauncherInteractionState(p.Session).Visible {
		t.Fatal("late completion revived state")
	}
}

type interactionTestEnvironment struct {
	backend *launcherIntegrationPlatform
	panel   *interactionTestPanel
}

func (e interactionTestEnvironment) ObserveLauncherEnvironment(ctx context.Context, emit func(platform.LauncherEnvironment)) error {
	return e.backend.ObserveLauncherEnvironment(ctx, func(v platform.LauncherEnvironment) {
		v.ObservedAt = time.Now().Add(time.Duration(e.panel.clock.Load()) * time.Second)
		emit(v)
	})
}

func TestLauncherInteractionDisabledHasNoSelectionOrOwnership(t *testing.T) {
	a, backend, panel, p := interactionFixture(t, func(c *config.LauncherInteractions) { c.Enabled = false })
	a.viewMu.Lock()
	host := a.launcher.hosts[p.Session]
	hasOwner := host != nil && host.interaction != nil
	a.viewMu.Unlock()
	if host == nil || !host.presentation.Visible {
		t.Fatal("disabled interactions retired the launcher presentation")
	}
	if hasOwner || panel.physicalChecks.Load() != 0 {
		t.Fatal("disabled interactions created or physically validated a native input owner")
	}
	state := a.GetLauncherInteractionState(p.Session)
	if state.SelectedItemID != "" || state.KeyboardMode {
		t.Fatal("default-off presentation selected an item")
	}
	panel.mu.Lock()
	enabled := panel.policy.Enabled
	panel.mu.Unlock()
	if enabled {
		t.Fatal("disabled profile acquired gestures")
	}
	if err := keyboardInteraction(a, state, true); err == nil {
		t.Fatal("disabled keyboard admitted")
	}
	if backend.mutations.Load() != 0 || panel.haptics.Load() != 0 {
		t.Fatal("disabled profile activated or emitted haptic")
	}
}

type interactionTestPlatform struct {
	*launcherIntegrationPlatform
	panel *interactionTestPanel
}

func (p interactionTestPlatform) HapticTick() { p.panel.haptics.Add(1) }
func TestLauncherInteractionHapticsRequireIntentionalTransition(t *testing.T) {
	a, _, panel, p := interactionFixture(t, func(c *config.LauncherInteractions) { c.Haptics = true })
	if panel.haptics.Load() != 0 || a.GetLauncherInteractionState(p.Session).SelectedItemID != "" {
		t.Fatal("passive presentation produced selection/haptic")
	}
	panel.send("scroll", 1, 80)
	launcherEventually(t, func() bool { return panel.haptics.Load() == 1 })
	panel.send("scroll", 1, 80)
	// A terminal replay cannot create a second transition or tick.
	launcherEventually(t, func() bool { panel.mu.Lock(); defer panel.mu.Unlock(); return panel.acks >= 1 })
	if panel.haptics.Load() != 1 {
		t.Fatal("replay haptic")
	}
}

func TestLauncherInteractionReplacementWaitsForPredecessorChain(t *testing.T) {
	a, _, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {})
	entered, release := make(chan struct{}), make(chan struct{})
	panel.mu.Lock()
	panel.keyInstalled = func() { close(entered); <-release }
	panel.mu.Unlock()
	result := make(chan error, 1)
	go func() { result <- keyboardInteraction(a, a.GetLauncherInteractionState(p.Session), true) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("keyboard preparation not reached")
	}
	a.viewMu.Lock()
	h := a.launcher.hosts[p.Session]
	first := h.interaction
	a.retireLauncherInteractionLocked(h)
	a.syncLauncherInteractionsLocked()
	middle := h.interaction
	a.retireLauncherInteractionLocked(h)
	a.syncLauncherInteractionsLocked()
	last := h.interaction
	a.viewMu.Unlock()
	if middle == nil || last == nil || middle == last {
		t.Fatal("replacement not reserved")
	}
	select {
	case <-middle.done:
		t.Fatal("cancelled middle abandoned predecessor")
	default:
	}
	panel.mu.Lock()
	admitted := panel.policy.Admission
	panel.mu.Unlock()
	if admitted == last.admission.Load() {
		t.Fatal("new native owner started before prior join")
	}
	close(release)
	select {
	case <-first.done:
	case <-time.After(time.Second):
		t.Fatal("first not joined")
	}
	select {
	case <-middle.done:
	case <-time.After(time.Second):
		t.Fatal("middle not joined")
	}
	launcherEventually(t, func() bool {
		panel.mu.Lock()
		defer panel.mu.Unlock()
		return panel.policy.Enabled && panel.policy.Admission == last.admission.Load()
	})
	if err := <-result; err == nil {
		t.Fatal("old command succeeded")
	}
}

func TestLauncherInteractionErrorHasOneAuthoritativeSequence(t *testing.T) {
	var names []string
	var states []LauncherInteractionState
	a := &App{eventSink: func(name string, value any) {
		names = append(names, name)
		states = append(states, value.(LauncherInteractionState))
	}}
	o := &launcherInteractionOwner{state: LauncherInteractionState{Visible: true, Admission: 1}}
	a.publishLauncherInteractionLocked(o, "failed")
	if len(names) != 1 || names[0] != "launcher:interaction" || states[0].Reason != "failed" {
		t.Fatal("error depends on equal-sequence cross-channel delivery", names, states)
	}
	a.publishLauncherInteractionLocked(o, "")
	if states[1].Sequence <= states[0].Sequence || states[1].Reason != "" {
		t.Fatal("success retained stale error", states)
	}
}

func (p *interactionTestPanel) ValidateLauncherPanel(ctx context.Context, display string) error {
	err := p.launcherIntegrationPanel.ValidateLauncherPanel(ctx, display)
	p.physicalChecks.Add(1)
	return err
}

func TestLauncherInteractionWaitsForPhysicalShow(t *testing.T) {
	ready := make(chan struct{})
	a, _, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {}, func(panel *interactionTestPanel) {
		panel.unavailable.Store(true)
		go func() {
			defer close(ready)
			for panel.physicalChecks.Load() == 0 {
				time.Sleep(time.Millisecond)
			}
			panel.unavailable.Store(false)
		}()
	})
	<-ready
	if !a.GetLauncherInteractionState(p.Session).Visible {
		t.Fatal("ready host retained dead input owner")
	}
	panel.mu.Lock()
	enabled := panel.policy.Enabled
	panel.mu.Unlock()
	if !enabled {
		t.Fatal("ready host did not install input")
	}
}

func TestLauncherInteractionInitializationDeadlineRetiresDeadOwner(t *testing.T) {
	a, backend, panel, p := interactionFixture(t, func(*config.LauncherInteractions) {})
	panel.unavailable.Store(true)
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				backend.send(100, 100)
			}
		}
	}()
	defer func() { close(stop); <-joined }()
	a.viewMu.Lock()
	h := a.launcher.hosts[p.Session]
	a.retireLauncherInteractionLocked(h)
	a.syncLauncherInteractionsLocked()
	owner := h.interaction
	a.viewMu.Unlock()
	if owner == nil {
		t.Fatal("missing initialization owner")
	}
	select {
	case <-owner.done:
	case <-time.After(3 * time.Second):
		t.Fatal("physical readiness was not bounded")
	}
	a.viewMu.Lock()
	live := owner.live.Load()
	current := h.interaction
	retry := h.interactionRetryAt
	a.syncLauncherInteractionsLocked()
	immediate := h.interaction
	a.viewMu.Unlock()
	if live || current != nil || immediate != nil || !time.Now().Before(retry) {
		t.Fatal("terminal initialization left live/dead owner or unbounded immediate retry")
	}
}
