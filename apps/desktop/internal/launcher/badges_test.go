package launcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func TestBadgeAuthorityOptInAndClockContinuity(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	if _, err := c.CaptureBadges(p.Scope); err == nil {
		t.Fatal("badges admitted while disabled")
	}
	c.settings.Profiles[0].ShowBadges = true
	c.reconcileLocked()
	p = c.Snapshot().Presentations[0]
	a, err := c.CaptureBadges(p.Scope)
	if err != nil {
		t.Fatal(err)
	}
	items := a.Items()
	if len(items) != 1 || items[0].ItemID != "one" || items[0].App.Process.PID == 0 {
		t.Fatal(items)
	}
	items[0].ItemID = "changed"
	if a.Items()[0].ItemID != "one" {
		t.Fatal("badge targets alias authority")
	}
	c.state.Presentations[0].Revision++
	c.state.Presentations[0].Widgets = nil
	latest, err := c.ValidateBadges(a)
	if err != nil || latest.Revision != p.Revision+1 {
		t.Fatal(latest, err)
	}
}

func TestBadgeAuthorityRetiresPrivateReferenceAndSpaceABA(t *testing.T) {
	for _, reason := range []string{"reference", "SpaceABA", "disabled", "hidden", "target"} {
		t.Run(reason, func(t *testing.T) {
			c := prepared(t)
			c.settings.Profiles[0].ShowBadges = true
			c.settings.Profiles[0].Items = []config.LauncherItem{{ID: "a", Kind: "app", Label: "App", ReferenceID: "ref"}, {ID: "g", Kind: "group", Label: "Group", Members: []string{"a"}}, {ID: "folder", Kind: "folder", Label: "Folder", ReferenceID: "folder-ref", FolderView: "list"}}
			c.references = map[string]platform.LauncherReference{"ref": {ID: "ref", Kind: "app", BundleID: "com.example.App", State: "ready", Revision: 1}}
			c.reconcileLocked()
			p := c.Snapshot().Presentations[0]
			a, err := c.CaptureBadges(p.Scope)
			if err != nil || len(a.Items()) != 2 || a.Items()[0].Reference.ID != "ref" {
				t.Fatal(a.Items(), err, p.Visible, p.Reason)
			}
			switch reason {
			case "reference":
				r := c.references["ref"]
				r.Revision++
				c.references["ref"] = r
			case "SpaceABA":
				original := c.env
				changed := original
				changed.Displays = append([]platform.LauncherDisplay(nil), original.Displays...)
				changed.Displays[0].SpaceID++
				changed.Sequence++
				c.acceptEnvironment(c.epoch, changed)
				original.Sequence += 2
				c.acceptEnvironment(c.epoch, original)
			case "disabled":
				c.settings.Profiles[0].ShowBadges = false
			case "hidden":
				c.state.Presentations[0].Visible = false
			case "target":
				target := c.targets["one"]
				target.Process.StartSeconds++
				c.targets["one"] = target
			}
			if _, err := c.ValidateBadges(a); err == nil {
				t.Fatal("stale badges admitted", reason)
			}
		})
	}
}
