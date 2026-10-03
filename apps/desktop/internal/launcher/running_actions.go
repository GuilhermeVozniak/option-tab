package launcher

import (
	"context"
	"strings"
)

// Relaunch retains configured-reference authority for pins. A running-only
// item instead captures its own process target; the platform owns temporary
// bundle-resource capture and graceful termination/opening of that exact app.
func (c *Controller) Relaunch(ctx context.Context, scope Scope, id string) error {
	if strings.HasPrefix(id, "pin:") {
		return c.PerformConfigured(ctx, scope, id, "relaunch")
	}
	if ctx == nil {
		return ErrRetired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	if !c.currentLocked(scope, id) {
		c.mu.Unlock()
		return ErrRetired
	}
	if c.actionBusy {
		c.mu.Unlock()
		return ErrBusy
	}
	if c.deps.Relaunch == nil {
		c.mu.Unlock()
		return ErrUnavailable
	}
	target := c.targets[id]
	if target.Process.PID <= 0 || target.Process.StartSeconds == 0 || target.BundleID == "" {
		c.mu.Unlock()
		return ErrUnavailable
	}
	profile := c.profileForDisplayLocked(scope.DisplayUUID)
	c.actionBusy = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.actionBusy = false; c.mu.Unlock() }()
	guard := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) || c.profileForDisplayLocked(scope.DisplayUUID) != profile {
			return ErrRetired
		}
		visible := false
		for _, p := range c.state.Presentations {
			visible = visible || (p.Epoch == scope.Epoch && p.Session == scope.Session && p.DisplayUUID == scope.DisplayUUID && p.ProfileID == profile && p.Visible)
		}
		if !visible {
			return ErrRetired
		}
		if current, exists := c.targets[id]; exists && current != target {
			return ErrRetired
		}
		// Native termination can remove this item before its completion. Absence
		// is permitted only inside this admitted operation; the native owner
		// still proves exact termination and no replacement before reopening.
		for _, current := range c.targets {
			if (current.Process.PID == target.Process.PID || current.BundleID == target.BundleID) && current.Process != target.Process {
				return ErrRetired
			}
		}
		return nil
	}
	native := target
	native.DisplayUUID = scope.DisplayUUID
	return c.deps.Relaunch(ctx, scope, native, guard)
}
