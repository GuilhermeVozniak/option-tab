package launcher

import (
	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func normalizeFocusedEvidence(e *platform.LauncherEnvironment) {
	p := e.FocusedProcess
	if !e.FocusKnown || p.PID <= 0 || p.StartSeconds <= 0 || p.StartMicros >= 1_000_000 || !config.ValidLauncherBundleID(e.FocusedBundleID) {
		e.FocusKnown = false
		e.FocusedProcess = platform.ProcessIdentity{}
		e.FocusedBundleID = ""
	}
}

func (c *Controller) profileForBindingLocked(b config.LauncherBinding) string {
	if c.env.FocusKnown {
		for _, r := range c.settings.Rules {
			if r.Enabled && r.BundleID == c.env.FocusedBundleID && (r.BindingID == "" || r.BindingID == b.ID) {
				return r.ProfileID
			}
		}
	}
	return b.ProfileID
}

func (c *Controller) profileForDisplayLocked(uuid string) string {
	for _, b := range c.settings.Bindings {
		if b.Target == "display" && b.DisplayUUID == uuid {
			return c.profileForBindingLocked(b)
		}
	}
	for _, d := range c.env.Displays {
		if d.UUID == uuid && d.Main {
			for _, b := range c.settings.Bindings {
				if b.Target == "main" {
					return c.profileForBindingLocked(b)
				}
			}
		}
	}
	return ""
}

// Retire on receipt, before coalescing can erase A→B→A. Publishing stays on Run.
func (c *Controller) retireChangedProfilesLocked() {
	for i := range c.state.Presentations {
		p := &c.state.Presentations[i]
		if p.Visible && p.ProfileID != c.profileForDisplayLocked(p.DisplayUUID) {
			p.Visible = false
			p.Revision++
			p.Reason = "profileChanged"
			p.Widgets = nil
			c.state.PointerOwned = false
			delete(c.hide, p.DisplayUUID)
			delete(c.failed, p.DisplayUUID)
		}
	}
}
