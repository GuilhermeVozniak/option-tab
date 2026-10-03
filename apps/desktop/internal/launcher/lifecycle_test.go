package launcher

import (
	"context"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func prepared(t *testing.T) *Controller {
	t.Helper()
	c := New(Deps{})
	s := config.DefaultReplacementDock()
	s.Enabled = true
	s.Profiles[0].Widgets[0].Enabled = true
	s.Profiles[0].Widgets[0].Grants = []string{"clock.read"}
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
	c.env = env()
	c.inventoryReady = true
	c.items = []Item{{ID: "one", Name: "One"}}
	c.targets["one"] = platform.LauncherAppTarget{Process: platform.ProcessIdentity{PID: 42, StartSeconds: 1}, BundleID: "fixture"}
	c.reconcileLocked()
	return c
}

func TestRetirementImmediatelyClearsWidgetSamples(t *testing.T) {
	c := prepared(t)
	c.Suspend(true)
	if len(c.Snapshot().Presentations[0].Widgets) != 0 {
		t.Fatal("retired snapshot retained widget sample")
	}
}

func TestContradictoryUUIDNeverAdmitsAction(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	e := env()
	e.Sequence = 2
	duplicate := e.Displays[0]
	duplicate.SpaceKind = "fullscreen"
	e.Displays = append(e.Displays, duplicate)
	c.acceptEnvironment(c.epoch, e)
	if c.currentLocked(p.Scope, "one") {
		t.Fatal("contradictory UUID admitted action")
	}
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Visible {
		t.Fatal("contradictory UUID visible")
	}
}

func TestIncompleteRetiresRevisionThenFreshSession(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	c.env.Complete = false
	c.reconcileLocked()
	hidden := c.Snapshot().Presentations[0]
	if hidden.Visible || hidden.Revision <= p.Revision {
		t.Fatal("incomplete inventory did not retire revision")
	}
	c.env = env()
	c.reconcileLocked()
	fresh := c.Snapshot().Presentations[0]
	if !fresh.Visible || fresh.Session == p.Session {
		t.Fatal("revived old presentation")
	}
}

func TestHiddenGrantedClockDoesNotRequestGrant(t *testing.T) {
	c := prepared(t)
	w := widgets(c.settings.Profiles[0], false, time.Now())[0]
	if w.Status == "grantRequired" || len(w.Root.Children) != 0 {
		t.Fatal("hidden clock reports missing grant or sample")
	}
}

func TestSelfTargetFinalAdmissionRefused(t *testing.T) {
	for _, bundle := range []bool{false, true} {
		c := prepared(t)
		p := c.Snapshot().Presentations[0]
		if bundle {
			c.deps.SelfBundleID = "fixture"
		} else {
			c.deps.SelfAppID = 42
		}
		if err := c.Activate(context.Background(), p.Scope, "one"); err != ErrRetired {
			t.Fatalf("self admitted: %v", err)
		}
	}
}

func TestFinalActionGuardRejectsRetirementAndKeepsBusy(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	entered := make(chan struct{})
	proceed := make(chan struct{})
	done := make(chan error, 1)
	c.deps.Activate = func(_ context.Context, _ Scope, _ platform.LauncherAppTarget, guard func() error) error {
		close(entered)
		<-proceed
		return guard()
	}
	go func() { done <- c.Activate(context.Background(), p.Scope, "one") }()
	recv(t, entered)
	if err := c.Activate(context.Background(), p.Scope, "one"); err != ErrBusy {
		t.Fatalf("parallel action: %v", err)
	}
	c.Suspend(true)
	close(proceed)
	if err := recv(t, done); err != ErrRetired {
		t.Fatalf("retired action: %v", err)
	}
}

func TestClockUpdateKeepsSessionAndAdvancesRevision(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	now := c.env.ObservedAt.Add(time.Minute)
	c.deps.Now = func() time.Time { return now }
	c.env.ObservedAt = now
	c.reconcileLocked()
	fresh := c.Snapshot().Presentations[0]
	if fresh.Session != p.Session || fresh.Revision <= p.Revision || fresh.Widgets[0].Root.Children[0].Text == p.Widgets[0].Root.Children[0].Text {
		t.Fatal("clock did not update in existing session")
	}
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Revision != fresh.Revision {
		t.Fatal("unchanged clock churned revision")
	}
}

type blockingInventory struct {
	entered, release chan struct{}
	identities       int
}

func (b *blockingInventory) Apps() ([]domain.App, error) {
	close(b.entered)
	<-b.release
	return []domain.App{{ID: 42, Name: "Fixture", BundleID: "fixture"}}, nil
}

func (b *blockingInventory) ProcessIdentity(id domain.AppID) (platform.ProcessIdentity, error) {
	b.identities++
	return platform.ProcessIdentity{PID: id, StartSeconds: 1}, nil
}

func TestRetiredBlockedAppsSkipsRemainingNativeReadsAndJoins(t *testing.T) {
	b := &blockingInventory{entered: make(chan struct{}), release: make(chan struct{})}
	c := New(Deps{Applications: b, Identities: b})
	s := config.DefaultReplacementDock()
	s.Enabled = true
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
	requests := make(chan uint64, 1)
	results := make(chan inventory, 1)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { c.inventoryWorker(ctx, requests, results); close(done) }()
	requests <- c.epoch
	recv(t, b.entered)
	c.Suspend(true)
	cancel()
	select {
	case <-done:
		t.Fatal("uncancellable Apps did not join")
	default:
	}
	close(b.release)
	recv(t, done)
	if b.identities != 0 {
		t.Fatal("retired Apps performed identity reads")
	}
}

type forbiddenPublish struct{ t *testing.T }

func (v forbiddenPublish) Publish(State) { v.t.Error("control method published inline") }
func TestControlMethodsNeverPublishInline(t *testing.T) {
	c := prepared(t)
	c.deps.View = forbiddenPublish{t}
	p := c.Snapshot().Presentations[0]
	c.FailDisplay(p.Scope, "test")
	c.Suspend(true)
	s := config.DefaultReplacementDock()
	if err := c.Configure(s); err != nil {
		t.Fatal(err)
	}
}

func TestAutoHideReentryCreatesFreshSession(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	c.settings.Profiles[0].AutoHide = true
	now := c.env.ObservedAt
	c.deps.Now = func() time.Time { return now }
	c.reconcileLocked()
	now = now.Add(501 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	if c.Snapshot().Presentations[0].Visible {
		t.Fatal("outside pointer kept auto-hide visible")
	}
	g := Layout(c.settings.Profiles[0], c.env.Displays[0], len(c.items), []domain.Bounds{expand(c.env.NativeDock.Bounds, 12)})
	c.env.PointerX = g.RevealBand.X + g.RevealBand.W/2
	c.env.PointerY = g.RevealBand.Y + g.RevealBand.H/2
	c.reconcileLocked()
	now = now.Add(251 * time.Millisecond)
	c.env.ObservedAt = now
	c.reconcileLocked()
	fresh := c.Snapshot().Presentations[0]
	if !fresh.Visible || fresh.Session == p.Session {
		t.Fatal("auto-hide revived retired session or failed reveal")
	}
}

func TestCompleteDisconnectOnlyRetiresMissingDisplay(t *testing.T) {
	c := prepared(t)
	d := c.env.Displays[0]
	d.UUID = "22222222-2222-2222-2222-222222222222"
	d.Main = false
	d.Frame.X = 1000
	d.UsableFrame.X = 1000
	c.env.Displays = append(c.env.Displays, d)
	c.settings.Bindings = append(c.settings.Bindings, config.LauncherBinding{ID: "second", Target: "display", DisplayUUID: d.UUID, ProfileID: "default"})
	c.reconcileLocked()
	before := c.Snapshot()
	if len(before.Presentations) != 2 {
		t.Fatal("second display missing")
	}
	c.env.Displays = c.env.Displays[:1]
	c.reconcileLocked()
	after := c.Snapshot()
	if len(after.Presentations) != 1 || after.Presentations[0].Session != before.Presentations[0].Session {
		t.Fatal("disconnect retired unaffected display")
	}
	c.env.Displays = append(c.env.Displays, d)
	c.reconcileLocked()
	if c.Snapshot().Presentations[1].Session == before.Presentations[1].Session {
		t.Fatal("reconnected display revived retired session")
	}
}

func TestWidgetPrimitivesRejectExecutableOrUnboundedTrees(t *testing.T) {
	for _, n := range []WidgetNode{{Kind: "script", Text: "run"}, {Kind: "text", Children: []WidgetNode{{Kind: "text"}}}, {Kind: "row", Children: make([]WidgetNode, 17)}} {
		if ValidateWidgetNode(n) == nil {
			t.Fatal("invalid primitive accepted")
		}
	}
	n := WidgetNode{Kind: "text", Text: "clock"}
	for range 5 {
		n = WidgetNode{Kind: "row", Children: []WidgetNode{n}}
	}
	if ValidateWidgetNode(n) == nil {
		t.Fatal("deep tree accepted")
	}
	c := prepared(t)
	c.settings.Profiles[0].Widgets[0].Grants = nil
	w := widgets(c.settings.Profiles[0], true, time.Now())[0]
	if w.Status != "grantRequired" || len(w.Root.Children) != 0 {
		t.Fatal("clock read without grant")
	}
}

func TestPointerOwnershipTracksEntryExitAndImmediateRetirement(t *testing.T) {
	c := prepared(t)
	p := c.Snapshot().Presentations[0]
	if c.Snapshot().PointerOwned {
		t.Fatal("outside pointer claimed")
	}
	c.env.PointerX = p.Bounds.X + p.Bounds.W/2
	c.env.PointerY = p.Bounds.Y + p.Bounds.H/2
	c.reconcileLocked()
	if !c.Snapshot().PointerOwned {
		t.Fatal("visible current panel did not own pointer")
	}
	c.env.PointerY = 100
	c.reconcileLocked()
	if c.Snapshot().PointerOwned {
		t.Fatal("outside pointer remained owned")
	}
	c.env.PointerY = p.Bounds.Y + p.Bounds.H/2
	c.reconcileLocked()
	c.Suspend(true)
	if c.Snapshot().PointerOwned {
		t.Fatal("retirement retained pointer")
	}
}
