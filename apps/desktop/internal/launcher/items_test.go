package launcher

import (
	"context"
	"errors"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func configuredController(t *testing.T, deps Deps) *Controller {
	t.Helper()
	c := New(deps)
	s := config.DefaultReplacementDock()
	s.Enabled = true
	s.Profiles[0].Items = []config.LauncherItem{{ID: "file", Kind: "file", Label: "File", ReferenceID: "ref"}}
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
	c.env = env()
	c.inventoryReady = true
	c.references = map[string]platform.LauncherReference{"ref": {ID: "ref", Kind: "file", State: "ready", Revision: 1}}
	c.reconcileLocked()
	return c
}

func TestConfiguredReferenceChangeRetiresPresentationAndAction(t *testing.T) {
	c := configuredController(t, Deps{})
	p := c.Snapshot().Presentations[0]
	if !p.Visible || len(p.Items) != 1 || p.Items[0].ID != "pin:file" {
		t.Fatalf("pin not presented: %+v", p)
	}
	if _, err := c.ConfiguredItem(p.Scope, "pin:file"); err != nil {
		t.Fatal(err)
	}
	r := c.references["ref"]
	r.Revision++
	c.references["ref"] = r
	if _, err := c.ConfiguredItem(p.Scope, "pin:file"); err == nil {
		t.Fatal("changed reference admitted before publication")
	}
	c.reconcileLocked()
	next := c.Snapshot().Presentations[0]
	if next.Revision <= p.Revision {
		t.Fatal("authority change reused presentation revision")
	}
	if _, err := c.ConfiguredItem(p.Scope, "pin:file"); !errors.Is(err, ErrRetired) {
		t.Fatal("old scope admitted", err)
	}
}

func TestConfiguredProcessReplacementRetiresPublishedScope(t *testing.T) {
	c := configuredController(t, Deps{})
	p := c.Snapshot().Presentations[0]
	r := c.references["ref"]
	r.Process = platform.ProcessIdentity{PID: 42, StartSeconds: 8}
	c.references["ref"] = r
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Revision == p.Revision {
		t.Fatal("process replacement reused scope")
	}
}

func TestConfiguredActivationRetiresBeforeNativeGuard(t *testing.T) {
	var c *Controller
	dispatched := false
	c = configuredController(t, Deps{PerformItem: func(ctx context.Context, scope Scope, item ConfiguredTarget, action string, guard func() error) error {
		if item.Reference.Revision != 1 || item.Item.ID != "file" || action != "open" {
			t.Fatal("wrong target")
		}
		c.Suspend(true)
		if err := guard(); err != nil {
			return err
		}
		dispatched = true
		return nil
	}})
	p := c.Snapshot().Presentations[0]
	if err := c.Activate(context.Background(), p.Scope, "pin:file"); !errors.Is(err, ErrRetired) || dispatched {
		t.Fatal("retired action dispatched", err)
	}
}

func TestConfiguredInventoryCancellationDropsOldAuthority(t *testing.T) {
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	c := configuredController(t, Deps{ResolveReference: func(ctx context.Context, id string) (platform.LauncherReference, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return platform.LauncherReference{ID: id, Kind: "file", State: "ready", Revision: 99}, nil
	}})
	out := inventory{epoch: c.epoch}
	done := make(chan struct{})
	go func() { defer close(done); c.readConfiguredInventory(context.Background(), &out) }()
	recv(t, started)
	c.Suspend(true)
	c.Suspend(false)
	recv(t, canceled)
	close(release)
	recv(t, done)
	if len(out.references) != 0 {
		t.Fatal("canceled result retained authority")
	}
}

func TestConfiguredItemsMergeOnlyTheExactSelectedRunningApp(t *testing.T) {
	c := New(Deps{})
	process := platform.ProcessIdentity{PID: 42, StartSeconds: 5}
	c.items = []Item{{ID: "running", Name: "Editor", Icon: "icon"}}
	c.targets["running"] = platform.LauncherAppTarget{Process: process, BundleID: "test.editor"}
	c.references = map[string]platform.LauncherReference{"ref": {ID: "ref", Kind: "app", Label: "Editor", State: "ready", Revision: 1, BundleID: "test.editor", Process: process}}
	p := config.LauncherProfile{Items: []config.LauncherItem{{ID: "editor", Kind: "app", Label: "My editor", ReferenceID: "ref"}}}
	items := c.composeItemsLocked(p)
	if len(items) != 1 || items[0].ID != "pin:editor" || !items[0].Running || items[0].Icon != "icon" {
		t.Fatalf("exact selected app not merged: %+v", items)
	}
	r := c.references["ref"]
	r.Process = platform.ProcessIdentity{PID: 43, StartSeconds: 5}
	c.references["ref"] = r
	items = c.composeItemsLocked(p)
	if len(items) != 2 || items[0].Running {
		t.Fatalf("bundle-only match hid another installation: %+v", items)
	}
}

func TestConfiguredGroupsKeepMissingMembersAndCopySnapshots(t *testing.T) {
	c := New(Deps{})
	p := config.LauncherProfile{Items: []config.LauncherItem{
		{ID: "one", Kind: "app", Label: "Missing app", ReferenceID: "missing"},
		{ID: "tools", Kind: "group", Label: "Tools", Members: []string{"one"}},
		{ID: "gap", Kind: "separator"},
		{ID: "site", Kind: "link", Label: "Website", URL: "https://example.com"},
	}}
	items := c.composeItemsLocked(p)
	if len(items) != 3 || items[0].Kind != "group" || len(items[0].Members) != 1 || items[0].Members[0].Status != "needsSelection" || items[1].Kind != "separator" || items[2].Status != "ready" {
		t.Fatalf("invalid group presentation: %+v", items)
	}
	snapshot := cloneState(State{Presentations: []Presentation{{Items: items}}})
	snapshot.Presentations[0].Items[0].Members[0].Name = "changed"
	if items[0].Members[0].Name == "changed" {
		t.Fatal("group members alias a published snapshot")
	}
}

func TestConfiguredReferenceMismatchAndSelfAppCannotBecomeReady(t *testing.T) {
	c := New(Deps{SelfBundleID: "test.self"})
	c.references = map[string]platform.LauncherReference{
		"file": {ID: "file", Kind: "folder", State: "ready", Revision: 1},
		"self": {ID: "self", Kind: "app", BundleID: "test.self", State: "ready", Revision: 1},
	}
	p := config.LauncherProfile{Items: []config.LauncherItem{
		{ID: "file", Kind: "file", Label: "File", ReferenceID: "file"},
		{ID: "self", Kind: "app", Label: "Option Tab", ReferenceID: "self"},
	}}
	items := c.composeItemsLocked(p)
	if items[0].Status != "changed" || items[1].Status != "excluded" {
		t.Fatalf("mismatched reference or self admitted: %+v", items)
	}
}

func TestConfiguredAdmittedActionSurvivesContentTickAndExpectedRelaunchExit(t *testing.T) {
	for _, mode := range []string{"openTick", "relaunchExit", "relaunchExitUnpublished", "referenceChange", "newProcess", "openExit", "hidden", "profile"} {
		t.Run(mode, func(t *testing.T) {
			var c *Controller
			c = configuredController(t, Deps{PerformItem: func(_ context.Context, _ Scope, target ConfiguredTarget, _ string, guard func() error) error {
				c.mu.Lock()
				p := &c.state.Presentations[0]
				p.Revision++
				switch mode {
				case "relaunchExit", "relaunchExitUnpublished", "openExit":
					r := c.references["ref"]
					r.Process = platform.ProcessIdentity{}
					c.references["ref"] = r
					if mode != "relaunchExitUnpublished" {
						p.Items[0].reference = r
					}
				case "referenceChange":
					r := c.references["ref"]
					r.Revision++
					c.references["ref"] = r
					p.Items[0].reference = r
				case "newProcess":
					r := c.references["ref"]
					r.Process.PID++
					c.references["ref"] = r
					p.Items[0].reference = r
				case "hidden":
					p.Visible = false
				case "profile":
					p.ProfileID = "other"
				}
				c.mu.Unlock()
				return guard()
			}})
			c.mu.Lock()
			c.settings.Profiles[0].Items[0].Kind = "app"
			r := c.references["ref"]
			r.Kind = "app"
			r.Process = platform.ProcessIdentity{PID: 42, StartSeconds: 5}
			c.references["ref"] = r
			c.reconcileLocked()
			c.mu.Unlock()
			p := c.Snapshot().Presentations[0]
			action := "relaunch"
			if mode == "openTick" || mode == "openExit" {
				action = "open"
			}
			err := c.PerformConfigured(context.Background(), p.Scope, "pin:file", action)
			if mode == "openTick" || mode == "relaunchExit" || mode == "relaunchExitUnpublished" {
				if err != nil {
					t.Fatal("benign admitted transition refused", err)
				}
			} else if !errors.Is(err, ErrRetired) {
				t.Fatal("changed authority accepted", err)
			}
			if _, err = c.ConfiguredItem(p.Scope, "pin:file"); err == nil {
				t.Fatal("old initial RPC revision accepted")
			}
		})
	}
}
