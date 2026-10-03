package launcher

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func childController(t *testing.T) (*Controller, ChildParent) {
	t.Helper()
	c := configuredController(t, Deps{})
	c.settings.Profiles[0].Items[0].Kind = "folder"
	c.settings.Profiles[0].Items[0].FolderView = "list"
	r := c.references["ref"]
	r.Kind = "folder"
	c.references["ref"] = r
	c.reconcileLocked()
	p := c.Snapshot().Presentations[0]
	parent, err := c.CaptureChildParent(p.Scope, "pin:file")
	if err != nil {
		t.Fatal(err)
	}
	return c, parent
}

func TestChildAuthoritySurvivesContentOnlyRevision(t *testing.T) {
	c, parent := childController(t)
	c.state.Presentations[0].Revision++
	latest, err := c.ValidateChildParent(parent)
	if err != nil || latest.Revision != parent.Scope.Revision+1 {
		t.Fatal("content tick retired child", latest, err)
	}
	if _, err := c.CaptureChildParent(parent.Scope, parent.ItemID); !errors.Is(err, ErrRetired) {
		t.Fatal("stale initial pixels admitted", err)
	}
}

func TestChildAuthorityRejectsChangedParentOrResource(t *testing.T) {
	for _, mode := range []string{"reference", "item", "process", "hidden", "session", "bounds", "profile", "inventory", "closed", "suspended"} {
		t.Run(mode, func(t *testing.T) {
			c, p := childController(t)
			switch mode {
			case "reference":
				r := c.references["ref"]
				r.Revision++
				c.references["ref"] = r
			case "process":
				r := c.references["ref"]
				r.Process = platform.ProcessIdentity{PID: 42, StartSeconds: 7}
				c.references["ref"] = r
			case "item":
				c.settings.Profiles[0].Items[0].ReferenceID = "other"
			case "hidden":
				c.state.Presentations[0].Visible = false
			case "session":
				c.state.Presentations[0].Session++
			case "bounds":
				c.state.Presentations[0].Bounds.X++
			case "profile":
				c.state.Presentations[0].ProfileID = "different"
			case "inventory":
				c.inventoryReady = false
			case "closed":
				c.closed = true
			case "suspended":
				c.suspended = true
			}
			if _, err := c.ValidateChildParent(p); !errors.Is(err, ErrRetired) {
				t.Fatal("changed authority admitted before publication", err)
			}
		})
	}
}

func TestChildEnvironmentABACannotReviveAdmission(t *testing.T) {
	for _, mode := range []string{"space", "topology", "scale", "mirror", "incomplete", "recovery", "dock"} {
		t.Run(mode, func(t *testing.T) {
			c, parent := childController(t)
			original := c.env
			changed := original
			changed.Displays = append([]platform.LauncherDisplay(nil), original.Displays...)
			changed.Sequence++
			switch mode {
			case "space":
				changed.Displays[0].SpaceID++
			case "topology":
				changed.Displays[0].Frame.X++
			case "scale":
				changed.Displays[0].Scale++
			case "mirror":
				changed.Displays[0].MirrorGroup = "mirror"
			case "incomplete":
				changed.Complete = false
			case "recovery":
				changed.PointerY = 790
			case "dock":
				changed.NativeDock.Bounds = domain.Bounds{X: 400, Y: 740, W: 200, H: 20}
				changed.PointerX, changed.PointerY = 500, 755
			}
			c.acceptEnvironment(c.epoch, changed)
			original.Sequence += 2
			c.acceptEnvironment(c.epoch, original)
			if _, err := c.ValidateChildParent(parent); !errors.Is(err, ErrRetired) {
				t.Fatal("A to B to A revived child before reconcile", err)
			}
		})
	}
}

func TestChildUnrelatedDisplayChangeDoesNotRetireParent(t *testing.T) {
	c, parent := childController(t)
	e := c.env
	e.Sequence++
	other := e.Displays[0]
	other.UUID, other.Main = "other", false
	other.Frame.X, other.UsableFrame.X = 1000, 1000
	e.Displays = append(append([]platform.LauncherDisplay(nil), e.Displays...), other)
	c.acceptEnvironment(c.epoch, e)
	if _, err := c.ValidateChildParent(parent); err != nil {
		t.Fatal("another display retired current parent", err)
	}
}

func TestChildCaptureOnlyRenderedRunningAppsAndFolderPins(t *testing.T) {
	c, _ := childController(t)
	c.targets["running"] = platform.LauncherAppTarget{Process: platform.ProcessIdentity{PID: 42, StartSeconds: 1}, BundleID: "test.app", Name: "App"}
	c.items = []Item{{ID: "running", Name: "App"}}
	c.reconcileLocked()
	p := c.state.Presentations[0]
	parent, err := c.CaptureChildParent(p.Scope, "running")
	if err != nil || parent.App.Process.PID != 42 || parent.Configured.ID != "" {
		t.Fatal("running app missing", parent, err)
	}
	c.items = nil
	c.reconcileLocked()
	if _, err := c.CaptureChildParent(c.state.Presentations[0].Scope, "running"); err == nil {
		t.Fatal("unrendered target admitted")
	}
	c.settings.Profiles[0].Items = []config.LauncherItem{{ID: "app", Kind: "app", Label: "App", ReferenceID: "ref"}, {ID: "group", Kind: "group", Label: "Group", Members: []string{"app"}}}
	r := c.references["ref"]
	r.Kind, r.BundleID = "app", "test.app"
	c.references["ref"] = r
	c.reconcileLocked()
	if _, err := c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:app"); err == nil {
		t.Fatal("stopped app received window child")
	}
	r.Process = platform.ProcessIdentity{PID: 42, StartSeconds: 1}
	c.references["ref"] = r
	c.reconcileLocked()
	parent, err = c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:app")
	if err != nil || parent.App.Process != r.Process {
		t.Fatal("exact group member app missing", err)
	}
	if _, err := c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:group"); err == nil {
		t.Fatal("group received child")
	}
}

func TestChildPlacementAllEdgesAndNegativeOrigins(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		t.Run(edge, func(t *testing.T) {
			c, _ := childController(t)
			c.settings.Profiles[0].Edge = edge
			d := &c.env.Displays[0]
			d.Frame.X, d.UsableFrame.X = -1000, -1000
			c.reconcileLocked()
			parent, err := c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:file")
			if err != nil {
				t.Fatal(err)
			}
			b, err := c.PlaceChild(parent, 420, 360)
			if err != nil || b.W != 420 || b.H != 360 || overlap(b, parent.Bounds) || b.X < -968 || b.Y < 32 || b.X+b.W > -32 || b.Y+b.H > 768 {
				t.Fatal("invalid child placement", b, err)
			}
			switch edge {
			case "bottom":
				if b.Y+b.H > parent.Bounds.Y {
					t.Fatal("not inward")
				}
			case "top":
				if b.Y < parent.Bounds.Y+parent.Bounds.H {
					t.Fatal("not inward")
				}
			case "left":
				if b.X < parent.Bounds.X+parent.Bounds.W {
					t.Fatal("not inward")
				}
			case "right":
				if b.X+b.W > parent.Bounds.X {
					t.Fatal("not inward")
				}
			}
		})
	}
}

func TestChildPlacementRefusesInvalidAndProtectsDock(t *testing.T) {
	c, p := childController(t)
	for _, size := range [][2]float64{{0, 1}, {-1, 30}, {math.NaN(), 100}, {100, math.Inf(1)}, {961, 720}, {960, 721}} {
		if _, err := c.PlaceChild(p, size[0], size[1]); err == nil {
			t.Fatal("invalid size admitted", size)
		}
	}
	c.env.NativeDock.Bounds = domain.Bounds{X: 300, Y: 300, W: 400, H: 300}
	b, err := c.PlaceChild(p, 420, 360)
	if err == nil && overlap(b, expand(c.env.NativeDock.Bounds, 12)) {
		t.Fatal("native Dock covered", b)
	}
	if err := c.SetChildBounds(p, 1, domain.Bounds{X: 1, Y: 1, W: 1000, H: 800}); err == nil {
		t.Fatal("unplaced renderer bounds accepted")
	}
}

func TestChildHoldKeepsAutoHideAndOldClearCannotReleaseSuccessor(t *testing.T) {
	c, p := childController(t)
	now := c.env.ObservedAt
	c.deps.Now = func() time.Time { return now }
	c.settings.Profiles[0].AutoHide = true
	b, err := c.PlaceChild(p, 420, 360)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetChildBounds(p, 10, b); err != nil {
		t.Fatal(err)
	}
	if err := c.SetChildBounds(p, 11, b); err != nil {
		t.Fatal(err)
	}
	c.ClearChildBounds(p, 10)
	c.env.PointerX, c.env.PointerY = b.X+b.W/2, b.Y+b.H/2
	c.reconcileLocked()
	now = now.Add(time.Second)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if !c.state.Presentations[0].Visible || !c.state.PointerOwned {
		t.Fatal("child pointer lost parent")
	}
	c.ClearChildBounds(p, 11)
	c.reconcileLocked()
	now = now.Add(501 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if c.state.Presentations[0].Visible || c.state.PointerOwned {
		t.Fatal("closed child kept parent awake")
	}
}

func TestChildHoldCannotSuppressRecoveryYield(t *testing.T) {
	c, p := childController(t)
	b, err := c.PlaceChild(p, 420, 360)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetChildBounds(p, 1, b); err != nil {
		t.Fatal(err)
	}
	c.env.PointerY = 790
	c.reconcileLocked()
	if c.state.Presentations[0].Visible || c.state.PointerOwned {
		t.Fatal("child suppressed native recovery")
	}
}

func TestChildClosedSessionCannotRestoreHold(t *testing.T) {
	c, p := childController(t)
	b, err := c.PlaceChild(p, 420, 360)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetChildBounds(p, 10, b); err != nil {
		t.Fatal(err)
	}
	c.ClearChildBounds(p, 10)
	for _, session := range []uint64{9, 10} {
		if err := c.SetChildBounds(p, session, b); !errors.Is(err, ErrRetired) {
			t.Fatal("retired child hold revived", session, err)
		}
	}
	if err := c.SetChildBounds(p, 11, b); err != nil {
		t.Fatal("successor refused", err)
	}
}

func TestChildDisplayAdmissionsAreBoundedWithoutReconnectionRevival(t *testing.T) {
	c, parent := childController(t)
	original := c.env
	for i := 0; i < 40; i++ {
		e := original
		e.Displays = append([]platform.LauncherDisplay(nil), original.Displays...)
		e.Sequence = uint64(i*2 + 2)
		e.Displays[0].UUID = "temporary"
		c.acceptEnvironment(c.epoch, e)
		c.reconcileLocked()
		if _, err := c.CaptureChildParent(c.state.Presentations[0].Scope, parent.ItemID); err != nil {
			t.Fatal(err)
		}
		e = original
		e.Sequence = uint64(i*2 + 3)
		c.acceptEnvironment(c.epoch, e)
		c.reconcileLocked()
		if _, err := c.ValidateChildParent(parent); !errors.Is(err, ErrRetired) {
			t.Fatal("removed admission revived", err)
		}
		if _, err := c.CaptureChildParent(c.state.Presentations[0].Scope, parent.ItemID); err != nil {
			t.Fatal(err)
		}
		if len(c.childAdmissions) > 1 {
			t.Fatal("disconnected displays retained", len(c.childAdmissions))
		}
	}
}

func TestChildRetirementPublishesEvenWhenParentPixelsReturnUnchanged(t *testing.T) {
	c, parent := childController(t)
	c.reconcileLocked()
	before := c.Snapshot()
	e := c.env
	e.Displays = append([]platform.LauncherDisplay(nil), e.Displays...)
	e.Sequence++
	e.Displays[0].SpaceID++
	c.acceptEnvironment(c.epoch, e)
	e.Sequence++
	e.Displays[0].SpaceID--
	c.acceptEnvironment(c.epoch, e)
	c.reconcileLocked()
	after := c.Snapshot()
	if reflect.DeepEqual(before, after) {
		t.Fatal("child retirement lost to parent-pixel coalescing")
	}
	if after.Presentations[0].Scope != parent.Scope {
		t.Fatal("child retirement changed parent pixel scope")
	}
}

func TestChildConnectingGapOwnsOnlySharedSpan(t *testing.T) {
	c, p := childController(t)
	b, err := c.PlaceChild(p, 420, 360)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetChildBounds(p, 1, b); err != nil {
		t.Fatal(err)
	}
	c.env.PointerX = p.Bounds.X + p.Bounds.W/2
	c.env.PointerY = (b.Y + b.H + p.Bounds.Y) / 2
	c.reconcileLocked()
	if !c.state.PointerOwned {
		t.Fatal("pointer crossing short gap lost ownership")
	}
	c.env.PointerX = b.X + 1
	c.reconcileLocked()
	if c.state.PointerOwned {
		t.Fatal("unconnected space beside parent claimed")
	}
}

func TestChildShrinksOnNarrowDisplayAndRefusesNoUsefulRegion(t *testing.T) {
	c, _ := childController(t)
	c.env.Displays[0].Frame = domain.Bounds{W: 240, H: 280}
	c.env.Displays[0].UsableFrame = domain.Bounds{Y: 25, W: 240, H: 255}
	c.reconcileLocked()
	p, err := c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:file")
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.PlaceChild(p, 420, 360)
	if err != nil || b.W < 120 || b.H < 96 || b.W > 176 || b.H > 216 {
		t.Fatal("narrow display failed to fit", b, err)
	}
	c.env.Displays[0].Frame.H = 190
	c.env.Displays[0].UsableFrame.H = 165
	c.reconcileLocked()
	p, err = c.CaptureChildParent(c.state.Presentations[0].Scope, "pin:file")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.PlaceChild(p, 420, 360); err == nil {
		t.Fatal("unusable child region admitted")
	}
}
