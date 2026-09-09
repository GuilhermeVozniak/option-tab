package launcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"image/png"
	"reflect"
	"slices"
	"strings"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func cloneItems(items []Item) []Item {
	out := append([]Item{}, items...)
	for i := range out {
		if out[i].Members != nil {
			out[i].Members = cloneItems(out[i].Members)
		}
	}
	return out
}

// Compose only copied, already-resolved references. No source, policy callback
// or filesystem operation may run while the controller holds its lock.
func (c *Controller) composeItemsLocked(profile config.LauncherProfile) []Item {
	byID := make(map[string]Item, len(profile.Items))
	hidden := map[string]bool{}
	merged := map[string]bool{}
	for _, configured := range profile.Items {
		item := Item{ID: "pin:" + configured.ID, Kind: configured.Kind, Name: configured.Label, Icon: c.itemIcons[configured.IconID], Status: "ready"}
		if configured.ReferenceID != "" {
			ref, ok := c.references[configured.ReferenceID]
			item.Status = "needsSelection"
			if ok {
				item.reference = ref
				item.ReferenceRevision = ref.Revision
				item.Status = ref.State
				if ref.ID != configured.ReferenceID || ref.Kind != configured.Kind {
					item.Status = "changed"
				} else if !slices.Contains([]string{"ready", "moved", "missing", "accessRequired", "changed", "unsupported", "unavailable", "excluded", "needsSelection"}, ref.State) || (ref.State == "ready" && ref.Revision == 0) {
					item.Status = "unavailable"
				}
				if configured.Kind == "app" && ((c.deps.SelfBundleID != "" && ref.BundleID == c.deps.SelfBundleID) || (c.deps.SelfAppID > 0 && ref.Process.PID == c.deps.SelfAppID)) {
					item.Status = "excluded"
				}
				if item.Status == "ready" && configured.Kind == "app" && ref.Process.PID > 0 && ref.Process.StartSeconds > 0 {
					for _, running := range c.items {
						target, exists := c.targets[running.ID]
						if exists && target.Process == ref.Process && target.BundleID == ref.BundleID {
							item.Running = true
							if item.Icon == "" {
								item.Icon = running.Icon
							}
							merged[running.ID] = true
							break
						}
					}
				}
			}
		}
		byID[configured.ID] = item
		for _, member := range configured.Members {
			hidden[member] = true
		}
	}
	out := make([]Item, 0, len(profile.Items)+len(c.items))
	for _, configured := range profile.Items {
		if hidden[configured.ID] {
			continue
		}
		item := byID[configured.ID]
		for _, id := range configured.Members {
			if member, ok := byID[id]; ok {
				item.Members = append(item.Members, member)
			}
		}
		out = append(out, item)
	}
	for _, item := range c.items {
		if !merged[item.ID] {
			item.Kind, item.Status, item.Running = "app", "ready", true
			out = append(out, item)
		}
	}
	return out
}

// Reference metadata is kept outside the renderer's copied presentation.
type ConfiguredTarget struct {
	Item      config.LauncherItem
	Reference platform.LauncherReference
}

func (c *Controller) configuredItemLocked(scope Scope, id string) (ConfiguredTarget, error) {
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) || !strings.HasPrefix(id, "pin:") {
		return ConfiguredTarget{}, ErrRetired
	}
	for _, p := range c.state.Presentations {
		if p.Scope != scope || !p.Visible || p.ProfileID != c.profileForDisplayLocked(scope.DisplayUUID) {
			continue
		}
		for _, profile := range c.settings.Profiles {
			if profile.ID != p.ProfileID {
				continue
			}
			for _, x := range profile.Items {
				if "pin:"+x.ID != id || !slices.Contains([]string{"app", "file", "folder", "link"}, x.Kind) {
					continue
				}
				ready := false
				currentReference := c.references[x.ReferenceID]
				for _, visible := range p.Items {
					ready = ready || (visible.ID == id && visible.Status == "ready" && visible.reference == currentReference)
					for _, member := range visible.Members {
						ready = ready || (member.ID == id && member.Status == "ready" && member.reference == currentReference)
					}
				}
				if !ready {
					return ConfiguredTarget{}, ErrUnavailable
				}
				x.Members = slices.Clone(x.Members)
				return ConfiguredTarget{Item: x, Reference: c.references[x.ReferenceID]}, nil
			}
		}
	}
	return ConfiguredTarget{}, ErrRetired
}

func (c *Controller) ConfiguredItem(scope Scope, id string) (ConfiguredTarget, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.configuredItemLocked(scope, id)
}

func (c *Controller) PerformConfigured(ctx context.Context, scope Scope, id, action string) error {
	return c.PerformConfiguredGuarded(ctx, scope, id, action, nil)
}

func (c *Controller) PerformConfiguredGuarded(ctx context.Context, scope Scope, id, action string, extra func() error) error {
	if ctx == nil {
		return ErrRetired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if action != "open" && action != "relaunch" {
		return ErrUnavailable
	}
	c.mu.Lock()
	target, err := c.configuredItemLocked(scope, id)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	if action == "relaunch" && (target.Item.Kind != "app" || target.Reference.Process.PID <= 0 || target.Reference.Process.StartSeconds == 0) {
		c.mu.Unlock()
		return ErrUnavailable
	}
	if c.actionBusy {
		c.mu.Unlock()
		return ErrBusy
	}
	if c.deps.PerformItem == nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	profileID := c.profileForDisplayLocked(scope.DisplayUUID)
	c.actionBusy = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.actionBusy = false; c.mu.Unlock() }()
	guard := func() error {
		if extra != nil {
			if err := extra(); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		// Initial UI admission above is exact. Once admitted, content ticks do
		// not revoke the same visible session/profile/resource authority.
		latest := Scope{}
		for _, p := range c.state.Presentations {
			if p.Epoch == scope.Epoch && p.DisplayUUID == scope.DisplayUUID && p.Session == scope.Session && p.ProfileID == profileID && p.Visible {
				latest = p.Scope
				break
			}
		}
		if latest.Session == 0 {
			return ErrRetired
		}
		if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) || c.profileForDisplayLocked(scope.DisplayUUID) != profileID {
			return ErrRetired
		}
		// Inventory can precede presentation publication. Compare its current
		// authority directly; the initial visible-item gate already admitted
		// this exact item and is intentionally not repeated against old pixels.
		current := ConfiguredTarget{}
		found := false
		for _, profile := range c.settings.Profiles {
			if profile.ID != profileID {
				continue
			}
			for _, item := range profile.Items {
				if "pin:"+item.ID == id {
					current = ConfiguredTarget{Item: item, Reference: c.references[item.ReferenceID]}
					found = true
					break
				}
			}
		}
		if !found {
			return ErrRetired
		}
		if action == "relaunch" && target.Reference.Process.PID > 0 && target.Reference.Process.StartSeconds > 0 && current.Reference.Process == (platform.ProcessIdentity{}) {
			// The native operation owns termination of the captured process.
			// A different positive process is never substituted for it.
			current.Reference.Process = target.Reference.Process
		}
		if !reflect.DeepEqual(current, target) {
			return ErrRetired
		}
		return nil
	}
	return c.deps.PerformItem(ctx, scope, target, action, guard)
}

// One inventory worker resolves selected resources without holding the owner
// lock. Retirement cancels it immediately; Run joins it before another read.
func (c *Controller) readConfiguredInventory(ctx context.Context, out *inventory) {
	c.mu.Lock()
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != out.epoch {
		c.mu.Unlock()
		return
	}
	settings := config.CloneReplacementDock(c.settings)
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	c.inventoryCancel = cancel
	c.mu.Unlock()
	defer func() { cancel(); c.mu.Lock(); c.inventoryCancel = nil; c.mu.Unlock() }()
	refs := map[string]platform.LauncherReference{}
	icons := map[string]string{}
	for _, p := range settings.Profiles {
		for _, x := range p.Items {
			if readCtx.Err() != nil {
				return
			}
			if x.ReferenceID != "" {
				if _, seen := refs[x.ReferenceID]; !seen {
					r := platform.LauncherReference{ID: x.ReferenceID, Kind: x.Kind, State: "needsSelection"}
					if c.deps.ResolveReference != nil {
						if resolved, err := c.deps.ResolveReference(readCtx, x.ReferenceID); err == nil && resolved.ID == x.ReferenceID {
							r = resolved
						}
					}
					if readCtx.Err() != nil {
						return
					}
					if r.Kind == "app" && c.deps.EligibleReference != nil && !c.deps.EligibleReference(r) {
						r.State = "excluded"
					}
					refs[x.ReferenceID] = r
				}
			}
			if x.IconID != "" && c.deps.ReadItemIcon != nil {
				if _, seen := icons[x.IconID]; !seen {
					icons[x.IconID] = ""
					data, err := c.deps.ReadItemIcon(readCtx, x.IconID)
					if readCtx.Err() != nil {
						return
					}
					if err == nil && len(data) <= 512*1024 {
						if cfg, err := png.DecodeConfig(bytes.NewReader(data)); err == nil && cfg.Width > 0 && cfg.Height > 0 && cfg.Width <= 256 && cfg.Height <= 256 {
							icons[x.IconID] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
						}
					}
				}
			}
		}
	}
	if readCtx.Err() == nil {
		out.references, out.icons = refs, icons
	}
}
