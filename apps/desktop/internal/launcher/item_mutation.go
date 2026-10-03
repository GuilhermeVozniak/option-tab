package launcher

import (
	"strings"

	"option-tab/internal/config"
)

// ItemMutationAuthority is Go-only structural authority for one exact presentation.
type ItemMutationAuthority struct {
	Scope         Scope
	ProfileID     string
	ItemsRevision string
	ItemID        string
	TargetID      string
	Admission     uint64
}

func (c *Controller) CaptureItemMutation(scope Scope, expectedItemsRevision, itemID, targetID string) (ItemMutationAuthority, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.captureItemMutationLocked(scope, expectedItemsRevision, itemID, targetID)
}

func (c *Controller) captureItemMutationLocked(scope Scope, hash, itemID, targetID string) (ItemMutationAuthority, error) {
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) {
		return ItemMutationAuthority{}, ErrRetired
	}
	if !strings.HasPrefix(itemID, "pin:") || !strings.HasPrefix(targetID, "pin:") || itemID == targetID {
		return ItemMutationAuthority{}, ErrUnavailable
	}
	for _, p := range c.state.Presentations {
		if p.Scope != scope || !p.Visible || p.ProfileID != c.profileForDisplayLocked(scope.DisplayUUID) {
			continue
		}
		visible := func(id string) bool {
			for _, item := range p.Items {
				if item.ID == id {
					return true
				}
				for _, member := range item.Members {
					if member.ID == id {
						return true
					}
				}
			}
			return false
		}
		if !visible(itemID) || !visible(targetID) {
			return ItemMutationAuthority{}, ErrUnavailable
		}
		for _, profile := range c.settings.Profiles {
			if profile.ID != p.ProfileID {
				continue
			}
			if !profile.RuntimeReorder || hash == "" || hash != p.ItemsRevision || hash != config.LauncherItemsRevision(profile.Items) {
				return ItemMutationAuthority{}, ErrRetired
			}
			source, target := false, false
			for _, item := range profile.Items {
				if item.ID == strings.TrimPrefix(itemID, "pin:") {
					source = item.Kind != "spacer" && item.Kind != "separator"
				}
				target = target || item.ID == strings.TrimPrefix(targetID, "pin:")
			}
			if !source || !target {
				return ItemMutationAuthority{}, ErrUnavailable
			}
			return ItemMutationAuthority{Scope: scope, ProfileID: profile.ID, ItemsRevision: hash, ItemID: strings.TrimPrefix(itemID, "pin:"), TargetID: strings.TrimPrefix(targetID, "pin:"), Admission: c.childAdmissionLocked(scope.DisplayUUID)}, nil
		}
	}
	return ItemMutationAuthority{}, ErrRetired
}

func (c *Controller) ValidateItemMutation(a ItemMutationAuthority) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if a.Admission == 0 || c.childAdmissions[a.Scope.DisplayUUID] != a.Admission {
		return ErrRetired
	}
	current, err := c.captureItemMutationLocked(a.Scope, a.ItemsRevision, "pin:"+a.ItemID, "pin:"+a.TargetID)
	if err != nil {
		return err
	}
	if current != a {
		return ErrRetired
	}
	return nil
}
