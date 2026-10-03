package launcher

import (
	"math"
	"reflect"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// ChildParent is Go-only authority captured from exact visible parent pixels.
// Revision may advance for content ticks; Admission cannot return after a
// display transition, even when the observer coalesces A→B→A before publishing.
type ChildParent struct {
	Scope             Scope
	ProfileID, ItemID string
	Configured        config.LauncherItem
	Reference         platform.LauncherReference
	App               platform.LauncherAppTarget
	Display           platform.LauncherDisplay
	Bounds            domain.Bounds
	Admission         uint64
}

type childHold struct {
	parent  ChildParent
	session uint64
	bounds  domain.Bounds
}

func (c *Controller) CaptureChildParent(scope Scope, id string) (ChildParent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) {
		return ChildParent{}, ErrRetired
	}
	for _, p := range c.state.Presentations {
		if p.Scope != scope || !p.Visible || p.ProfileID != c.profileForDisplayLocked(scope.DisplayUUID) {
			continue
		}
		visible := false
		for _, item := range p.Items {
			visible = visible || item.ID == id
			for _, member := range item.Members {
				visible = visible || member.ID == id
			}
		}
		if !visible {
			return ChildParent{}, ErrUnavailable
		}
		out := ChildParent{Scope: scope, ProfileID: p.ProfileID, ItemID: id, Bounds: p.Bounds, Admission: c.childAdmissionLocked(scope.DisplayUUID)}
		for _, display := range c.env.Displays {
			if display.UUID == scope.DisplayUUID {
				out.Display = display
				break
			}
		}
		if strings.HasPrefix(id, "pin:") {
			target, err := c.configuredItemLocked(scope, id)
			if err != nil {
				return ChildParent{}, err
			}
			out.Configured, out.Reference = target.Item, target.Reference
			switch target.Item.Kind {
			case "folder":
			case "app":
				out.App = platform.LauncherAppTarget{Process: target.Reference.Process, BundleID: target.Reference.BundleID, Name: target.Reference.Label}
			default:
				return ChildParent{}, ErrUnavailable
			}
		} else {
			out.App = c.targets[id]
		}
		if out.Configured.Kind != "folder" {
			if !c.childAppEligible(out.App) {
				return ChildParent{}, ErrUnavailable
			}
			out.App.DisplayUUID = scope.DisplayUUID
		}
		return out, nil
	}
	return ChildParent{}, ErrRetired
}

func (c *Controller) childAppEligible(app platform.LauncherAppTarget) bool {
	return app.Process.PID > 0 && app.Process.StartSeconds > 0 && app.Process.StartMicros < 1_000_000 && app.BundleID != "" && app.Process.PID != c.deps.SelfAppID && (c.deps.SelfBundleID == "" || app.BundleID != c.deps.SelfBundleID)
}

func (c *Controller) childAdmissionLocked(uuid string) uint64 {
	if c.childAdmissions == nil {
		c.childAdmissions = map[string]uint64{}
	}
	if c.childAdmissions[uuid] == 0 {
		c.nextChildAdmission++
		c.childAdmissions[uuid] = c.nextChildAdmission
	}
	return c.childAdmissions[uuid]
}

func (c *Controller) ValidateChildParent(parent ChildParent) (Scope, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.validateChildParentLocked(parent)
}

func (c *Controller) validateChildParentLocked(parent ChildParent) (Scope, error) {
	scope := parent.Scope
	if parent.Admission == 0 || c.childAdmissions[scope.DisplayUUID] != parent.Admission || c.closed || c.suspended || !c.settings.Enabled || c.epoch != scope.Epoch || !c.inventoryReady || !c.ordinaryLocked(scope.DisplayUUID) || c.profileForDisplayLocked(scope.DisplayUUID) != parent.ProfileID {
		return Scope{}, ErrRetired
	}
	displayCurrent := false
	for _, display := range c.env.Displays {
		if display == parent.Display {
			displayCurrent = true
			break
		}
	}
	if !displayCurrent {
		return Scope{}, ErrRetired
	}
	latest := Scope{}
	for _, p := range c.state.Presentations {
		if p.Epoch == scope.Epoch && p.Session == scope.Session && p.DisplayUUID == scope.DisplayUUID && p.ProfileID == parent.ProfileID && p.Visible && p.Bounds == parent.Bounds {
			latest = p.Scope
			break
		}
	}
	if latest.Session == 0 {
		return Scope{}, ErrRetired
	}
	if parent.Configured.ID != "" {
		found := false
		for _, profile := range c.settings.Profiles {
			if profile.ID != parent.ProfileID {
				continue
			}
			for _, item := range profile.Items {
				if "pin:"+item.ID == parent.ItemID && reflect.DeepEqual(item, parent.Configured) {
					found = true
					break
				}
			}
		}
		if !found || c.references[parent.Configured.ReferenceID] != parent.Reference {
			return Scope{}, ErrRetired
		}
	} else {
		current := c.targets[parent.ItemID]
		current.DisplayUUID = scope.DisplayUUID
		if current != parent.App || !c.childAppEligible(current) {
			return Scope{}, ErrRetired
		}
	}
	return latest, nil
}

// Called synchronously after accepting an environment sample, before Run can
// coalesce it. Only affected display admissions retire.
func (c *Controller) retireChangedChildDisplaysLocked(previous platform.LauncherEnvironment) {
	old := map[string]platform.LauncherDisplay{}
	for _, display := range previous.Displays {
		old[display.UUID] = display
	}
	current := map[string]platform.LauncherDisplay{}
	for _, display := range c.env.Displays {
		current[display.UUID] = display
	}
	ready := c.environmentReadyLocked()
	for uuid := range c.childAdmissions {
		display, exists := current[uuid]
		if c.env.Complete && !exists {
			delete(c.childAdmissions, uuid)
			delete(c.childSessions, uuid)
			delete(c.childHolds, uuid)
			continue
		}
		if !ready || !exists || display != old[uuid] || previous.Generation != c.env.Generation || c.protectedPointer(display) {
			c.nextChildAdmission++
			c.childAdmissions[uuid] = c.nextChildAdmission
			delete(c.childHolds, uuid)
		}
	}
}

// PlaceChild uses backend geometry only. It reserves the physical recovery
// inset on all edges and tries inward placement first, shrinking only when the
// available region cannot fit the requested useful size.
func (c *Controller) PlaceChild(parent ChildParent, width, height float64) (domain.Bounds, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.placeChildLocked(parent, width, height)
}

func (c *Controller) placeChildLocked(parent ChildParent, width, height float64) (domain.Bounds, error) {
	if _, err := c.validateChildParentLocked(parent); err != nil {
		return domain.Bounds{}, err
	}
	if !finite(width) || !finite(height) || width < 120 || height < 96 || width > 960 || height > 720 {
		return domain.Bounds{}, ErrUnavailable
	}
	d := parent.Display
	if !validBounds(d.Frame) || !validBounds(d.UsableFrame) || !validBounds(parent.Bounds) {
		return domain.Bounds{}, ErrUnavailable
	}
	left, right := math.Max(d.UsableFrame.X, d.Frame.X+32), math.Min(d.UsableFrame.X+d.UsableFrame.W, d.Frame.X+d.Frame.W-32)
	top, bottom := math.Max(d.UsableFrame.Y, d.Frame.Y+32), math.Min(d.UsableFrame.Y+d.UsableFrame.H, d.Frame.Y+d.Frame.H-32)
	edge := "bottom"
	for _, profile := range c.settings.Profiles {
		if profile.ID == parent.ProfileID {
			edge = profile.Edge
			break
		}
	}
	p := parent.Bounds
	regions := map[string]domain.Bounds{
		"bottom": {X: left, Y: top, W: right - left, H: math.Min(bottom, p.Y-8) - top},
		"top":    {X: left, Y: math.Max(top, p.Y+p.H+8), W: right - left, H: bottom - math.Max(top, p.Y+p.H+8)},
		"left":   {X: math.Max(left, p.X+p.W+8), Y: top, W: right - math.Max(left, p.X+p.W+8), H: bottom - top},
		"right":  {X: left, Y: top, W: math.Min(right, p.X-8) - left, H: bottom - top},
	}
	order := []string{edge}
	for _, candidate := range []string{"bottom", "top", "left", "right"} {
		if candidate != edge {
			order = append(order, candidate)
		}
	}
	for _, side := range order {
		r := regions[side]
		if r.W < 120 || r.H < 96 {
			continue
		}
		w, h := math.Min(width, r.W), math.Min(height, r.H)
		b := domain.Bounds{X: math.Max(r.X, math.Min(p.X+(p.W-w)/2, r.X+r.W-w)), Y: math.Max(r.Y, math.Min(p.Y+(p.H-h)/2, r.Y+r.H-h)), W: w, H: h}
		switch side {
		case "bottom":
			b.Y = r.Y + r.H - h
		case "top":
			b.Y = r.Y
		case "left":
			b.X = r.X
		case "right":
			b.X = r.X + r.W - w
		}
		if overlap(b, expand(c.env.NativeDock.Bounds, 12)) || overlap(b, p) {
			continue
		}
		return b, nil
	}
	return domain.Bounds{}, ErrUnavailable
}

func (c *Controller) SetChildBounds(parent ChildParent, session uint64, bounds domain.Bounds) error {
	c.mu.Lock()
	if session == 0 {
		c.mu.Unlock()
		return ErrRetired
	}
	placed, err := c.placeChildLocked(parent, bounds.W, bounds.H)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	if placed != bounds {
		c.mu.Unlock()
		return ErrUnavailable
	}
	if c.childHolds == nil {
		c.childHolds = map[string]childHold{}
	}
	prior := c.childHolds[parent.Scope.DisplayUUID]
	if c.childSessions[parent.Scope.DisplayUUID] >= session && prior.session != session {
		c.mu.Unlock()
		return ErrRetired
	}
	if c.childSessions == nil {
		c.childSessions = map[string]uint64{}
	}
	c.childSessions[parent.Scope.DisplayUUID] = session
	c.childHolds[parent.Scope.DisplayUUID] = childHold{parent: parent, session: session, bounds: bounds}
	c.mu.Unlock()
	c.notify()
	return nil
}

func (c *Controller) ClearChildBounds(parent ChildParent, session uint64) {
	c.mu.Lock()
	hold := c.childHolds[parent.Scope.DisplayUUID]
	if hold.session == session && hold.parent.Scope.Session == parent.Scope.Session && hold.parent.Scope.Epoch == parent.Scope.Epoch && hold.parent.Admission == parent.Admission {
		delete(c.childHolds, parent.Scope.DisplayUUID)
	}
	c.mu.Unlock()
	c.notify()
}

func (c *Controller) childOwnsPointerLocked(p Presentation) bool {
	hold, exists := c.childHolds[p.DisplayUUID]
	if !exists || p.Bounds != hold.parent.Bounds || p.ProfileID != hold.parent.ProfileID || p.Session != hold.parent.Scope.Session {
		return false
	}
	if _, err := c.validateChildParentLocked(hold.parent); err != nil {
		delete(c.childHolds, p.DisplayUUID)
		return false
	}
	x, y := c.env.PointerX, c.env.PointerY
	if contains(hold.bounds, x, y) {
		return true
	}
	// Only the short shared span connects the two rectangles, never their
	// full bounding box or the rest of the display.
	a, b := p.Bounds, hold.bounds
	left, right := math.Max(a.X, b.X), math.Min(a.X+a.W, b.X+b.W)
	top, bottom := math.Max(a.Y, b.Y), math.Min(a.Y+a.H, b.Y+b.H)
	if right > left {
		start, end := math.Min(a.Y+a.H, b.Y+b.H), math.Max(a.Y, b.Y)
		if end-start <= 24 && contains(domain.Bounds{X: left, Y: start, W: right - left, H: end - start}, x, y) {
			return true
		}
	}
	if bottom > top {
		start, end := math.Min(a.X+a.W, b.X+b.W), math.Max(a.X, b.X)
		if end-start <= 24 && contains(domain.Bounds{X: start, Y: top, W: end - start, H: bottom - top}, x, y) {
			return true
		}
	}
	return false
}
