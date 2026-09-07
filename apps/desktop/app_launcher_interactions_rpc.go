package main

import (
	"context"
	"time"

	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

func (a *App) submitLauncherInteraction(scope launcher.Scope, admission uint64, job launcherInteractionJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	job.ctx = ctx
	job.admission = admission
	job.done = make(chan error, 1)
	a.viewMu.Lock()
	if a.launcher == nil || !a.launcherAllowedLocked() {
		a.viewMu.Unlock()
		return interactionError("stale")
	}
	h := a.launcher.hosts[scope.Session]
	if h == nil || h.interaction == nil || h.presentation.Scope != scope {
		a.viewMu.Unlock()
		return interactionError("stale")
	}
	o := h.interaction
	if !o.live.Load() || o.admission.Load() != admission {
		a.viewMu.Unlock()
		return interactionError("stale")
	}
	a.viewMu.Unlock()
	select {
	case o.jobs <- job:
	case <-o.ctx.Done():
		return interactionError("stale")
	default:
		return interactionError("busy")
	}
	select {
	case err := <-job.done:
		return err
	case <-ctx.Done():
		return interactionError("stale")
	case <-o.ctx.Done():
		return interactionError("stale")
	}
}

func (a *App) SetLauncherKeyboardMode(epoch uint64, displayUUID string, session, presentationRevision, admission uint64, enabled bool) error {
	return a.submitLauncherInteraction(launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: presentationRevision}, admission, launcherInteractionJob{kind: "keyboard", enabled: enabled})
}

func (a *App) CommitLauncherLetter(epoch uint64, displayUUID string, session, presentationRevision, admission, sequence uint64, text string, modifiers uint64, composing bool) error {
	return a.submitLauncherInteraction(launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: presentationRevision}, admission, launcherInteractionJob{kind: "letter", sequence: sequence, text: text, modifiers: modifiers, composing: composing})
}

func (a *App) ActivateLauncherSelection(epoch uint64, displayUUID string, session, presentationRevision, admission, sequence uint64) error {
	return a.submitLauncherInteraction(launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: presentationRevision}, admission, launcherInteractionJob{kind: "activate", sequence: sequence})
}

func (a *App) SetLauncherReorderTarget(epoch uint64, displayUUID string, session, presentationRevision, admission, sequence uint64, itemID string) error {
	return a.submitLauncherInteraction(launcher.Scope{Epoch: epoch, DisplayUUID: displayUUID, Session: session, Revision: presentationRevision}, admission, launcherInteractionJob{kind: "reorder", sequence: sequence, text: itemID})
}

func (o *launcherInteractionOwner) job(j launcherInteractionJob) error {
	if j.ctx.Err() != nil {
		return interactionError("stale")
	}
	if err := o.guard(j.admission, j.kind == "letter" || j.kind == "activate"); err != nil {
		if j.kind == "letter" || j.kind == "activate" {
			o.exitKeyboard()
		}
		return err
	}
	if j.kind != "keyboard" {
		if j.sequence == 0 || j.sequence <= o.inputSequence {
			return interactionError("stale")
		}
		o.inputSequence = j.sequence
	}
	switch j.kind {
	case "keyboard":
		o.app.viewMu.Lock()
		available := o.state.LetterInputAvailable
		o.app.viewMu.Unlock()
		if j.enabled && (!o.config.Enabled || !o.config.LetterNavigation || !available) {
			return interactionError("unavailable")
		}
		return o.keyboard(j.ctx, j.enabled)
	case "letter":
		if !o.config.Enabled || !o.config.LetterNavigation || len(j.text) > 16 {
			return interactionError("unavailable")
		}
		selected := o.letters.Step(launcher.LetterPacket{Scope: o.authority.Scope, Admission: o.letterAdmission, Sequence: j.sequence, Timestamp: time.Now(), Key: j.text, Modifiers: j.modifiers, Composing: j.composing}, o.authority.NavigationItems())
		if selected != "" {
			return o.selectItem(selected, j.admission, true)
		}
		return nil
	case "activate":
		if !o.config.Enabled || !o.config.EnterActivates {
			return interactionError("unavailable")
		}
		o.app.viewMu.Lock()
		selected := o.state.SelectedItemID
		o.app.viewMu.Unlock()
		latest, err := o.runtime.core.ValidateInteraction(o.authority)
		if err != nil || selected == "" {
			return interactionError("stale")
		}
		guard := func() error {
			if j.ctx.Err() != nil {
				return interactionError("stale")
			}
			return o.guard(j.admission, true)
		}
		if err = o.runtime.core.ActivateGuarded(j.ctx, latest, selected, guard); err != nil {
			o.fail("failed")
			return interactionError("failed")
		}
		return nil
	case "reorder":
		if !o.config.Enabled || !o.config.Haptics || !o.reorder {
			return nil
		}
		o.app.viewMu.Lock()
		valid := false
		for _, item := range o.host.presentation.Items {
			valid = valid || item.ID == j.text
			for _, member := range item.Members {
				valid = valid || member.ID == j.text
			}
		}
		o.app.viewMu.Unlock()
		if !valid {
			return interactionError("stale")
		}
		o.tickHaptic("reorder:"+j.text, j.admission, false)
		return nil
	}
	return interactionError("unavailable")
}

func (o *launcherInteractionOwner) rotate() {
	o.app.viewMu.Lock()
	o.runtime.nextInteraction++
	o.admission.Store(o.runtime.nextInteraction)
	o.state.Admission = o.admission.Load()
	o.state.KeyboardMode = false
	o.inputSequence = 0
	o.app.viewMu.Unlock()
}

func (o *launcherInteractionOwner) keyboard(ctx context.Context, enabled bool) error {
	source, ok := o.panel.(platform.LauncherKeyboardSource)
	if !ok {
		return interactionError("unavailable")
	}
	o.rotate()
	admission := o.admission.Load()
	s := o.authority.Scope
	err := source.SetLauncherKeyboardPolicy(platform.LauncherKeyboardPolicy{Epoch: s.Epoch, Session: s.Session, Revision: s.Revision, Admission: admission, DisplayUUID: s.DisplayUUID, Enabled: enabled}, func() bool { return ctx.Err() == nil && o.logical() && o.admission.Load() == admission })
	if err == nil && enabled {
		v, ok := o.panel.(platform.LauncherKeyboardValidator)
		if !ok || !v.ValidateLauncherKeyboard(s.Epoch, s.Session, s.Revision, admission) {
			err = interactionError("stale")
		}
	}
	if err == nil && (ctx.Err() != nil || !o.logical() || o.admission.Load() != admission) {
		err = interactionError("stale")
	}
	if err != nil && enabled {
		// A failed post-install admission must relinquish the native key window.
		_ = source.SetLauncherKeyboardPolicy(platform.LauncherKeyboardPolicy{Epoch: s.Epoch, Session: s.Session, Revision: s.Revision, Admission: admission, DisplayUUID: s.DisplayUUID}, nil)
	}
	if o.logical() && o.admission.Load() == admission {
		o.app.viewMu.Lock()
		o.state.KeyboardMode = err == nil && enabled
		reason := ""
		if err != nil {
			reason = "unavailable"
		}
		o.app.publishLauncherInteractionLocked(o, reason)
		o.app.viewMu.Unlock()
		if installErr := o.install(); err == nil {
			err = installErr
		}
	}
	if err != nil {
		return interactionError("unavailable")
	}
	return nil
}

func (o *launcherInteractionOwner) exitKeyboard() {
	o.app.viewMu.Lock()
	active := o.state.KeyboardMode
	o.app.viewMu.Unlock()
	if active {
		_ = o.keyboard(o.ctx, false)
	}
}

func (o *launcherInteractionOwner) selectItem(id string, admission uint64, keyboard bool) error {
	if err := o.guard(admission, keyboard); err != nil {
		return err
	}
	o.app.viewMu.Lock()
	if !o.logical() || o.admission.Load() != admission || o.host.interaction != o {
		o.app.viewMu.Unlock()
		return interactionError("stale")
	}
	changed := o.state.SelectedItemID != id
	o.state.SelectedItemID = id
	if changed {
		o.app.publishLauncherInteractionLocked(o, "")
	}
	o.app.viewMu.Unlock()
	if changed {
		o.tickHaptic("selection:"+id, admission, keyboard)
	}
	return nil
}

func (o *launcherInteractionOwner) tickHaptic(target string, admission uint64, keyboard bool) {
	if !o.haptic.Allow(o.authority.Scope, o.config.Enabled && o.config.Haptics, true, target, time.Now()) {
		return
	}
	if o.guard(admission, keyboard) != nil {
		return
	}
	if h, ok := o.app.platform.(platform.HapticFeedback); ok {
		h.HapticTick()
	}
}

func (o *launcherInteractionOwner) event(e platform.LauncherGestureEvent) {
	if e.Admission != o.admission.Load() || o.guard(e.Admission, false) != nil {
		o.ack(e)
		return
	}
	v, ok := o.panel.(platform.LauncherGestureValidator)
	if !ok || !v.ValidateLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID) {
		o.ack(e)
		return
	}
	packet := launcher.GesturePacket{Scope: launcher.Scope{Epoch: e.Epoch, DisplayUUID: e.DisplayUUID, Session: e.Session, Revision: e.Revision}, Admission: o.reducerAdmission, Sequence: e.Sequence, GestureID: e.GestureID, Timestamp: e.Timestamp, PanelX: e.PanelX, PanelY: e.PanelY, DeltaX: e.DeltaX, DeltaY: e.DeltaY, Magnification: e.Magnification, Kind: e.Kind, Phase: e.Phase, MomentumPhase: e.MomentumPhase, Precise: e.Precise, Owned: e.Owned}
	intent := o.reducer.Step(packet)
	terminal := e.Phase == "ended" || e.Phase == "cancelled" || e.MomentumPhase == "ended" || e.MomentumPhase == "cancelled"
	if intent == nil {
		if terminal || e.Kind == "swipe" {
			o.ack(e)
		}
		return
	}
	defer o.ack(e)
	if o.guard(e.Admission, false) != nil || !v.ValidateLauncherGesture(e.Epoch, e.Session, e.Revision, e.Admission, e.GestureID) {
		return
	}
	nav := o.authority.NavigationItems()
	if len(nav) == 0 {
		return
	}
	o.app.viewMu.Lock()
	selected := o.state.SelectedItemID
	o.app.viewMu.Unlock()
	if intent.Action == "next" || intent.Action == "previous" {
		index := -1
		for i, item := range nav {
			if item.ID == selected {
				index = i
				break
			}
		}
		if intent.Action == "next" {
			index = (index + 1) % len(nav)
		} else {
			if index < 0 {
				index = 0
			}
			index = (index + len(nav) - 1) % len(nav)
		}
		_ = o.selectItem(nav[index].ID, e.Admission, false)
		return
	}
	latest, err := o.runtime.core.ValidateInteraction(o.authority)
	if err != nil {
		return
	}
	switch intent.Action {
	case "show":
		if selected == "" {
			o.app.viewMu.Lock()
			for _, item := range o.host.presentation.Items {
				if (item.Kind == "app" || item.Kind == "folder") && item.Status == "ready" {
					selected = item.ID
					break
				}
				for _, member := range item.Members {
					if member.Kind == "app" && member.Status == "ready" {
						selected = member.ID
						break
					}
				}
				if selected != "" {
					break
				}
			}
			o.app.viewMu.Unlock()
			if selected == "" {
				return
			}
			if o.selectItem(selected, e.Admission, false) != nil {
				return
			}
		}
		if _, err = o.app.ShowLauncherItemPanel(latest.Epoch, latest.DisplayUUID, latest.Session, latest.Revision, selected); err != nil {
			o.fail("failed")
		} else {
			o.fail("")
		}
	case "hide":
		o.app.viewMu.Lock()
		if o.logical() && o.admission.Load() == e.Admission {
			o.app.retireLauncherItemPanelLocked(latest.Session)
		}
		o.app.viewMu.Unlock()
	}
}
