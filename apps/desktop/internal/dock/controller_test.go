package dock

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type testObservationSource struct{ events chan platform.DockObservation }

func (s *testObservationSource) ObserveDock(ctx context.Context, emit func(platform.DockObservation)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-s.events:
			emit(event)
		}
	}
}

type dockRecordView struct {
	mu             sync.Mutex
	shows, updates []State
	hides          []uint64
	pointers       []PointerState
}

func (v *dockRecordView) Show(s State) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shows = append(v.shows, s)
}

func (v *dockRecordView) Update(s State) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.updates = append(v.updates, s)
}

func (v *dockRecordView) Hide(session uint64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.hides = append(v.hides, session)
}
func (v *dockRecordView) showCount() int   { v.mu.Lock(); defer v.mu.Unlock(); return len(v.shows) }
func (v *dockRecordView) hideCount() int   { v.mu.Lock(); defer v.mu.Unlock(); return len(v.hides) }
func (v *dockRecordView) updateCount() int { v.mu.Lock(); defer v.mu.Unlock(); return len(v.updates) }

func (v *dockRecordView) last() State {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.updates) > 0 && (len(v.shows) == 0 || v.updates[len(v.updates)-1].Session >= v.shows[len(v.shows)-1].Session) {
		return v.updates[len(v.updates)-1]
	}
	if len(v.shows) > 0 {
		return v.shows[len(v.shows)-1]
	}
	return State{}
}

type dockHarness struct {
	c          *Controller
	source     *testObservationSource
	view       *dockRecordView
	env        *fake.Fake
	settings   config.Settings
	sequence   uint64
	generation uint64
}

func newDockHarness(t *testing.T, windows platform.WindowSource, change func(*config.Settings)) *dockHarness {
	t.Helper()
	env := fake.New()
	env.ScreenList = []domain.Screen{{ID: 1, Main: true, Bounds: domain.Bounds{W: 1200, H: 800}, Visible: domain.Bounds{Y: 24, W: 1200, H: 700}}}
	env.ActiveAppID = 10
	if windows == nil {
		env.SetWindows([]domain.Window{{ID: 101, AppID: 10, AppName: "Example", BundleID: "example.app", Title: "First", OnScreen: true, SpaceID: 1, ScreenID: 1}})
		windows = env
	}
	s := config.Default()
	s.Dock.Enabled = true
	if change != nil {
		change(&s)
	}
	source := &testObservationSource{events: make(chan platform.DockObservation)}
	v := &dockRecordView{}
	c := NewController(Deps{Observations: source, Windows: windows, Env: env, View: v, SelfBundleID: "self.app"}, s)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go c.Run(ctx)
	synctest.Wait()
	return &dockHarness{c: c, source: source, view: v, env: env, settings: s, generation: 1}
}

func dockTestItem(id domain.AppID) *platform.DockItem {
	return &platform.DockItem{AppID: id, BundleID: "example.app", Path: "/Applications/Example.app", Title: "Example", Bounds: domain.Bounds{X: 400, Y: 740, W: 48, H: 48}, ScreenID: 1, Edge: "bottom"}
}

func (h *dockHarness) observe(item *platform.DockItem, x, y float64) {
	h.sequence++
	h.source.events <- platform.DockObservation{Sequence: h.sequence, Generation: h.generation, PointerX: x, PointerY: y, Item: item, Status: "ready"}
	synctest.Wait()
}

func TestDockControllerHoverDelayAndPointerContinuity(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, nil)
		h.observe(dockTestItem(10), 424, 750)
		time.Sleep(299 * time.Millisecond)
		synctest.Wait()
		if h.view.showCount() != 0 {
			t.Fatal("opened before hover delay")
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if h.view.showCount() != 1 || h.view.last().SelectedWindowID != 101 {
			t.Fatalf("missing exact selected window: %+v", h.view.last())
		}
		st := h.view.last()
		// Cross the short gap, then enter the actual bounded panel.
		h.observe(nil, 424, 735)
		time.Sleep(300 * time.Millisecond)
		synctest.Wait()
		h.observe(nil, st.Bounds.X+10, st.Bounds.Y+10)
		time.Sleep(300 * time.Millisecond)
		synctest.Wait()
		if h.view.hideCount() != 0 {
			t.Fatal("closed crossing corridor/panel")
		}
		h.observe(nil, 1100, 100)
		time.Sleep(249 * time.Millisecond)
		synctest.Wait()
		if h.view.hideCount() != 0 {
			t.Fatal("closed before dismissal delay")
		}
		time.Sleep(51 * time.Millisecond)
		synctest.Wait()
		if h.view.hideCount() != 1 {
			t.Fatal("did not dismiss after escape")
		}
	})
}

func TestDockControllerOpeningJitterDoesNotExtendShownHover(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.DismissDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		time.Sleep(150 * time.Millisecond)
		h.observe(nil, 451, 760) // three points outside the icon, within opening slop
		time.Sleep(150 * time.Millisecond)
		synctest.Wait()
		if h.view.showCount() != 1 {
			t.Fatal("opening jitter restarted hover")
		}
		h.observe(nil, 451, 780) // slop must no longer keep a shown panel open
		if h.view.hideCount() != 1 {
			t.Fatal("opening slop incorrectly retained shown panel")
		}
	})
}

type blockingWindowSource struct {
	requests chan chan []domain.Window
}

func (s *blockingWindowSource) Windows() ([]domain.Window, error) {
	response := make(chan []domain.Window)
	s.requests <- response
	return <-response, nil
}

func TestDockControllerDropsOldGenerationAndCoalescesQueries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &blockingWindowSource{requests: make(chan chan []domain.Window, 8)}
		h := newDockHarness(t, source, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		old := <-source.requests
		h.generation = 2
		h.observe(dockTestItem(20), 424, 750)
		if len(source.requests) != 0 {
			t.Fatal("overlapping window queries")
		}
		old <- []domain.Window{{ID: 101, AppID: 10, Title: "Stale"}}
		synctest.Wait()
		if h.view.showCount() != 0 {
			t.Fatal("stale generation shown")
		}
		current := <-source.requests
		current <- []domain.Window{{ID: 201, AppID: 20, Title: "Current"}}
		synctest.Wait()
		st := h.view.last()
		if h.view.showCount() != 1 || st.SelectedWindowID != 201 {
			t.Fatalf("current result lost: %+v", st)
		}
		h.c.Refresh(st.Session)
		synctest.Wait()
		refresh := <-source.requests
		for range 10 {
			h.c.Refresh(st.Session)
		}
		synctest.Wait()
		if len(source.requests) != 0 {
			t.Fatal("refreshes were not coalesced")
		}
		h.c.Suspend(true)
		synctest.Wait()
		refresh <- []domain.Window{{ID: 202, AppID: 20, Title: "Late"}}
		synctest.Wait()
		if h.view.updateCount() != 0 || h.view.hideCount() != 1 || len(source.requests) != 0 {
			t.Fatal("suspended session accepted query")
		}
	})
}

func TestDockControllerFilteringExactSelectionAndStaleCommands(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.env.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "First", SpaceID: 1}, {ID: 102, AppID: 10, Title: "Second", SpaceID: 2}, {ID: 999, AppID: 99, Title: "Other"}})
		h.observe(dockTestItem(10), 424, 750)
		st := h.view.last()
		if len(st.Windows) != 2 {
			t.Fatalf("wrong PID windows: %+v", st.Windows)
		}
		h.c.SelectWindow(st.Session, 102)
		synctest.Wait()
		if h.view.last().SelectedWindowID != 102 {
			t.Fatal("exact selection failed")
		}
		h.c.SelectWindow(st.Session, 999)
		synctest.Wait()
		if h.view.last().SelectedWindowID != 102 {
			t.Fatal("accepted another app target")
		}
		h.settings.Dock.Scope.Spaces = config.SpacesActive
		h.env.ActiveSpaceID = 3
		h.c.Configure(h.settings)
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		fresh := h.view.last()
		if fresh.Session == st.Session || fresh.EmptyReason != "filtered" || len(fresh.Windows) != 0 {
			t.Fatalf("filter update: %+v", fresh)
		}
		h.c.Dismiss(st.Session)
		h.c.SelectWindow(st.Session, 101)
		synctest.Wait()
		if h.view.hideCount() != 1 || h.view.last().SelectedWindowID != 0 {
			t.Fatal("old webview command affected new session")
		}
		h.settings.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "example.app", Hide: config.HideAlways}}
		h.c.Configure(h.settings)
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		if h.view.showCount() != 2 {
			t.Fatal("blacklisted app produced a panel")
		}
	})
}

func TestDockControllerMovesResizedPanelAndRejectsMissingDisplay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.env.ScreenList = []domain.Screen{{ID: 1, Bounds: domain.Bounds{X: -1200, W: 1200, H: 800}, Visible: domain.Bounds{X: -1200, Y: 24, W: 1200, H: 700}}}
		item := dockTestItem(10)
		item.Bounds.X = -800
		h.observe(item, -780, 750)
		st := h.view.last()
		h.c.SetPanelBounds(st.Session, domain.Bounds{W: 300, H: 160})
		synctest.Wait()
		item.Bounds.X = -500
		h.observe(item, -480, 750)
		moved := h.view.last()
		if moved.Session != st.Session || moved.Bounds.W != 300 || moved.Bounds.H != 160 || moved.Bounds.X <= st.Bounds.X {
			t.Fatalf("bad reposition: before=%+v after=%+v", st.Bounds, moved.Bounds)
		}
		h.env.ScreenList = nil
		h.c.Refresh(st.Session)
		synctest.Wait()
		if h.view.hideCount() != 1 {
			t.Fatal("display removal left orphan panel")
		}
	})
}

func TestDockPointerCorridorIsNarrow(t *testing.T) {
	icon := domain.Bounds{X: 100, Y: 700, W: 50, H: 50}
	panel := domain.Bounds{X: 0, Y: 400, W: 500, H: 250}
	for _, point := range []struct {
		x, y float64
		want bool
	}{{125, 675, true}, {480, 675, false}, {100, 500, true}, {500, 750, false}, {99, 730, false}} {
		if got := overDockSurface(point.x, point.y, icon, panel, "bottom", 12); got != point.want {
			t.Errorf("(%v,%v)=%v, want %v", point.x, point.y, got, point.want)
		}
	}
}

type dockAppInventory struct {
	apps     []domain.App
	presence platform.WindowPresence
}

func (s dockAppInventory) Apps() ([]domain.App, error) { return s.apps, nil }

func (s dockAppInventory) AppWindowPresence(domain.AppID) platform.WindowPresence { return s.presence }

func TestDockQueryUsesNativePresenceAndHiddenInventory(t *testing.T) {
	env := fake.New()
	env.ScreenList = []domain.Screen{{ID: 1, Bounds: domain.Bounds{W: 1200, H: 800}}}
	env.SetWindows([]domain.Window{{ID: 301, AppID: 10, BundleID: "example.app"}})
	settings := config.Default()
	settings.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "example.app", Hide: config.HideWhenNoWindow}}
	item := Item{Kind: "app", AppID: 10, BundleID: "example.app", Title: "Example", ScreenID: 1}
	for _, test := range []struct {
		presence platform.WindowPresence
		excluded bool
		reason   string
	}{{platform.WindowsNone, true, "noWindows"}, {platform.WindowsUnknown, false, "unavailable"}, {platform.WindowsPresent, false, "filtered"}} {
		inventory := dockAppInventory{apps: []domain.App{{ID: 10, Name: "Example", BundleID: "example.app"}}, presence: test.presence}
		got := queryWindows(Deps{Windows: env, Env: env, Apps: inventory, AppWindows: inventory}, settings, item)
		if got.err != nil || got.excluded != test.excluded || got.emptyReason != test.reason {
			t.Errorf("presence %s result=%+v", test.presence, got)
		}
	}
	hidden := dockAppInventory{apps: []domain.App{{ID: 10, Name: "Example", BundleID: "example.app", Hidden: true}}, presence: platform.WindowsUnknown}
	settings.Filters.ShowHiddenApps = config.VisHide
	if got := queryWindows(Deps{Windows: env, Env: env, Apps: hidden, AppWindows: hidden}, settings, item); !got.excluded {
		t.Fatal("hidden windowless app bypassed global filters")
	}
	if got := queryWindows(Deps{Windows: env, Env: env, Apps: dockAppInventory{}}, config.Default(), item); got.emptyReason != "notRunning" {
		t.Fatal("terminated app left actionable window state")
	}
}

func TestDockControllerDismissAndPermissionInvalidation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		h.c.Dismiss(h.view.last().Session)
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		if h.view.showCount() != 1 {
			t.Fatal("dismissed icon immediately reopened")
		}
		h.observe(nil, 1100, 100)
		h.observe(dockTestItem(10), 424, 750)
		if h.view.showCount() != 2 {
			t.Fatal("fresh hover did not reopen")
		}
		// A delayed permission observation cannot invalidate a newer sample.
		h.source.events <- platform.DockObservation{Sequence: 1, Generation: 1, Status: "permissionDenied"}
		synctest.Wait()
		if h.view.hideCount() != 1 {
			t.Fatal("stale observation hid current panel")
		}
		h.sequence++
		h.source.events <- platform.DockObservation{Sequence: h.sequence, Generation: 2, Status: "permissionDenied"}
		synctest.Wait()
		if h.view.hideCount() != 2 {
			t.Fatal("permission loss left panel open")
		}
	})
}

func TestDockControllerEmptyStatesAndDisabledPendingQuery(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.env.SetWindows(nil)
		h.observe(dockTestItem(10), 424, 750)
		if h.view.last().EmptyReason != "noWindows" || h.view.last().SelectedWindowID != 0 {
			t.Fatal("invented a window")
		}
		h.observe(dockTestItem(0), 424, 750)
		if h.view.last().EmptyReason != "notRunning" {
			t.Fatal("pinned app status missing")
		}
		h.settings.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "example.app", Hide: config.HideWhenNoWindow}}
		h.c.Configure(h.settings)
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		if h.view.showCount() != 2 {
			t.Fatal("windowless blacklist ignored")
		}
	})
	synctest.Test(t, func(t *testing.T) {
		source := &blockingWindowSource{requests: make(chan chan []domain.Window, 8)}
		h := newDockHarness(t, source, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		pending := <-source.requests
		h.settings.Dock.Enabled = false
		h.c.Configure(h.settings)
		synctest.Wait()
		pending <- []domain.Window{{ID: 101, AppID: 10, Title: "Late"}}
		synctest.Wait()
		if h.view.showCount() != 0 || len(source.requests) != 0 {
			t.Fatal("disabled controller admitted query result")
		}
	})
}

type retiringObservationSource struct {
	started   chan struct{}
	cancelled chan struct{}
	release   chan struct{}
}

func (s *retiringObservationSource) ObserveDock(ctx context.Context, _ func(platform.DockObservation)) error {
	s.started <- struct{}{}
	<-ctx.Done()
	s.cancelled <- struct{}{}
	<-s.release
	return ctx.Err()
}

func TestDockControllerSerializesObserverRetirement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &retiringObservationSource{make(chan struct{}, 16), make(chan struct{}, 16), make(chan struct{})}
		settings := config.Default()
		settings.Dock.Enabled = true
		c := NewController(Deps{Observations: s}, settings)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.Run(ctx)
		synctest.Wait()
		<-s.started
		for range 5 {
			c.Configure(settings)
		}
		synctest.Wait()
		if len(c.commands) != 0 {
			t.Error("owner stopped processing commands during native retirement")
		}
		if len(s.started) != 0 {
			t.Errorf("started %d observers before old native cleanup", len(s.started))
		}
		c.Suspend(true)
		synctest.Wait()
		close(s.release)
		synctest.Wait()
		// Pending starts were coalesced and the latest suspended state wins.
		if len(s.started) != 0 {
			t.Error("observer started despite latest suspension")
		}
		c.Suspend(false)
		synctest.Wait()
		if len(s.started) != 1 {
			t.Errorf("resume starts=%d, want one", len(s.started))
		}
		cancel()
		synctest.Wait()
		select {
		case <-c.done:
		default:
			t.Error("controller did not finish native cleanup")
		}
	})
}

func TestDockControllerScreenMoveRejectsInFlightOldGeometry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &blockingWindowSource{requests: make(chan chan []domain.Window, 8)}
		h := newDockHarness(t, source, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.env.ScreenList = append(h.env.ScreenList, domain.Screen{ID: 2, Bounds: domain.Bounds{X: 1200, W: 1000, H: 800}, Visible: domain.Bounds{X: 1200, Y: 24, W: 1000, H: 700}})
		item := dockTestItem(10)
		h.observe(item, 424, 750)
		initial := <-source.requests
		initial <- []domain.Window{{ID: 101, AppID: 10, Title: "Window"}}
		synctest.Wait()
		before := h.view.last()
		h.c.Refresh(before.Session)
		synctest.Wait()
		old := <-source.requests
		item.ScreenID = 2
		item.Bounds.X = 1500
		h.observe(item, 1524, 750)
		if h.view.updateCount() != 0 {
			t.Error("published moved icon against stale display bounds")
		}
		if h.view.hideCount() != 1 {
			t.Error("old-screen panel remained visible while resolving new display")
		}
		old <- []domain.Window{{ID: 101, AppID: 10, Title: "Old query"}}
		synctest.Wait()
		if h.view.showCount() != 1 || h.view.updateCount() != 0 {
			t.Error("old-screen query was published after migration")
		}
		current := <-source.requests
		current <- []domain.Window{{ID: 101, AppID: 10, Title: "New query"}}
		synctest.Wait()
		state := h.view.last()
		if state.Session == before.Session || state.Item.ScreenID != 2 || state.Bounds.X < 1200 || state.Bounds.X+state.Bounds.W > 2200 {
			t.Errorf("new display geometry/session invalid: %+v", state)
		}
	})
}

func TestDockControllerShutdownWaitsForObserverCleanup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := &retiringObservationSource{make(chan struct{}, 1), make(chan struct{}, 1), make(chan struct{})}
		settings := config.Default()
		settings.Dock.Enabled = true
		c := NewController(Deps{Observations: s}, settings)
		ctx, cancel := context.WithCancel(context.Background())
		go c.Run(ctx)
		synctest.Wait()
		cancel()
		synctest.Wait()
		select {
		case <-c.done:
			t.Error("Run returned before native observer cleanup")
		default:
		}
		close(s.release)
		synctest.Wait()
		select {
		case <-c.done:
		default:
			t.Error("Run did not return after native observer cleanup")
		}
	})
}

type callbackDockView struct{ show func(State) }

func (v callbackDockView) Show(s State)   { v.show(s) }
func (v callbackDockView) Update(s State) { v.show(s) }
func (v callbackDockView) Hide(uint64)    {}

func TestDockCommandsDoNotBlockWhileViewWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewController(Deps{}, config.Default())
		viewBlocked, releaseView := make(chan struct{}), make(chan struct{})
		c.deps.View = callbackDockView{show: func(State) { close(viewBlocked); <-releaseView }}
		l := &controllerLoop{controller: c, admission: c.AdmissionEpoch(), state: State{Session: 1}}
		viewDone := make(chan struct{})
		go func() { l.publish(true); close(viewDone) }()
		<-viewBlocked
		sent := make(chan struct{})
		go func() {
			for range 1000 {
				c.Refresh(1)
			}
			close(sent)
		}()
		synctest.Wait()
		select {
		case <-sent:
		default:
			t.Error("command producer blocked behind View")
		}
		close(c.done) // Release the old blocking implementation on RED.
		close(releaseView)
		<-viewDone
		<-sent
	})
}

func TestDockViewCanReenterWithDismissFlood(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewController(Deps{}, config.Default())
		c.deps.View = callbackDockView{show: func(s State) {
			for range 1000 {
				c.Dismiss(s.Session)
			}
		}}
		l := &controllerLoop{controller: c, admission: c.AdmissionEpoch(), state: State{Session: 1}}
		returned := make(chan struct{})
		go func() { l.publish(true); close(returned) }()
		synctest.Wait()
		select {
		case <-returned:
		default:
			t.Error("View callback deadlocked reentering controller")
		}
		close(c.done)
		<-returned
	})
}

func TestDockMailboxCoalescesByKindWithLatestOrderAndSession(t *testing.T) {
	c := NewController(Deps{}, config.Default())
	for range 1000 {
		c.Refresh(4)
		c.Dismiss(5)
		c.Suspend(true)
		c.SetPanelBounds(6, domain.Bounds{W: 300, H: 200})
		c.SelectWindow(7, 101)
		c.Configure(config.Default())
	}
	c.Dismiss(3) // A stale session must neither replace nor reorder session 5.
	c.Refresh(8) // Latest update moves this kind to the end.
	batch := c.takeCommands()
	want := []string{"dismiss", "suspend", "bounds", "select", "configure", "refresh"}
	if len(batch) != len(want) {
		t.Fatalf("mailbox size=%d", len(batch))
	}
	for i, kind := range want {
		if batch[i].kind != kind {
			t.Errorf("position %d=%s, want %s", i, batch[i].kind, kind)
		}
	}
	if batch[0].session != 5 || batch[5].session != 8 {
		t.Errorf("stale session replaced newer commands: %+v", batch)
	}
	c.finish()
	c.Dismiss(9)
	if len(c.takeCommands()) != 0 {
		t.Error("terminal mailbox admitted command")
	}
}

func TestDockCoalescedSuspendResumeStillRetiresPresentation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		entered, release := make(chan struct{}), make(chan struct{})
		first := true
		h.c.deps.View = callbackDockView{show: func(state State) {
			h.view.Show(state)
			if first {
				first = false
				close(entered)
				<-release
			}
		}}
		h.observe(dockTestItem(10), 424, 750)
		<-entered
		old := h.view.last()
		// Owner is inside View and cannot drain the mailbox. The latest state is
		// false, but the intervening suspension must durably invalidate old work.
		h.c.Suspend(true)
		h.c.Suspend(false)
		close(release)
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		fresh := h.view.last()
		if fresh.Session <= old.Session || h.view.showCount() != 2 {
			t.Fatalf("coalesced suspension retained old presentation: old=%d new=%d shows=%d", old.Session, fresh.Session, h.view.showCount())
		}
	})
}

func TestDockAdmissionInvalidatesBeforeOwnerDrainsMailbox(t *testing.T) {
	c := NewController(Deps{}, config.Default())
	initial := c.AdmissionEpoch()
	if initial == 0 {
		t.Fatal("initial real observation could bypass admission gate")
	}
	c.Suspend(true)
	c.Suspend(false)
	afterSuspend := c.AdmissionEpoch()
	if afterSuspend <= initial {
		t.Fatal("coalesced resume lost synchronous invalidation")
	}
	c.Configure(config.Default())
	if c.AdmissionEpoch() <= afterSuspend {
		t.Fatal("configuration did not invalidate pending delivery")
	}
}

func TestDockPublishedStateCarriesCurrentAdmissionEpoch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		old := h.view.last()
		if old.AdmissionEpoch == 0 || old.AdmissionEpoch != h.c.AdmissionEpoch() {
			t.Fatal("real state missing admission epoch")
		}
		h.c.Suspend(true)
		h.c.Suspend(false)
		if old.AdmissionEpoch == h.c.AdmissionEpoch() {
			t.Fatal("already-created state still has current epoch")
		}
		synctest.Wait()
		h.observe(dockTestItem(10), 424, 750)
		if h.view.last().AdmissionEpoch != h.c.AdmissionEpoch() {
			t.Fatal("resumed state not current")
		}
	})
}

func (v *dockRecordView) Pointer(p PointerState) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.pointers = append(v.pointers, p)
}

func (v *dockRecordView) pointerSamples() []PointerState {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]PointerState(nil), v.pointers...)
}

func TestDockPointerTransport(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := newDockHarness(t, nil, func(s *config.Settings) { s.Dock.HoverDelayMs = 0 })
		h.observe(dockTestItem(10), 424, 750)
		samples := h.view.pointerSamples()
		if len(samples) != 1 {
			t.Fatalf("show must publish initial pointer: %+v", samples)
		}
		st := h.view.last()
		if p := samples[0]; p.Session != st.Session || p.AdmissionEpoch != st.AdmissionEpoch || p.Sequence != 1 || p.Inside || p.X != 424-st.Bounds.X {
			t.Fatalf("bad initial pointer %+v", p)
		}
		h.observe(nil, st.Bounds.X+10, st.Bounds.Y+20)
		samples = h.view.pointerSamples()
		if p := samples[len(samples)-1]; p.Sequence != 2 || !p.Inside || p.X != 10 || p.Y != 20 {
			t.Fatalf("bad panel point %+v", p)
		}
		h.observe(nil, st.Bounds.X+10, st.Bounds.Y+20)
		if len(h.view.pointerSamples()) != 2 {
			t.Fatal("duplicate pointer emitted")
		}
		h.c.SetPanelBounds(st.Session, domain.Bounds{W: 500, H: 300})
		synctest.Wait()
		moved := h.view.last()
		samples = h.view.pointerSamples()
		if p := samples[len(samples)-1]; p.Sequence != 3 || p.X != st.Bounds.X+10-moved.Bounds.X || p.Y != st.Bounds.Y+20-moved.Bounds.Y {
			t.Fatalf("resize did not reproject pointer %+v", p)
		}
		h.observe(nil, 424, 735)
		samples = h.view.pointerSamples()
		if p := samples[len(samples)-1]; p.Inside {
			t.Fatalf("corridor treated as panel %+v", p)
		}
		h.c.Dismiss(st.Session)
		synctest.Wait()
		count := len(h.view.pointerSamples())
		h.observe(nil, 10, 10)
		if len(h.view.pointerSamples()) != count {
			t.Fatal("pointer after hide")
		}
		h.observe(dockTestItem(10), 424, 750)
		samples = h.view.pointerSamples()
		fresh := samples[len(samples)-1]
		if fresh.Session == st.Session || fresh.Sequence != 1 {
			t.Fatalf("new presentation not reset %+v", fresh)
		}
		h.c.SetPanelBounds(st.Session, domain.Bounds{W: 600, H: 400})
		synctest.Wait()
		if len(h.view.pointerSamples()) != len(samples) {
			t.Fatal("stale session emitted pointer")
		}
		h.c.Suspend(true)
		synctest.Wait()
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()
		if len(h.view.pointerSamples()) != len(samples) {
			t.Fatal("suspended pointer emitted")
		}
	})
}

func TestDockPointerRejectsSynchronousAdmissionInvalidation(t *testing.T) {
	s := config.Default()
	s.Dock.Enabled = true
	v := &dockRecordView{}
	c := NewController(Deps{View: v}, s)
	l := controllerLoop{controller: c, settings: s, admission: 1, shown: true, session: 7, state: State{Session: 7, AdmissionEpoch: 1, Bounds: domain.Bounds{W: 100, H: 100}}, last: &platform.DockObservation{PointerX: 20, PointerY: 20}}
	c.Suspend(true)
	l.publishPointer()
	if len(v.pointerSamples()) != 0 {
		t.Fatal("invalidated admission emitted pointer before suspension drained")
	}
}

func TestDockPointerGeometryOnlyChangeAndStaleObservation(t *testing.T) {
	s := config.Default()
	s.Dock.Enabled = true
	v := &dockRecordView{}
	c := NewController(Deps{View: v}, s)
	l := controllerLoop{controller: c, settings: s, admission: 1, epoch: 5, sequence: 9, shown: true, session: 7, state: State{Session: 7, AdmissionEpoch: 1, Bounds: domain.Bounds{X: 10, Y: 10, W: 100, H: 100}}, last: &platform.DockObservation{PointerX: 20, PointerY: 20}}
	l.publishPointer()
	l.state.Bounds.W = 200
	l.publish(false)
	if got := v.pointerSamples(); len(got) != 2 || got[1].Sequence != 2 || got[1].X != 10 {
		t.Fatalf("geometry-only update missing: %+v", got)
	}
	l.observe(observation{epoch: 4, value: platform.DockObservation{Sequence: 10, Status: "ready", PointerX: 50, PointerY: 50}})
	l.observe(observation{epoch: 5, value: platform.DockObservation{Sequence: 8, Status: "ready", PointerX: 50, PointerY: 50}})
	if len(v.pointerSamples()) != 2 {
		t.Fatal("stale native observation published")
	}
}
