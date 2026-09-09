package main

import (
	"context"
	"encoding/base64"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/launcher"
	"option-tab/internal/platform"
	"option-tab/internal/widgetproviders"
	"option-tab/internal/widgets"
)

type WidgetCatalogItem struct {
	PackageID            string            `json:"packageID"`
	Digest               string            `json:"digest"`
	Version              string            `json:"version"`
	Name                 widgets.Localized `json:"name"`
	Description          widgets.Localized `json:"description"`
	RequiredCapabilities []string          `json:"requiredCapabilities"`
	OptionalCapabilities []string          `json:"optionalCapabilities"`
	Settings             []widgets.Setting `json:"settings"`
	Builtin              bool              `json:"builtin"`
}

type LauncherWidgetChoice struct {
	ID   string            `json:"id"`
	Name widgets.Localized `json:"name"`
}

type LauncherWidgetSlot struct {
	ID         string                 `json:"id"`
	StackID    string                 `json:"stackID,omitempty"`
	Name       widgets.Localized      `json:"name"`
	Members    []LauncherWidgetChoice `json:"members"`
	SelectedID string                 `json:"selectedID"`
	Status     string                 `json:"status"`
	State      *widgets.InstanceState `json:"state,omitempty"`
}

type LauncherWidgetState struct {
	Epoch       uint64               `json:"epoch"`
	DisplayUUID string               `json:"displayUUID"`
	Session     uint64               `json:"session"`
	ProfileID   string               `json:"profileID"`
	Revision    uint64               `json:"revision"`
	Visible     bool                 `json:"visible"`
	Slots       []LauncherWidgetSlot `json:"slots"`
}

// App.viewMu owns configuration, package handles, stack choices and view cache.
// The widget runtime owns provider/action lifetimes and never publishes inline
// from Configure. Native calls and source joins stay outside App locks.
type appWidgetsRuntime struct {
	core     *widgets.Runtime
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	once     sync.Once
	packages map[string]*widgets.Package
	selected map[string]string
	requests []widgets.Request
	views    map[uint64]LauncherWidgetState
	revision uint64
}

func (a *App) wireWidgetsDefault() { a.wireWidgets(widgets.Providers{}) }

func (a *App) wireProductionWidgets() {
	p := widgets.Providers{Battery: widgetproviders.Battery{Source: platform.NewBatterySource()}, Network: widgetproviders.Network{Source: platform.NewNetworkSource()}, Audio: widgetproviders.Audio{Source: platform.NewAudioOutputSource()}}
	if a.media != nil {
		p.Music = widgetproviders.NewMedia(a.media.controller, platform.MediaMusic)
		p.Spotify = widgetproviders.NewMedia(a.media.controller, platform.MediaSpotify)
	}
	a.wireWidgets(p)
}

// Dependency wiring occurs before the app starts. Tests inject inert providers.
func (a *App) wireWidgets(providers widgets.Providers, packages ...*widgets.Package) {
	if a.widgets != nil {
		a.widgets.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &appWidgetsRuntime{ctx: ctx, cancel: cancel, done: make(chan struct{}), packages: map[string]*widgets.Package{}, selected: map[string]string{}, views: map[uint64]LauncherWidgetState{}}
	for _, p := range append(widgets.Builtins(), packages...) {
		if p != nil {
			r.packages[p.Digest()] = p
		}
	}
	r.core = widgets.NewRuntime(widgets.Deps{Providers: providers, Changed: func([]widgets.InstanceState) {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		if a.widgets == r {
			a.publishWidgetsLocked()
		}
	}})
	a.widgets = r
}

func (a *App) startWidgets() {
	if a.widgets == nil {
		return
	}
	r := a.widgets
	r.once.Do(func() { go func() { defer close(r.done); _ = r.core.Run(r.ctx) }() })
}

func (a *App) widgetPackageLocked(w config.WidgetInstance) *widgets.Package {
	if a.widgets == nil {
		return nil
	}
	// This is the sole legacy grant migration: the exact immutable compiled
	// clock, whose only original authority was clock.read. Community IDs alone
	// never migrate a digest or inherit capabilities.
	if w.PackageID == config.BuiltinClockPackage && w.Digest == config.BuiltinClockDigest {
		p, _ := widgets.Builtin(config.BuiltinClockPackage)
		return p
	}
	p := a.widgets.packages[w.Digest]
	if p == nil || p.Manifest().ID != w.PackageID || !widgetPackageSupported(p) {
		return nil
	}
	return p
}

func widgetCatalogItem(p *widgets.Package) WidgetCatalogItem {
	m := p.Manifest()
	builtin, exists := widgets.Builtin(m.ID)
	return WidgetCatalogItem{PackageID: m.ID, Digest: p.Digest(), Version: m.Version, Name: m.Name, Description: m.Description, RequiredCapabilities: append([]string{}, m.RequiredCapabilities...), OptionalCapabilities: append([]string{}, m.OptionalCapabilities...), Settings: append([]widgets.Setting{}, m.Settings...), Builtin: exists && builtin.Digest() == p.Digest()}
}

func (a *App) GetWidgetCatalog() []WidgetCatalogItem {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	out := []WidgetCatalogItem{}
	if a.widgets == nil || !a.launcherChoicesAllowedLocked() {
		return out
	}
	for _, p := range a.widgets.packages {
		out = append(out, widgetCatalogItem(p))
	}
	slices.SortFunc(out, func(a, b WidgetCatalogItem) int {
		if a.Name["en"] < b.Name["en"] {
			return -1
		}
		if a.Name["en"] > b.Name["en"] {
			return 1
		}
		if a.Digest < b.Digest {
			return -1
		}
		if a.Digest > b.Digest {
			return 1
		}
		return 0
	})
	return out
}

func (a *App) widgetParentLocked(epoch uint64, displayUUID string, session uint64, profileID string) (*appLauncherHost, bool) {
	if a.widgets == nil || a.widgets.ctx.Err() != nil || !a.launcherAllowedLocked() || !a.launcher.ready {
		return nil, false
	}
	h := a.launcher.hosts[session]
	if h == nil || !h.presentation.Visible || h.presentation.Epoch != epoch || h.presentation.DisplayUUID != displayUUID || h.presentation.ProfileID != profileID {
		return nil, false
	}
	for _, p := range a.launcher.core.Snapshot().Presentations {
		if p.Epoch == epoch && p.DisplayUUID == displayUUID && p.Session == session && p.ProfileID == profileID && p.Visible {
			return h, true
		}
	}
	return nil, false
}

func (a *App) widgetProfileLocked(id string) (config.LauncherProfile, bool) {
	for _, p := range a.settingsSnapshot().ReplacementDock.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return config.LauncherProfile{}, false
}

func widgetStackKey(session uint64, stack string) string {
	return strconv.FormatUint(session, 10) + "/" + stack
}

func (a *App) widgetNameLocked(w config.WidgetInstance) widgets.Localized {
	if p := a.widgetPackageLocked(w); p != nil {
		return p.Manifest().Name
	}
	return widgets.Localized{"en": w.PackageID}
}

func (a *App) widgetSlotsLocked(p launcher.Presentation) []LauncherWidgetSlot {
	profile, ok := a.widgetProfileLocked(p.ProfileID)
	if !ok {
		return []LauncherWidgetSlot{}
	}
	instances := map[string]config.WidgetInstance{}
	memberStack := map[string]config.WidgetStack{}
	for _, w := range profile.Widgets {
		instances[w.ID] = w
	}
	for _, stack := range profile.Stacks {
		for _, id := range stack.Members {
			memberStack[id] = stack
		}
	}
	out := []LauncherWidgetSlot{}
	seen := map[string]bool{}
	for _, w := range profile.Widgets {
		slot := LauncherWidgetSlot{ID: w.ID, Name: a.widgetNameLocked(w), SelectedID: w.ID, Members: []LauncherWidgetChoice{}}
		if stack, ok := memberStack[w.ID]; ok {
			if seen[stack.ID] {
				continue
			}
			seen[stack.ID] = true
			enabled := false
			for _, id := range stack.Members {
				enabled = enabled || instances[id].Enabled
			}
			if !enabled {
				continue
			}
			slot.ID, slot.StackID, slot.Name, slot.SelectedID = "stack:"+stack.ID, stack.ID, widgets.Localized{"en": stack.Name}, stack.ActiveID
			if selected := a.widgets.selected[widgetStackKey(p.Session, stack.ID)]; slices.Contains(stack.Members, selected) {
				slot.SelectedID = selected
			}
			for _, id := range stack.Members {
				slot.Members = append(slot.Members, LauncherWidgetChoice{ID: id, Name: a.widgetNameLocked(instances[id])})
			}
		} else if !w.Enabled {
			continue
		}
		out = append(out, slot)
	}
	return out
}

func (a *App) widgetRequestLocked(p launcher.Presentation, id string) (widgets.Request, bool) {
	profile, _ := a.widgetProfileLocked(p.ProfileID)
	for _, w := range profile.Widgets {
		if w.ID != id {
			continue
		}
		pkg := a.widgetPackageLocked(w)
		if pkg == nil {
			break
		}
		manifest := pkg.Manifest()
		if _, err := widgets.ValidateSettings(manifest, w.Settings); err != nil {
			break
		}
		for _, cap := range w.Grants {
			if !slices.Contains(manifest.RequiredCapabilities, cap) && !slices.Contains(manifest.OptionalCapabilities, cap) {
				return widgets.Request{}, false
			}
		}
		return widgets.Request{Package: pkg, ControllerEpoch: p.Epoch, DisplayUUID: p.DisplayUUID, Session: p.Session, ProfileID: p.ProfileID, InstanceID: w.ID, Enabled: w.Enabled, Grants: w.Grants, Settings: w.Settings}, true
	}
	return widgets.Request{}, false
}

func (a *App) syncWidgetsLocked() {
	r := a.widgets
	if r == nil {
		return
	}
	requests := []widgets.Request{}
	activeStacks := map[string]bool{}
	if a.launcher != nil && a.launcherAllowedLocked() && a.launcher.ready {
		for _, p := range a.launcher.core.Snapshot().Presentations {
			if _, ok := a.widgetParentLocked(p.Epoch, p.DisplayUUID, p.Session, p.ProfileID); !ok {
				continue
			}
			for _, slot := range a.widgetSlotsLocked(p) {
				if slot.StackID != "" {
					activeStacks[widgetStackKey(p.Session, slot.StackID)] = true
				}
				if q, ok := a.widgetRequestLocked(p, slot.SelectedID); ok {
					requests = append(requests, q)
				}
			}
		}
	}
	for key := range r.selected {
		if !activeStacks[key] {
			delete(r.selected, key)
		}
	}
	if !reflect.DeepEqual(r.requests, requests) {
		if err := r.core.Configure(requests); err != nil {
			_ = r.core.Configure(nil)
			requests = nil
		}
		r.requests = requests
	}
	a.publishWidgetsLocked()
}

func (a *App) widgetStateLocked(session uint64) LauncherWidgetState {
	out := LauncherWidgetState{Session: session, Slots: []LauncherWidgetSlot{}}
	if a.widgets == nil || a.launcher == nil {
		return out
	}
	out.Revision = a.widgets.revision
	h := a.launcher.hosts[session]
	if h == nil {
		return out
	}
	p := h.presentation
	out.Epoch, out.DisplayUUID, out.ProfileID = p.Epoch, p.DisplayUUID, p.ProfileID
	if _, ok := a.widgetParentLocked(p.Epoch, p.DisplayUUID, session, p.ProfileID); !ok {
		return out
	}
	out.Visible = true
	out.Slots = a.widgetSlotsLocked(p)
	states := a.widgets.core.Snapshot()
	for j := range out.Slots {
		slot := &out.Slots[j]
		slot.Status = "unavailable"
		if _, ok := a.widgetRequestLocked(p, slot.SelectedID); ok {
			slot.Status = "preparing"
		}
		for _, state := range states {
			l := state.Lease
			if l.ControllerEpoch == p.Epoch && l.DisplayUUID == p.DisplayUUID && l.Session == p.Session && l.ProfileID == p.ProfileID && l.InstanceID == slot.SelectedID {
				slot.State, slot.Status = &state, state.Status
				break
			}
		}
	}
	return out
}

func (a *App) publishWidgetsLocked() {
	r := a.widgets
	if r == nil {
		return
	}
	wanted := map[uint64]bool{}
	if a.launcher != nil {
		for session := range a.launcher.hosts {
			wanted[session] = true
		}
	}
	for session := range r.views {
		wanted[session] = true
	}
	for session := range wanted {
		next := a.widgetStateLocked(session)
		previous, exists := r.views[session]
		next.Revision = previous.Revision
		if reflect.DeepEqual(previous, next) || (!exists && !next.Visible) {
			continue
		}
		r.revision++
		next.Revision = r.revision
		if next.Visible {
			r.views[session] = next
		} else {
			delete(r.views, session)
		}
		a.emit("launcher:widgets", next)
	}
}

func (a *App) GetLauncherWidgets(session uint64) LauncherWidgetState {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	return a.widgetStateLocked(session)
}

func (a *App) widgetActionGuard(lease widgets.Lease) (*appWidgetsRuntime, func() error, error) {
	a.viewMu.Lock()
	r := a.widgets
	host, ok := a.widgetParentLocked(lease.ControllerEpoch, lease.DisplayUUID, lease.Session, lease.ProfileID)
	var token uint64
	if ok {
		if panel, isPanel := host.window.wheelPanel().(platform.LauncherPanel); isPanel {
			token = panel.LauncherToken()
		}
	}
	a.viewMu.Unlock()
	if !ok || token == 0 {
		return nil, nil, widgets.ErrRetired
	}
	logical := func() error {
		a.viewMu.Lock()
		defer a.viewMu.Unlock()
		current, allowed := a.widgetParentLocked(lease.ControllerEpoch, lease.DisplayUUID, lease.Session, lease.ProfileID)
		if !allowed || a.widgets != r || current != host {
			return widgets.ErrRetired
		}
		panel, isPanel := current.window.wheelPanel().(platform.LauncherPanel)
		if !isPanel || panel.LauncherToken() != token {
			return widgets.ErrRetired
		}
		return nil
	}
	guard := func() error {
		if err := logical(); err != nil {
			return err
		}
		validator, ok := host.window.wheelPanel().(platform.LauncherPanelValidator)
		if !ok {
			return widgets.ErrRetired
		}
		// Native AppKit/Space validation can reenter Go. Never hold viewMu
		// across it, and reread logical admission after that external work.
		if err := validator.ValidateLauncherPanel(r.ctx, lease.DisplayUUID); err != nil {
			return widgets.ErrRetired
		}
		return logical()
	}
	return r, guard, nil
}

func (a *App) GetWidgetActionOptions(lease widgets.Lease, actionToken string) (widgets.ActionOptions, error) {
	r, guard, err := a.widgetActionGuard(lease)
	if err != nil {
		return widgets.ActionOptions{}, err
	}
	options, err := r.core.ActionOptions(r.ctx, lease, actionToken)
	if err == nil {
		err = guard()
	}
	return options, err
}

func (a *App) PerformWidgetAction(lease widgets.Lease, actionToken, optionToken string, value *float64) error {
	r, guard, err := a.widgetActionGuard(lease)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.ctx, 3*time.Second)
	defer cancel()
	return r.core.Perform(ctx, lease, actionToken, optionToken, value, guard)
}

func (a *App) GetWidgetAsset(lease widgets.Lease, assetToken string) (string, error) {
	r, guard, err := a.widgetActionGuard(lease)
	if err != nil {
		return "", err
	}
	data, err := r.core.Asset(lease, assetToken)
	if err != nil {
		return "", err
	}
	if err = guard(); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func (a *App) SelectLauncherWidget(epoch uint64, displayUUID string, session uint64, profileID, stackID, instanceID string) error {
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if _, ok := a.widgetParentLocked(epoch, displayUUID, session, profileID); !ok {
		return widgets.ErrRetired
	}
	profile, ok := a.widgetProfileLocked(profileID)
	if !ok {
		return widgets.ErrRetired
	}
	for _, stack := range profile.Stacks {
		if stack.ID == stackID && slices.Contains(stack.Members, instanceID) {
			a.widgets.selected[widgetStackKey(session, stackID)] = instanceID
			a.syncWidgetsLocked()
			return nil
		}
	}
	return widgets.ErrInvalid
}
