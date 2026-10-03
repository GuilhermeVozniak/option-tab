package launcher

import (
	"errors"
	"testing"
	"time"
)

func TestAutoHideHoldKeepsOwnedInteractionAndReleasesAfterCancel(t *testing.T) {
	c := prepared(t)
	c.settings.Profiles[0].AutoHide = true
	now := c.env.ObservedAt
	c.deps.Now = func() time.Time { return now }
	p := c.Snapshot().Presentations[0]
	if err := c.SetAutoHideHold(p.Scope, 1, "begin"); err != nil {
		t.Fatal(err)
	}
	c.reconcileLocked()
	now = now.Add(600 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if !c.Snapshot().Presentations[0].Visible {
		t.Fatal("owned interaction auto-hid outside panel")
	}
	p = c.Snapshot().Presentations[0]
	if err := c.SetAutoHideHold(p.Scope, 2, "end"); err != nil {
		t.Fatal(err)
	}
	c.reconcileLocked()
	now = now.Add(501 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Visible {
		t.Fatal("cancelled interaction kept panel visible")
	}
}

func TestAutoHideHoldExpiresWithoutHeartbeatAndCannotBeReplayed(t *testing.T) {
	c := prepared(t)
	c.settings.Profiles[0].AutoHide = true
	now := c.env.ObservedAt
	c.deps.Now = func() time.Time { return now }
	p := c.Snapshot().Presentations[0]
	if err := c.SetAutoHideHold(p.Scope, 1, "begin"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAutoHideHold(p.Scope, 3, "end"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetAutoHideHold(p.Scope, 2, "begin"); !errors.Is(err, ErrRetired) {
		t.Fatal("late begin revived released hold", err)
	}
	if err := c.SetAutoHideHold(p.Scope, 4, "begin"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2100 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	now = now.Add(501 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Visible {
		t.Fatal("stale frontend hold never expired")
	}
	if err := c.SetAutoHideHold(p.Scope, 5, "begin"); !errors.Is(err, ErrRetired) {
		t.Fatal("hidden session revived", err)
	}
}

func TestAutoHideHoldSurvivesClockButYieldsToRecoveryAndRetirement(t *testing.T) {
	for _, change := range []string{"nativeDock", "suspend", "space", "profile", "items"} {
		t.Run(change, func(t *testing.T) {
			c := prepared(t)
			c.settings.Profiles[0].AutoHide = true
			p := c.Snapshot().Presentations[0]
			if err := c.SetAutoHideHold(p.Scope, 1, "begin"); err != nil {
				t.Fatal(err)
			}
			c.state.Presentations[0].Revision++
			p = c.Snapshot().Presentations[0]
			if err := c.SetAutoHideHold(p.Scope, 2, "renew"); err != nil {
				t.Fatal("clock revision retired hold", err)
			}
			switch change {
			case "nativeDock":
				c.env.PointerY = 790
			case "suspend":
				c.Suspend(true)
			case "space":
				c.env.Displays[0].SpaceID++
			case "profile":
				c.settings.Profiles[0].IconPx++
			case "items":
				c.items[0].Name = "Changed"
			}
			if err := c.SetAutoHideHold(p.Scope, 3, "renew"); !errors.Is(err, ErrRetired) {
				t.Fatal("changed owner retained hold", err)
			}
			c.reconcileLocked()
			if change == "nativeDock" || change == "suspend" {
				if c.Snapshot().Presentations[0].Visible {
					t.Fatal("hold blocked recovery")
				}
			}
		})
	}
}

func TestAutoHideHoldFreshInteractionAfterItemsChange(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	if err := c.SetAutoHideHold(p.Scope, 1, "begin"); err != nil {
		t.Fatal(err)
	}
	c.items[0].Name = "Updated application"
	c.reconcileLocked()
	latest := c.Snapshot().Presentations[0]
	if latest.Session != p.Session || !latest.Visible {
		t.Fatal("fixture lost visible session")
	}
	if err := c.SetAutoHideHold(latest.Scope, 2, "renew"); !errors.Is(err, ErrRetired) {
		t.Fatal("stale renewal adopted changed items", err)
	}
	if err := c.SetAutoHideHold(latest.Scope, 3, "begin"); err != nil {
		t.Fatal("fresh click refused after new items rendered", err)
	}
}
