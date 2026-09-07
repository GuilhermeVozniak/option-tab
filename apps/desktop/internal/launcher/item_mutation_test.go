package launcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func mutationController(t *testing.T) (*Controller, Presentation) {
	c := configuredController(t, Deps{})
	c.settings.Profiles[0].RuntimeReorder = true
	c.settings.Profiles[0].Items = append(c.settings.Profiles[0].Items, config.LauncherItem{ID: "gap", Kind: "separator"})
	c.references = nil // Structural edits do not require a ready selected resource.
	c.reconcileLocked()
	return c, c.Snapshot().Presentations[0]
}

func TestItemMutationAdmission(t *testing.T) {
	c, p := mutationController(t)
	a, err := c.CaptureItemMutation(p.Scope, p.ItemsRevision, "pin:file", "pin:gap")
	if err != nil || a.ItemID != "file" || a.TargetID != "gap" || !p.RuntimeReorder {
		t.Fatal(a, err)
	}
	if err = c.ValidateItemMutation(a); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"pin:gap", "pin:file"}, {"running", "pin:gap"}, {"pin:file", "pin:missing"}, {"pin:file", "pin:file"}} {
		if _, err = c.CaptureItemMutation(p.Scope, p.ItemsRevision, pair[0], pair[1]); err == nil {
			t.Fatal(pair)
		}
	}
	if _, err = c.CaptureItemMutation(p.Scope, "wrong", "pin:file", "pin:gap"); err == nil {
		t.Fatal("wrong hash admitted")
	}
}

func TestItemMutationRetires(t *testing.T) {
	for _, mode := range []string{"clock", "off", "hide", "hash", "suspend", "spaceABA"} {
		t.Run(mode, func(t *testing.T) {
			c, p := mutationController(t)
			a, err := c.CaptureItemMutation(p.Scope, p.ItemsRevision, "pin:file", "pin:gap")
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "clock":
				c.state.Presentations[0].Revision++
			case "off":
				c.settings.Profiles[0].RuntimeReorder = false
			case "hide":
				c.state.Presentations[0].Visible = false
			case "hash":
				c.settings.Profiles[0].Items[0].Label = "changed"
			case "suspend":
				c.suspended = true
			case "spaceABA":
				original := c.env
				changed := original
				changed.Displays = append([]platform.LauncherDisplay(nil), original.Displays...)
				changed.Sequence++
				changed.Displays[0].SpaceID++
				c.acceptEnvironment(c.epoch, changed)
				original.Sequence += 2
				c.acceptEnvironment(c.epoch, original)
			}
			if err = c.ValidateItemMutation(a); err == nil {
				t.Fatal("stale authority accepted")
			}
		})
	}
}

func TestItemMutationConfiguredGroupMembers(t *testing.T) {
	c, _ := mutationController(t)
	c.settings.Profiles[0].Items = []config.LauncherItem{
		{ID: "a", Kind: "app", Label: "A", ReferenceID: "missing-a"},
		{ID: "b", Kind: "app", Label: "B", ReferenceID: "missing-b"},
		{ID: "g", Kind: "group", Label: "Group", Members: []string{"a", "b"}},
	}
	c.reconcileLocked()
	p := c.Snapshot().Presentations[0]
	a, err := c.CaptureItemMutation(p.Scope, p.ItemsRevision, "pin:a", "pin:b")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.ValidateItemMutation(a); err != nil {
		t.Fatal(err)
	}
	c.settings.Profiles[0].RuntimeReorder = false
	c.reconcileLocked()
	p = c.Snapshot().Presentations[0]
	if p.RuntimeReorder {
		t.Fatal("opt out missing")
	}
	if _, err = c.CaptureItemMutation(p.Scope, p.ItemsRevision, "pin:a", "pin:b"); err == nil {
		t.Fatal("disabled capture")
	}
}
