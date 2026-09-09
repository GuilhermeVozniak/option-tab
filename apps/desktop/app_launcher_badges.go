package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"option-tab/internal/launcher"
	"option-tab/internal/platform"
)

type LauncherBadgeViewEntry struct {
	ItemID string                      `json:"itemID"`
	State  platform.LauncherBadgeState `json:"state"`
	Kind   platform.LauncherBadgeKind  `json:"kind"`
	Count  *uint32                     `json:"count,omitempty"`
}
type LauncherBadgeViewState struct {
	Epoch                uint64                       `json:"epoch"`
	DisplayUUID          string                       `json:"displayUUID"`
	Session              uint64                       `json:"session"`
	PresentationRevision uint64                       `json:"presentationRevision"`
	Owner                uint64                       `json:"owner"`
	Sequence             uint64                       `json:"sequence"`
	Visible              bool                         `json:"visible"`
	Status               platform.LauncherBadgeStatus `json:"status"`
	Entries              []LauncherBadgeViewEntry     `json:"entries"`
}
type launcherBadgePlan struct {
	host      *appLauncherHost
	authority launcher.BadgeAuthority
	refs      *appLauncherItems
}
type appLauncherBadges struct {
	worker *launcherBadgeWorker
	source platform.LauncherBadgeSource
	plans  map[uint64]launcherBadgePlan
	views  map[uint64]LauncherBadgeViewState
}
type (
	launcherBadgeBinding struct {
		item   launcher.BadgeItem
		target platform.LauncherBadgeTarget
		key    string
	}
	preparedLauncherBadgePlan struct {
		plan     launcherBadgePlan
		panel    platform.LauncherPanel
		token    uint64
		bindings []launcherBadgeBinding
	}
)

func (a *App) newLauncherBadges(r *appLauncherRuntime) *appLauncherBadges {
	b := &appLauncherBadges{source: platform.NewLauncherBadgeSource(), plans: map[uint64]launcherBadgePlan{}, views: map[uint64]LauncherBadgeViewState{}}
	b.worker = newLauncherBadgeWorker(func(ctx context.Context, generation uint64) { a.runLauncherBadges(ctx, r, b, generation) })
	return b
}

func cloneLauncherBadgeView(v LauncherBadgeViewState) LauncherBadgeViewState {
	v.Entries = append([]LauncherBadgeViewEntry{}, v.Entries...)
	for i := range v.Entries {
		if v.Entries[i].Count != nil {
			n := *v.Entries[i].Count
			v.Entries[i].Count = &n
		}
	}
	return v
}

func (a *App) syncLauncherBadgesLocked() {
	r := a.launcher
	if r == nil || r.badges == nil {
		return
	}
	b := r.badges
	plans := map[uint64]launcherBadgePlan{}
	if a.launcherAllowedLocked() && r.ready {
		for session, h := range r.hosts {
			// Core content may advance before its asynchronous View reaches
			// this host. Keep existing authority only if the core independently
			// validates its exact resources, profile and display admission.
			if old, ok := b.plans[session]; ok && old.host == h && old.refs == a.launcherItems && h.presentation.Visible {
				if _, err := r.core.ValidateBadges(old.authority); err == nil && a.badgeSettingsCurrent(old.authority) {
					plans[session] = old
					continue
				}
			}
			authority, err := r.core.CaptureBadges(h.presentation.Scope)
			if err != nil || !h.presentation.Visible || len(authority.Items()) == 0 || !a.badgeSettingsCurrent(authority) {
				continue
			}
			plans[session] = launcherBadgePlan{host: h, authority: authority, refs: a.launcherItems}
		}
	}
	same := len(plans) == len(b.plans)
	if same {
		for session, p := range plans {
			old, ok := b.plans[session]
			if !ok || old.host != p.host || old.refs != p.refs {
				same = false
				break
			}
			if _, err := r.core.ValidateBadges(old.authority); err != nil {
				same = false
				break
			}
		}
	}
	if same {
		return
	}
	generation := b.worker.update()
	for session, v := range b.views {
		v.Sequence++
		v.Visible = false
		v.Entries = []LauncherBadgeViewEntry{}
		v.Status = platform.BadgeSourceUnavailable
		a.emit("launcher:badges", v)
		delete(b.views, session)
	}
	b.plans = plans
	for session, p := range plans {
		v := badgeUnknownView(p.authority, generation)
		b.views[session] = v
		a.emit("launcher:badges", cloneLauncherBadgeView(v))
	}
}

func badgeUnknownView(a launcher.BadgeAuthority, owner uint64) LauncherBadgeViewState {
	v := LauncherBadgeViewState{Epoch: a.Scope.Epoch, DisplayUUID: a.Scope.DisplayUUID, Session: a.Scope.Session, PresentationRevision: a.Scope.Revision, Owner: owner, Sequence: 1, Visible: true, Status: platform.BadgeSourceUnavailable, Entries: []LauncherBadgeViewEntry{}}
	for _, item := range a.Items() {
		v.Entries = append(v.Entries, LauncherBadgeViewEntry{ItemID: item.ItemID, State: platform.BadgeUnavailable})
	}
	return v
}

func (a *App) GetLauncherBadges(session uint64) LauncherBadgeViewState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	a.syncLauncherBadgesLocked()
	if a.launcher == nil || a.launcher.badges == nil {
		return LauncherBadgeViewState{Entries: []LauncherBadgeViewEntry{}}
	}
	v, ok := a.launcher.badges.views[session]
	if !ok {
		return LauncherBadgeViewState{Session: session, Entries: []LauncherBadgeViewEntry{}}
	}
	if h := a.launcher.hosts[session]; h != nil {
		v.PresentationRevision = h.presentation.Revision
	}
	return cloneLauncherBadgeView(v)
}

func (a *App) badgePlanCurrentLocked(r *appLauncherRuntime, b *appLauncherBadges, g uint64, p launcherBadgePlan) (launcher.Scope, bool) {
	if a.launcher != r || r.badges != b || !b.worker.current(g) || !a.launcherAllowedLocked() || !r.ready || r.hosts[p.authority.Scope.Session] != p.host || !p.host.presentation.Visible {
		return launcher.Scope{}, false
	}
	current, ok := b.plans[p.authority.Scope.Session]
	if !ok || current.host != p.host || current.authority.Admission != p.authority.Admission {
		return launcher.Scope{}, false
	}
	scope, err := r.core.ValidateBadges(p.authority)
	return scope, err == nil && a.badgeSettingsCurrent(p.authority)
}

func (a *App) badgePlanPhysical(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64, p preparedLauncherBadgePlan) bool {
	if ctx.Err() != nil {
		return false
	}
	validator, ok := p.panel.(platform.LauncherPanelValidator)
	if !ok || validator.ValidateLauncherPanel(ctx, p.plan.authority.Scope.DisplayUUID) != nil {
		return false
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	_, current := a.badgePlanCurrentLocked(r, b, g, p.plan)
	return current && ctx.Err() == nil && p.plan.host.window.wheelPanel() == p.panel && p.panel.LauncherToken() == p.token
}

func (a *App) badgeTarget(ctx context.Context, p launcherBadgePlan, item launcher.BadgeItem) (platform.LauncherBadgeTarget, error) {
	if item.Reference.ID == "" {
		resolver, ok := a.platform.(platform.LauncherRunningBadgeResolver)
		if !ok {
			return platform.LauncherBadgeTarget{}, launcher.ErrUnavailable
		}
		return resolver.ResolveRunningLauncherBadgeTarget(ctx, item.App.Process, item.App.BundleID)
	}
	a.viewMu.Lock()
	m := a.launcherItems
	if m == nil || m != p.refs || m.closed || m.refs == nil {
		a.viewMu.Unlock()
		return platform.LauncherBadgeTarget{}, launcher.ErrRetired
	}
	resolver, ok := m.refs.(platform.LauncherBadgeReferenceResolver)
	if !ok {
		a.viewMu.Unlock()
		return platform.LauncherBadgeTarget{}, launcher.ErrUnavailable
	}
	m.wg.Add(1)
	a.viewMu.Unlock()
	defer m.wg.Done()
	child, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	defer cancel()
	if m.ctx.Err() != nil {
		cancel()
	}
	if err := child.Err(); err != nil {
		return platform.LauncherBadgeTarget{}, err
	}
	target, err := resolver.ResolveLauncherBadgeTarget(child, item.Reference)
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.launcherItems != m || m.closed || child.Err() != nil {
		return platform.LauncherBadgeTarget{}, launcher.ErrRetired
	}
	return target, err
}

func (a *App) prepareBadgePlan(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64, p launcherBadgePlan) (preparedLauncherBadgePlan, bool) {
	result := preparedLauncherBadgePlan{plan: p}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		a.viewMu.Lock()
		_, current := a.badgePlanCurrentLocked(r, b, g, p)
		panel, _ := p.host.window.wheelPanel().(platform.LauncherPanel)
		a.viewMu.Unlock()
		if !current || ctx.Err() != nil {
			return result, false
		}
		if panel != nil && panel.LauncherToken() != 0 {
			result.panel = panel
			result.token = panel.LauncherToken()
			break
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, false
		case <-deadline.C:
			timer.Stop()
			return result, false
		case <-timer.C:
		}
	}
	if !a.badgePlanPhysical(ctx, r, b, g, result) {
		return result, false
	}
	for _, item := range p.authority.Items() {
		binding := launcherBadgeBinding{item: item}
		target, err := a.badgeTarget(ctx, p, item)
		if err == nil {
			binding.target = target
		}
		result.bindings = append(result.bindings, binding)
	}
	return result, a.badgePlanPhysical(ctx, r, b, g, result)
}

func (a *App) runLauncherBadges(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64) {
	for ctx.Err() == nil {
		a.viewMu.Lock()
		current := a.launcher == r && r.badges == b && b.worker.current(g) && b.source != nil && len(b.plans) != 0
		if current {
			for _, p := range b.plans {
				if _, ok := a.badgePlanCurrentLocked(r, b, g, p); !ok {
					current = false
					break
				}
			}
		}
		a.viewMu.Unlock()
		if !current {
			return
		}
		// An attempt returns only after source observation and private reads join.
		a.attemptLauncherBadges(ctx, r, b, g)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (a *App) attemptLauncherBadges(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64) {
	a.viewMu.Lock()
	if a.launcher != r || r.badges != b || !b.worker.current(g) {
		a.viewMu.Unlock()
		return
	}
	var plans []launcherBadgePlan
	for _, p := range b.plans {
		plans = append(plans, p)
	}
	source := b.source
	a.viewMu.Unlock()
	if len(plans) == 0 || source == nil {
		return
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].authority.Scope.Session < plans[j].authority.Scope.Session })
	unavailable := make([]preparedLauncherBadgePlan, 0, len(plans))
	for _, p := range plans {
		unavailable = append(unavailable, preparedLauncherBadgePlan{plan: p})
	}
	scheduledRefresh := false
	defer func() {
		if !scheduledRefresh {
			a.publishBadgeFailure(ctx, r, b, g, unavailable)
		}
	}()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, p := range plans {
		if p.refs != nil {
			stop := context.AfterFunc(p.refs.ctx, cancel)
			defer stop()
			if p.refs.ctx.Err() != nil {
				cancel()
			}
		}
	}
	incomplete := false
	var prepared []preparedLauncherBadgePlan
	var targets []platform.LauncherBadgeTarget
	keys := map[platform.LauncherBadgeTarget]string{}
	for _, p := range plans {
		plan, ok := a.prepareBadgePlan(child, r, b, g, p)
		if !ok {
			incomplete = true
			a.publishBadgeFailure(child, r, b, g, []preparedLauncherBadgePlan{{plan: p}})
			continue
		}
		for i := range plan.bindings {
			v := &plan.bindings[i]
			if v.target.CanonicalAppPath == "" {
				incomplete = true
				continue
			}
			// Normalize caller-owned fields before identity deduplication.
			v.target.ItemKey = ""
			v.target.TargetRevision = 0
			key := keys[v.target]
			if key == "" {
				if len(targets) >= 256 {
					incomplete = true
					continue
				}
				key = fmt.Sprint("target:", len(targets))
				keys[v.target] = key
				t := v.target
				t.ItemKey = key
				t.TargetRevision = g
				targets = append(targets, t)
			}
			v.key = key
		}
		a.clearUnresolvedBadgeBindings(child, r, b, g, plan)
		prepared = append(prepared, plan)
	}
	if child.Err() != nil {
		return
	}
	if len(targets) == 0 {
		return
	}
	if incomplete {
		// Retry missing union members without permanently suppressing ready ones.
		var stop context.CancelFunc
		child, stop = context.WithTimeout(child, time.Second)
		defer stop()
	}
	var generation, sequence uint64
	observeErr := source.ObserveLauncherBadges(child, targets, func(snapshot platform.LauncherBadgeSnapshot) {
		if child.Err() != nil || snapshot.Generation == 0 || snapshot.Sequence == 0 || snapshot.Generation < generation || (snapshot.Generation == generation && snapshot.Sequence <= sequence) {
			return
		}
		generation, sequence = snapshot.Generation, snapshot.Sequence
		if len(snapshot.Entries) != len(targets) || len(snapshot.Entries) > 256 {
			a.publishBadgeFailure(child, r, b, g, prepared)
			return
		}
		seen := make(map[string]bool, len(targets))
		for _, entry := range snapshot.Entries {
			if entry.TargetRevision != g || seen[entry.ItemKey] {
				a.publishBadgeFailure(child, r, b, g, prepared)
				return
			}
			seen[entry.ItemKey] = true
		}
		for _, target := range targets {
			if !seen[target.ItemKey] {
				a.publishBadgeFailure(child, r, b, g, prepared)
				return
			}
		}
		for _, plan := range prepared {
			if !a.publishBadgeSnapshot(child, r, b, g, plan, snapshot) {
				cancel()
				return
			}
		}
	})
	scheduledRefresh = incomplete && child.Err() == context.DeadlineExceeded && ctx.Err() == nil && (observeErr == nil || errors.Is(observeErr, context.DeadlineExceeded) || errors.Is(observeErr, context.Canceled))
}

func (a *App) publishBadgeSnapshot(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64, p preparedLauncherBadgePlan, s platform.LauncherBadgeSnapshot) bool {
	values := map[string]platform.LauncherBadgeEntry{}
	for _, entry := range s.Entries {
		if entry.TargetRevision == g {
			values[entry.ItemKey] = entry
		}
	}
	resolutionCurrent := true
	entries := make([]LauncherBadgeViewEntry, 0, len(p.bindings))
	for _, binding := range p.bindings {
		out := LauncherBadgeViewEntry{ItemID: binding.item.ItemID, State: platform.BadgeUnavailable}
		if s.Status == platform.BadgeSourceUnsupported {
			out.State = platform.BadgeUnsupported
		}
		fresh, err := a.badgeTarget(ctx, p.plan, binding.item)
		fresh.ItemKey = ""
		fresh.TargetRevision = 0
		if binding.key != "" && (err != nil || fresh != binding.target) {
			resolutionCurrent = false
		}
		if err == nil && binding.key != "" && fresh == binding.target && s.Status == platform.BadgeReady {
			entry, ok := values[binding.key]
			if ok {
				switch entry.State {
				case platform.BadgeUnsupported:
					out.State = platform.BadgeUnsupported
				case platform.BadgeKnown:
					if entry.Kind == platform.BadgeAbsent || entry.Kind == platform.BadgeIndicator {
						out.State = platform.BadgeKnown
						out.Kind = entry.Kind
					}
					if entry.Kind == platform.BadgeCount && entry.Count != nil && *entry.Count <= 999999 {
						n := *entry.Count
						out.State = platform.BadgeKnown
						out.Kind = entry.Kind
						out.Count = &n
					}
				}
			}
		}
		entries = append(entries, out)
	}
	if !a.badgePlanPhysical(ctx, r, b, g, p) {
		a.publishBadgeFailure(ctx, r, b, g, []preparedLauncherBadgePlan{p})
		return false
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	scope, current := a.badgePlanCurrentLocked(r, b, g, p.plan)
	if !current || ctx.Err() != nil {
		return false
	}
	v := b.views[scope.Session]
	v.PresentationRevision = scope.Revision
	v.Sequence++
	v.Entries = entries
	v.Status = platform.BadgeSourceUnavailable
	switch s.Status {
	case platform.BadgeReady:
		v.Status = platform.BadgeReady
	case platform.BadgeSourceUnsupported:
		v.Status = platform.BadgeSourceUnsupported
	}
	b.views[scope.Session] = cloneLauncherBadgeView(v)
	a.emit("launcher:badges", cloneLauncherBadgeView(v))
	return resolutionCurrent
}

func (a *App) publishBadgeFailure(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64, plans []preparedLauncherBadgePlan) {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if ctx.Err() != nil {
		return
	}
	for _, p := range plans {
		scope, current := a.badgePlanCurrentLocked(r, b, g, p.plan)
		if !current {
			continue
		}
		v := badgeUnknownView(p.plan.authority, g)
		v.PresentationRevision = scope.Revision
		v.Sequence = b.views[scope.Session].Sequence + 1
		b.views[scope.Session] = v
		a.emit("launcher:badges", cloneLauncherBadgeView(v))
	}
}

func (a *App) badgeSettingsCurrent(authority launcher.BadgeAuthority) bool {
	enabled := false
	for _, profile := range a.settingsSnapshot().ReplacementDock.Profiles {
		if profile.ID == authority.ProfileID {
			enabled = profile.ShowBadges
		}
	}
	if !enabled {
		return false
	}
	for _, item := range authority.Items() {
		ref := item.Reference
		if ref.ID == "" {
			ref = platform.LauncherReference{Kind: "app", Label: item.App.Name, BundleID: item.App.BundleID, Process: item.App.Process}
		}
		if !a.launcherReferenceEligible(ref) {
			return false
		}
	}
	return true
}

// A scheduled partial refresh preserves only entries whose private target was
// resolved in this attempt. Missing bindings must not inherit old counts while
// waiting for the source's first packet.
func (a *App) clearUnresolvedBadgeBindings(ctx context.Context, r *appLauncherRuntime, b *appLauncherBadges, g uint64, p preparedLauncherBadgePlan) {
	missing := map[string]bool{}
	for _, binding := range p.bindings {
		if binding.key == "" {
			missing[binding.item.ItemID] = true
		}
	}
	if len(missing) == 0 {
		return
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	scope, current := a.badgePlanCurrentLocked(r, b, g, p.plan)
	if !current || ctx.Err() != nil {
		return
	}
	v := cloneLauncherBadgeView(b.views[scope.Session])
	changed := false
	for i, entry := range v.Entries {
		if missing[entry.ItemID] && (entry.State != platform.BadgeUnavailable || entry.Count != nil || entry.Kind != "") {
			v.Entries[i] = LauncherBadgeViewEntry{ItemID: entry.ItemID, State: platform.BadgeUnavailable}
			changed = true
		}
	}
	if !changed {
		return
	}
	v.PresentationRevision = scope.Revision
	v.Sequence++
	b.views[scope.Session] = cloneLauncherBadgeView(v)
	a.emit("launcher:badges", cloneLauncherBadgeView(v))
}
