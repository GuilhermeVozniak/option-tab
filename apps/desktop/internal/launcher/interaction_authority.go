package launcher

import (
	"reflect"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// InteractionAuthority is private to Go owners. Its initial Scope stays fixed
// while ValidateInteraction returns the current content-only revision.
type InteractionAuthority struct {
	Scope     Scope
	ProfileID string
	Display   platform.LauncherDisplay
	Bounds    domain.Bounds
	Admission uint64
	settings  config.ReplacementDockSettings
	items     []Item
	targets   map[string]platform.LauncherAppTarget
}

// NavigationItems intentionally flattens ready group members; selecting one lets
// the renderer reveal that owning group. It never includes group containers.
func (a InteractionAuthority) NavigationItems() []LetterItem {
	out := []LetterItem{}
	var appendItems func([]Item)
	appendItems = func(items []Item) {
		for _, item := range items {
			if item.Kind == "group" {
				appendItems(item.Members)
				continue
			}
			if item.Status != "ready" {
				continue
			}
			switch item.Kind {
			case "app", "file", "folder", "link":
				out = append(out, LetterItem{ID: item.ID, Name: item.Name, Visible: true, Actionable: true})
			}
		}
	}
	appendItems(a.items)
	return out
}

func (c *Controller) CaptureInteraction(scope Scope) (InteractionAuthority, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captureInteractionLocked(scope)
}

func (c *Controller) captureInteractionLocked(scope Scope) (InteractionAuthority, error) {
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) {
		return InteractionAuthority{}, ErrRetired
	}
	for _, p := range c.state.Presentations {
		if p.Scope != scope || !p.Visible || p.ProfileID != c.profileForDisplayLocked(scope.DisplayUUID) {
			continue
		}
		var profile *config.LauncherProfile
		for i := range c.settings.Profiles {
			if c.settings.Profiles[i].ID == p.ProfileID {
				profile = &c.settings.Profiles[i]
				break
			}
		}
		if profile == nil || !reflect.DeepEqual(p.Items, c.composeItemsLocked(*profile)) {
			return InteractionAuthority{}, ErrRetired
		}
		a := InteractionAuthority{Scope: scope, ProfileID: p.ProfileID, Bounds: p.Bounds, Admission: c.childAdmissionLocked(scope.DisplayUUID), settings: config.CloneReplacementDock(c.settings), items: cloneItems(p.Items), targets: map[string]platform.LauncherAppTarget{}}
		found := false
		for _, display := range c.env.Displays {
			if display.UUID == scope.DisplayUUID {
				a.Display = display
				found = true
				break
			}
		}
		if !found {
			return InteractionAuthority{}, ErrRetired
		}
		count := 0
		for _, item := range p.Items {
			count++
			count += len(item.Members)
			if !strings.HasPrefix(item.ID, "pin:") {
				target, ok := c.targets[item.ID]
				if !ok || !c.childAppEligible(target) {
					return InteractionAuthority{}, ErrUnavailable
				}
				a.targets[item.ID] = target
			}
		}
		if count > 144 {
			return InteractionAuthority{}, ErrUnavailable
		}
		return a, nil
	}
	return InteractionAuthority{}, ErrRetired
}

// ValidateInteraction performs only bounded in-memory checks. No native work,
// source calls, clock reads or callbacks run while core.mu is held.
func (c *Controller) ValidateInteraction(a InteractionAuthority) (Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.Admission == 0 || c.childAdmissions[a.Scope.DisplayUUID] != a.Admission {
		return Scope{}, ErrRetired
	}
	latest := Scope{}
	for _, p := range c.state.Presentations {
		if p.Epoch == a.Scope.Epoch && p.Session == a.Scope.Session && p.DisplayUUID == a.Scope.DisplayUUID && p.Revision >= a.Scope.Revision {
			latest = p.Scope
			break
		}
	}
	if latest.Session == 0 {
		return Scope{}, ErrRetired
	}
	current, err := c.captureInteractionLocked(latest)
	if err != nil {
		return Scope{}, err
	}
	current.Scope = a.Scope
	if !reflect.DeepEqual(current, a) {
		return Scope{}, ErrRetired
	}
	return latest, nil
}
