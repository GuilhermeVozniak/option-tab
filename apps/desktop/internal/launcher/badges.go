package launcher

import (
	"reflect"
	"slices"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// BadgeItem and BadgeAuthority are private Go observation authority, never
// renderer DTOs. Each configured application retains its selected reference.
type BadgeItem struct {
	ItemID    string
	Reference platform.LauncherReference
	App       platform.LauncherAppTarget
}

type BadgeAuthority struct {
	Scope     Scope
	ProfileID string
	Admission uint64
	display   platform.LauncherDisplay
	bounds    domain.Bounds
	profile   config.LauncherProfile
	items     []BadgeItem
}

func (a BadgeAuthority) Items() []BadgeItem { return slices.Clone(a.items) }

func (c *Controller) CaptureBadges(scope Scope) (BadgeAuthority, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captureBadgesLocked(scope)
}

func (c *Controller) captureBadgesLocked(scope Scope) (BadgeAuthority, error) {
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) {
		return BadgeAuthority{}, ErrRetired
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
		if profile == nil || !profile.ShowBadges || !reflect.DeepEqual(p.Items, c.composeItemsLocked(*profile)) {
			return BadgeAuthority{}, ErrRetired
		}
		a := BadgeAuthority{Scope: scope, ProfileID: p.ProfileID, Admission: c.childAdmissionLocked(scope.DisplayUUID), bounds: p.Bounds, profile: config.CloneReplacementDock(config.ReplacementDockSettings{Profiles: []config.LauncherProfile{*profile}}).Profiles[0], items: []BadgeItem{}}
		found := false
		for _, display := range c.env.Displays {
			if display.UUID == scope.DisplayUUID {
				a.display = display
				found = true
				break
			}
		}
		if !found {
			return BadgeAuthority{}, ErrRetired
		}
		var collect func([]Item) error
		collect = func(items []Item) error {
			for _, item := range items {
				if item.Kind == "group" {
					if err := collect(item.Members); err != nil {
						return err
					}
					continue
				}
				if item.Kind != "app" || item.Status != "ready" {
					continue
				}
				entry := BadgeItem{ItemID: item.ID}
				if strings.HasPrefix(item.ID, "pin:") {
					entry.Reference = item.reference
					if entry.Reference.Kind != "app" || entry.Reference.ID == "" || entry.Reference.Revision == 0 || !config.ValidLauncherBundleID(entry.Reference.BundleID) {
						continue
					}
				} else {
					var exists bool
					entry.App, exists = c.targets[item.ID]
					if !exists || !c.childAppEligible(entry.App) {
						return ErrUnavailable
					}
				}
				a.items = append(a.items, entry)
				if len(a.items) > 144 {
					return ErrUnavailable
				}
			}
			return nil
		}
		if err := collect(p.Items); err != nil {
			return BadgeAuthority{}, err
		}
		return a, nil
	}
	return BadgeAuthority{}, ErrRetired
}

// Widget content ticks may advance the parent revision. Native display
// transitions, configured references and exact running targets may not change.
func (c *Controller) ValidateBadges(a BadgeAuthority) (Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.Admission == 0 || c.childAdmissions[a.Scope.DisplayUUID] != a.Admission {
		return Scope{}, ErrRetired
	}
	for _, p := range c.state.Presentations {
		if p.Epoch != a.Scope.Epoch || p.Session != a.Scope.Session || p.DisplayUUID != a.Scope.DisplayUUID || p.Revision < a.Scope.Revision {
			continue
		}
		current, err := c.captureBadgesLocked(p.Scope)
		if err != nil {
			return Scope{}, err
		}
		current.Scope = a.Scope
		if !reflect.DeepEqual(current, a) {
			return Scope{}, ErrRetired
		}
		return p.Scope, nil
	}
	return Scope{}, ErrRetired
}
