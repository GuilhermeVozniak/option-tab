package launcher

import (
	"reflect"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func (c *Controller) reconcileLocked() {
	old := c.state
	out := State{Epoch: c.epoch, Enabled: c.settings.Enabled, Status: "ready", Displays: []DisplayState{}, Presentations: []Presentation{}}
	if c.closed {
		out.Status = "closed"
	} else if !c.settings.Enabled {
		out.Status = "disabled"
	} else if c.suspended {
		out.Status = "suspended"
	} else if !c.environmentReadyLocked() {
		out.Status = "unavailable"
	}
	if out.Status == "closed" || out.Status == "disabled" || out.Status == "suspended" {
		out.Displays = old.Displays
		for _, p := range old.Presentations {
			p.Visible = false
			p.Epoch = c.epoch
			p.Reason = out.Status
			p.Widgets = nil
			out.Presentations = append(out.Presentations, p)
		}
		c.state = out
		return
	}
	displays := c.env.Displays
	if !c.env.Complete {
		for _, d := range old.Displays {
			d.Status = "unavailable"
			d.Reason = "environmentUnavailable"
			out.Displays = append(out.Displays, d)
		}
		for _, p := range old.Presentations {
			if p.Visible || p.Reason != "environmentUnavailable" || len(p.Widgets) != 0 {
				p.Revision++
			}
			p.Visible = false
			p.Reason = "environmentUnavailable"
			p.Widgets = nil
			out.Presentations = append(out.Presentations, p)
		}
		c.state = out
		return
	}
	profiles := map[string]config.LauncherProfile{}
	for _, p := range c.settings.Profiles {
		profiles[p.ID] = p
	}
	resolved := map[string]config.LauncherBinding{}
	bound := map[string]bool{}
	for _, b := range c.settings.Bindings {
		if b.Target == "display" {
			for _, d := range displays {
				if d.UUID == b.DisplayUUID {
					resolved[d.UUID] = b
					bound[b.ID] = true
				}
			}
		}
	}
	for _, b := range c.settings.Bindings {
		if b.Target == "main" {
			for _, d := range displays {
				if d.Main {
					if _, exists := resolved[d.UUID]; !exists {
						resolved[d.UUID] = b
						bound[b.ID] = true
					} else {
						out.Displays = append(out.Displays, DisplayState{UUID: d.UUID, Name: d.Name, Main: true, BindingID: b.ID, ProfileID: b.ProfileID, SpaceKind: d.SpaceKind, Status: "conflict", Reason: "bindingConflict"})
						bound[b.ID] = true
					}
					break
				}
			}
		}
	}
	previous := map[string]Presentation{}
	for _, p := range old.Presentations {
		previous[p.DisplayUUID] = p
	}
	now := c.deps.Now()
	seen := map[string]bool{}
	for _, d := range displays {
		if d.UUID == "" || seen[d.UUID] {
			continue
		}
		seen[d.UUID] = true
		ds := DisplayState{UUID: d.UUID, Name: d.Name, Main: d.Main, SpaceKind: d.SpaceKind, Status: "unbound"}
		b, ok := resolved[d.UUID]
		if !ok {
			out.Displays = append(out.Displays, ds)
			continue
		}
		profile := profiles[b.ProfileID]
		ds.BindingID = b.ID
		ds.ProfileID = b.ProfileID
		p := Presentation{Scope: Scope{Epoch: c.epoch, DisplayUUID: d.UUID}, ProfileID: b.ProfileID, IconPx: profile.IconPx, Items: append([]Item{}, c.items...), Widgets: []Widget{}}
		prior := previous[d.UUID]
		p.Session = prior.Session
		p.Revision = prior.Revision
		reason := ""
		if !c.environmentReadyLocked() {
			reason = "environmentUnavailable"
		} else if d.SpaceKind != "ordinary" || d.SpaceStatus != "known" || d.SpaceID == 0 || d.MirrorGroup != "" {
			reason = "spaceUnavailable"
		} else if c.protectedPointer(d) {
			reason = "nativeDock"
		} else if !c.inventoryReady {
			reason = "inventoryUnavailable"
		}
		protected := []domain.Bounds{}
		if validBounds(c.env.NativeDock.Bounds) && overlap(d.Frame, c.env.NativeDock.Bounds) {
			protected = append(protected, expand(c.env.NativeDock.Bounds, 12))
		}
		geometry := Layout(profile, d, len(p.Items), protected)
		p.Bounds = geometry.Bounds
		if reason == "" && geometry.Status != "ready" {
			reason = "layoutUnavailable"
		}
		if until, failed := c.failed[d.UUID]; failed {
			if now.Before(until) {
				reason = "hostUnavailable"
			} else {
				delete(c.failed, d.UUID)
			}
		}
		p.Visible = reason == ""
		if p.Visible && profile.AutoHide {
			hold := c.hide[d.UUID]
			wasVisible := prior.Visible && prior.Epoch == c.epoch && c.spaces[d.UUID] == d.SpaceID
			if wasVisible {
				if contains(p.Bounds, c.env.PointerX, c.env.PointerY) {
					hold.leave = time.Time{}
				} else if hold.leave.IsZero() {
					hold.leave = now
				}
				p.Visible = hold.leave.IsZero() || now.Sub(hold.leave) < 500*time.Millisecond
				hold.enter = time.Time{}
			} else {
				if contains(geometry.RevealBand, c.env.PointerX, c.env.PointerY) {
					if hold.enter.IsZero() {
						hold.enter = now
					}
					p.Visible = now.Sub(hold.enter) >= 250*time.Millisecond
				} else {
					hold.enter = time.Time{}
					p.Visible = false
				}
				hold.leave = time.Time{}
			}
			c.hide[d.UUID] = hold
			if !p.Visible {
				reason = "autoHidden"
			}
		} else if !p.Visible {
			delete(c.hide, d.UUID)
		}
		p.Reason = reason
		p.Widgets = widgets(profile, p.Visible, now)
		if p.Visible && (!prior.Visible || prior.Epoch != c.epoch || c.spaces[d.UUID] != d.SpaceID) {
			c.nextSession++
			p.Session = c.nextSession
			p.Revision = 1
		} else if !reflect.DeepEqual(p, prior) {
			p.Revision++
		}
		c.spaces[d.UUID] = d.SpaceID
		if p.Visible && c.ordinaryLocked(d.UUID) && contains(p.Bounds, c.env.PointerX, c.env.PointerY) {
			out.PointerOwned = true
		}
		ds.Status = "ready"
		if !p.Visible {
			ds.Status = "yielded"
		}
		ds.Reason = reason
		out.Displays = append(out.Displays, ds)
		out.Presentations = append(out.Presentations, p)
	}
	for _, b := range c.settings.Bindings {
		if !bound[b.ID] {
			out.Displays = append(out.Displays, DisplayState{UUID: b.DisplayUUID, BindingID: b.ID, ProfileID: b.ProfileID, Status: "disconnected", Reason: "displayDisconnected"})
		}
	}
	// Missing UUIDs are removed only after the complete inventory above. Their old sessions cannot return.
	for uuid := range c.spaces {
		if !seen[uuid] {
			delete(c.spaces, uuid)
			delete(c.hide, uuid)
			delete(c.failed, uuid)
		}
	}
	c.state = out
}
