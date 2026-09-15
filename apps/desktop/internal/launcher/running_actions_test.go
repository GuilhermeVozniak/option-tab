package launcher

import (
	"context"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/platform"
)

func TestRunningRelaunchCapturedLifetime(t *testing.T) {
	for _, change := range []string{"unchanged", "content", "exit", "replaced", "newInstance", "hidden", "cancelled", "reconfigured"} {
		t.Run(change, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := New(Deps{})
			settings := config.DefaultReplacementDock()
			settings.Enabled = true
			if err := c.Configure(settings); err != nil {
				t.Fatal(err)
			}
			c.env, c.inventoryReady = env(), true
			target := platform.LauncherAppTarget{Process: platform.ProcessIdentity{PID: 42, StartSeconds: 1}, BundleID: "owned", Name: "Owned"}
			c.targets = map[string]platform.LauncherAppTarget{"running": target, "other": {Process: platform.ProcessIdentity{PID: 9, StartSeconds: 1}, BundleID: "other"}}
			c.items = []Item{{ID: "running", Name: "Owned"}, {ID: "other", Name: "Other"}}
			c.reconcileLocked()
			p := c.Snapshot().Presentations[0]
			dispatched := false
			c.deps.Relaunch = func(_ context.Context, scope Scope, captured platform.LauncherAppTarget, guard func() error) error {
				want := target
				want.DisplayUUID = p.DisplayUUID
				if scope != p.Scope || captured != want {
					t.Fatalf("wrong captured target: %+v", captured)
				}
				if err := c.Relaunch(ctx, p.Scope, "running"); err != ErrBusy {
					t.Fatalf("overlapping relaunch admitted: %v", err)
				}
				switch change {
				case "content":
					c.state.Presentations[0].Revision++
				case "exit":
					delete(c.targets, "running")
					c.items = c.items[1:]
					c.reconcileLocked()
				case "replaced":
					next := target
					next.Process.StartSeconds++
					c.targets["running"] = next
				case "newInstance":
					delete(c.targets, "running")
					next := target
					next.Process.PID++
					c.targets["new"] = next
				case "hidden":
					c.state.Presentations[0].Visible = false
				case "cancelled":
					cancel()
				case "reconfigured":
					settings.Profiles[0].Name = "Changed profile"
					if err := c.Configure(settings); err != nil {
						t.Fatal(err)
					}
				}
				if err := guard(); err != nil {
					return err
				}
				dispatched = true
				return nil
			}
			err := c.Relaunch(ctx, p.Scope, "running")
			allowed := change == "unchanged" || change == "content" || change == "exit"
			if allowed && (err != nil || !dispatched) {
				t.Fatalf("captured relaunch refused: %v", err)
			}
			if !allowed && (err == nil || dispatched) {
				t.Fatalf("retired relaunch dispatched: %v", err)
			}
			if c.actionBusy {
				t.Fatal("completed relaunch retained command gate")
			}
		})
	}
}
