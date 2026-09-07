package launcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func TestInteractionAuthorityClockContinuity(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	a, err := c.CaptureInteraction(p.Scope)
	if err != nil {
		t.Fatal(err)
	}
	c.state.Presentations[0].Revision++
	c.state.Presentations[0].Widgets = nil
	latest, err := c.ValidateInteraction(a)
	if err != nil || latest.Revision != p.Revision+1 {
		t.Fatal(latest, err)
	}
	if _, err = c.CaptureInteraction(p.Scope); err == nil {
		t.Fatal("stale initial scope")
	}
	items := a.NavigationItems()
	if len(items) != 1 || items[0].ID != "one" {
		t.Fatal(items)
	}
	items[0].ID = "mutated"
	if a.NavigationItems()[0].ID != "one" {
		t.Fatal("aliased navigation")
	}
}

func TestInteractionAuthorityRetirement(t *testing.T) {
	for _, mode := range []string{"hidden", "bounds", "target", "settings", "SpaceABA"} {
		t.Run(mode, func(t *testing.T) {
			c := prepared(t)
			p := c.Snapshot().Presentations[0]
			a, err := c.CaptureInteraction(p.Scope)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "hidden":
				c.state.Presentations[0].Visible = false
			case "bounds":
				c.state.Presentations[0].Bounds.X++
			case "target":
				target := c.targets["one"]
				target.Process.StartSeconds++
				c.targets["one"] = target
			case "settings":
				c.settings.Profiles[0].Name = "changed"
			case "SpaceABA":
				original := c.env
				changed := original
				changed.Displays = append([]platform.LauncherDisplay(nil), original.Displays...)
				changed.Displays[0].SpaceID++
				changed.Sequence++
				c.acceptEnvironment(c.epoch, changed)
				original.Sequence += 2
				c.acceptEnvironment(c.epoch, original)
			}
			if _, err = c.ValidateInteraction(a); err == nil {
				t.Fatal("retired authority admitted")
			}
		})
	}
}

func TestInteractionAuthorityReferencesAndGroupNavigation(t *testing.T) {
	c := configuredController(t, Deps{})
	c.settings.Profiles[0].Items = []config.LauncherItem{{ID: "a", Kind: "app", Label: "A", ReferenceID: "ref"}, {ID: "g", Kind: "group", Label: "Group", Members: []string{"a"}}, {ID: "link", Kind: "link", Label: "Link", URL: "https://example.com"}, {ID: "gap", Kind: "separator"}, {ID: "missing", Kind: "file", Label: "Missing", ReferenceID: "other"}}
	c.references["ref"] = platform.LauncherReference{ID: "ref", Kind: "app", State: "ready", Revision: 1}
	c.reconcileLocked()
	p := c.Snapshot().Presentations[0]
	a, err := c.CaptureInteraction(p.Scope)
	if err != nil {
		t.Fatal(err)
	}
	nav := a.NavigationItems()
	if len(nav) != 2 || nav[0].ID != "pin:a" || nav[1].ID != "pin:link" {
		t.Fatal(nav)
	}
	r := c.references["ref"]
	r.Revision++
	c.references["ref"] = r
	if _, err = c.ValidateInteraction(a); err == nil {
		t.Fatal("changed reference before publication")
	}
	if _, err = c.CaptureInteraction(p.Scope); err == nil {
		t.Fatal("captured obsolete rendered reference")
	}
}
