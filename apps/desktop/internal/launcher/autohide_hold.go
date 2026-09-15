package launcher

import "time"

// A renderer must renew only while a click, drag or menu is still owned. A
// missing cancel or destroyed webview cannot leave an unbounded visibility hold.
const autoHideHoldLifetime = 2 * time.Second

type autoHideHold struct {
	authority InteractionAuthority
	sequence  uint64
	expires   time.Time
}

// SetAutoHideHold never reveals a hidden panel or overrides native recovery.
// Sequence tombstones stop a delayed begin/heartbeat from undoing a release.
func (c *Controller) SetAutoHideHold(scope Scope, sequence uint64, phase string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if sequence == 0 || (phase != "begin" && phase != "renew" && phase != "end") {
		return ErrRetired
	}
	current, err := c.captureInteractionLocked(scope)
	if err != nil {
		return ErrRetired
	}
	hold, exists := c.autoHideHolds[scope.DisplayUUID]
	if exists && hold.authority.Scope.Epoch == scope.Epoch && hold.authority.Scope.Session == scope.Session {
		if sequence <= hold.sequence {
			return ErrRetired
		}
	} else {
		hold = autoHideHold{authority: current}
		exists = false
	}
	switch phase {
	case "renew":
		if !exists || hold.expires.IsZero() || !c.deps.Now().Before(hold.expires) {
			return ErrRetired
		}
		if _, err := c.validateInteractionLocked(hold.authority); err != nil {
			return ErrRetired
		}
	case "begin":
		// Fresh input must name the current rendered revision. A heartbeat has
		// no such re-admission authority after items or geometry change.
		hold.authority = current
	}
	hold.sequence = sequence
	hold.expires = time.Time{}
	if phase != "end" {
		hold.expires = c.deps.Now().Add(autoHideHoldLifetime)
	}
	if c.autoHideHolds == nil {
		c.autoHideHolds = map[string]autoHideHold{}
	}
	c.autoHideHolds[scope.DisplayUUID] = hold
	c.notify()
	return nil
}

func (c *Controller) autoHideHeldLocked(p Presentation) bool {
	hold, exists := c.autoHideHolds[p.DisplayUUID]
	if !exists || hold.expires.IsZero() || !c.deps.Now().Before(hold.expires) {
		return false
	}
	if p.Epoch != hold.authority.Scope.Epoch || p.Session != hold.authority.Scope.Session {
		delete(c.autoHideHolds, p.DisplayUUID)
		return false
	}
	_, err := c.validateInteractionLocked(hold.authority)
	return err == nil
}
