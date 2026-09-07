package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"time"

	"option-tab/internal/automation"
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
	"option-tab/internal/preview"
	"option-tab/internal/switcher"
)

func automationPreviewFixture(t *testing.T) (*App, *appAutomationPlatform) {
	t.Helper()
	p := &appAutomationPlatform{fake.New()}
	p.ActiveAppID = 1
	p.ScreenList = []domain.Screen{{ID: 1, Main: true, Visible: domain.Bounds{X: -1440, Y: -100, W: 1440, H: 900}}}
	p.SetWindows([]domain.Window{{ID: 1, AppID: 1, Title: "Fixture", BundleID: "test.fixture", AppName: "Fixture", SpaceID: 1, ScreenID: 1, OnScreen: true}})
	settings := config.Default()
	settings.Appearance.Style = config.StyleTitles
	settings.Appearance.PreviewSelected = false
	a := newApp(p, settings, "")
	a.wireAutomation(nil)
	a.automation.previewFactory = func(uint64, func(platform.MediaPanelEvent)) *dockWindow { d, _, _, _ := dockHarness(); return d }
	t.Cleanup(a.stopCapture)
	return a, p
}

func showAutomationFixture(t *testing.T, a *App, p *appAutomationPlatform) uint64 {
	t.Helper()
	windows, _ := p.Windows()
	result, err := a.showAutomationPreviews(context.Background(), platform.ProcessIdentity{PID: 1, StartSeconds: 123}, windows, &platform.AutomationPoint{X: -5000, Y: -5000}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	id, err := strconv.ParseUint(result.Token, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAutomationPreviewIndependentTokenHideAndClamping(t *testing.T) {
	a, p := automationPreviewFixture(t)
	first := showAutomationFixture(t, a, p)
	bounds := a.automation.preview.bounds
	if bounds.X != -1440 || bounds.Y != -100 {
		t.Fatalf("negative display clamp %+v", bounds)
	}
	second := showAutomationFixture(t, a, p)
	if _, err := a.hideAutomationPreviews(context.Background(), strconv.FormatUint(first, 10), func() error { return nil }); err == nil {
		t.Fatal("old token hid replacement")
	}
	if a.GetAutomationPreviewState(second) == nil {
		t.Fatal("replacement missing")
	}
}

func TestAutomationPreviewFactoryUnavailableAndLateGuard(t *testing.T) {
	a, p := automationPreviewFixture(t)
	a.automation.previewFactory = nil
	windows, _ := p.Windows()
	if _, err := a.showAutomationPreviews(context.Background(), platform.ProcessIdentity{PID: 1, StartSeconds: 123}, windows, nil, func() error { return nil }); err == nil {
		t.Fatal("unsupported factory admitted")
	}
}

func TestAutomationPreviewRPCRequiresRenderedRevision(t *testing.T) {
	a, p := automationPreviewFixture(t)
	session := showAutomationFixture(t, a, p)
	if err := a.SelectAutomationPreview(session, 0, 1); err == nil {
		t.Fatal("zero revision admitted selection")
	}
	if err := a.SetAutomationPreviewSize(session, 0, 500, 400); err == nil {
		t.Fatal("zero revision admitted geometry")
	}
	if err := a.CloseAutomationPreview(session, 0); err == nil {
		t.Fatal("zero revision admitted close")
	}
	if a.GetAutomationPreviewState(session) == nil {
		t.Fatal("invalid RPC retired owner")
	}
}

func TestAutomationPreviewGuardAfterFactoryAndBeforeCommit(t *testing.T) {
	a, p := automationPreviewFixture(t)
	original := showAutomationFixture(t, a, p)
	ctx, cancel := context.WithCancel(context.Background())
	a.automation.previewFactory = func(uint64, func(platform.MediaPanelEvent)) *dockWindow {
		cancel()
		d, _, _, _ := dockHarness()
		return d
	}
	windows, _ := p.Windows()
	if _, err := a.showAutomationPreviews(ctx, platform.ProcessIdentity{PID: 1, StartSeconds: 123}, windows, nil, func() error { return nil }); err == nil {
		t.Fatal("cancelled preparation replaced owner")
	}
	if a.GetAutomationPreviewState(original) == nil {
		t.Fatal("failed show retired previous owner")
	}
}

type previewActionPlatform struct {
	*appAutomationPlatform
	prepare    func()
	dispatched int
}

func (p *previewActionPlatform) PerformAutomationWindowAction(ctx context.Context, kind string, id platform.AutomationWindowIdentity, fullscreen *bool, guard func() error) error {
	if p.prepare != nil {
		p.prepare()
	}
	if err := guard(); err != nil {
		return err
	}
	p.dispatched++
	return nil
}

func TestAutomationPreviewActionRechecksAfterNativePreparation(t *testing.T) {
	a, p := automationPreviewFixture(t)
	native := &previewActionPlatform{appAutomationPlatform: p}
	a.platform = native
	session := showAutomationFixture(t, a, p)
	state := a.GetAutomationPreviewState(session)
	native.prepare = func() { showAutomationFixture(t, a, p) }
	if err := a.PerformAutomationPreviewAction(session, state.Revision, "close", 1, false); err == nil {
		t.Fatal("obsolete prepared close accepted")
	}
	if native.dispatched != 0 {
		t.Fatal("obsolete prepared close dispatched")
	}
	if a.automation.preview.state.Session == session {
		t.Fatal("replacement missing")
	}
}

func TestAutomationPreviewFramesRejectWrongIdentityAndRetirement(t *testing.T) {
	a, p := automationPreviewFixture(t)
	session := showAutomationFixture(t, a, p)
	a.viewMu.Lock()
	owner := a.automation.preview
	owner.capture = true
	a.publishAutomationPreviewLocked(owner)
	a.viewMu.Unlock()
	var frames []automationPreviewFrames
	a.eventSink = func(name string, data any) {
		if name == "automation-preview:frames" {
			frames = append(frames, data.(automationPreviewFrames))
		}
	}
	wrong := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 1, StartSeconds: 999}}
	a.emitAutomationPreviewFrame(1, "wrong", wrong)
	if len(frames) != 0 {
		t.Fatal("replacement process frame admitted")
	}
	exact, _ := p.WindowIdentity(1)
	a.emitAutomationPreviewFrame(1, "exact", exact)
	if len(frames) != 1 || frames[0].Session != session || frames[0].Frames[1] != "exact" {
		t.Fatalf("exact frame missing: %+v", frames)
	}
	revision := a.automation.preview.state.Revision
	if err := a.CloseAutomationPreview(session, revision); err != nil {
		t.Fatal(err)
	}
	a.emitAutomationPreviewFrame(1, "late", exact)
	if len(frames) != 1 {
		t.Fatal("late retired frame emitted")
	}
}

func TestAutomationPreviewSettingsInvalidationAndOldHostCallback(t *testing.T) {
	a, p := automationPreviewFixture(t)
	first := showAutomationFixture(t, a, p)
	old := a.automation.preview.host
	second := showAutomationFixture(t, a, p)
	a.automationPreviewHostClosed(first, old, nil)
	if a.GetAutomationPreviewState(second) == nil {
		t.Fatal("old host callback retired replacement")
	}
	a.settingsMu.Lock()
	a.settings.Behavior.Paused = true
	a.settingsMu.Unlock()
	a.viewMu.Lock()
	a.syncAutomationPreviewLocked()
	a.viewMu.Unlock()
	if a.automation.preview != nil {
		t.Fatal("paused owner retained")
	}
}

type automationCaptureFixture struct{ started, stopped chan domain.WindowID }

func (f *automationCaptureFixture) StreamWindow(ctx context.Context, id domain.WindowID, _ int, _ func(string)) error {
	f.started <- id
	<-ctx.Done()
	f.stopped <- id
	return ctx.Err()
}

func TestAutomationPreviewHideKeepsOtherCaptureOwner(t *testing.T) {
	a, p := automationPreviewFixture(t)
	a.captures.Close()
	a.automation.peer.Close()
	source := &automationCaptureFixture{started: make(chan domain.WindowID, 4), stopped: make(chan domain.WindowID, 4)}
	a.captures = preview.New(source, func(domain.WindowID, string) {})
	a.automation.peer = a.captures.NewPeerWithIdentity(a.emitAutomationPreviewFrame)
	a.captures.Update([]domain.WindowID{9}, 9, 100)
	session := showAutomationFixture(t, a, p)
	a.viewMu.Lock()
	owner := a.automation.preview
	owner.capture = true
	a.publishAutomationPreviewLocked(owner)
	a.updateAutomationPreviewCaptureLocked(owner)
	revision := owner.state.Revision
	a.viewMu.Unlock()
	for range 2 {
		select {
		case <-source.started:
		case <-time.After(time.Second):
			t.Fatal("both capture owners did not start")
		}
	}
	if err := a.CloseAutomationPreview(session, revision); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-source.stopped:
		if id != 1 {
			t.Fatalf("closed unrelated capture %d", id)
		}
	case <-time.After(time.Second):
		t.Fatal("preview capture did not stop")
	}
	select {
	case id := <-source.stopped:
		t.Fatalf("other owner unexpectedly stopped %d", id)
	case <-time.After(20 * time.Millisecond):
	}
	a.captures.Hide()
	select {
	case id := <-source.stopped:
		if id != 9 {
			t.Fatal(id)
		}
	case <-time.After(time.Second):
		t.Fatal("other owner cleanup missing")
	}
}

func TestAutomationPreviewFrameSnapshotIsBoundedAndCopied(t *testing.T) {
	a, p := automationPreviewFixture(t)
	session := showAutomationFixture(t, a, p)
	a.viewMu.Lock()
	owner := a.automation.preview
	owner.capture = true
	a.publishAutomationPreviewLocked(owner)
	a.viewMu.Unlock()
	exact, _ := p.WindowIdentity(1)
	a.emitAutomationPreviewFrame(1, "first", exact)
	snapshot := a.GetAutomationPreviewState(session)
	if snapshot.Frames[1] != "first" || snapshot.FrameSequence != 1 {
		t.Fatalf("snapshot missing initial frame %+v", snapshot)
	}
	snapshot.Frames[1] = "mutated"
	if a.GetAutomationPreviewState(session).Frames[1] != "first" {
		t.Fatal("snapshot aliases live map")
	}
	a.emitAutomationPreviewFrame(1, strings.Repeat("x", 512*1024+1), exact)
	if a.GetAutomationPreviewState(session).FrameSequence != 1 {
		t.Fatal("oversized frame admitted")
	}
}

func TestAutomationCachedFramesCoalescesOnlyExactIdentity(t *testing.T) {
	id := platform.AutomationWindowIdentity{ID: 1, Process: platform.ProcessIdentity{PID: 1, StartSeconds: 123}}
	old := automation.CachedFrame{Window: id, CapturedAt: time.Now().Add(-time.Second), PNG: []byte("old")}
	fresh := automation.CachedFrame{Window: id, CapturedAt: time.Now(), PNG: []byte("new")}
	other := fresh
	other.Window.Process.StartSeconds++
	got := coalesceAutomationFrames([]automation.CachedFrame{fresh, old, other})
	if len(got) != 2 || string(got[0].PNG) != "new" || got[1].Window != other.Window {
		t.Fatalf("incorrect identity coalescing %+v", got)
	}
}

func TestAutomationMergedOwnerCacheQueryExportsExactIdentityOnly(t *testing.T) {
	_, p := automationPreviewFixture(t)
	id, _ := p.WindowIdentity(1)
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	old := automation.CachedFrame{Window: id, CapturedAt: now.Add(-time.Second), PNG: []byte("invalid old bytes")}
	fresh := automation.CachedFrame{Window: id, CapturedAt: now, PNG: data.Bytes()}
	for _, conflict := range []bool{false, true} {
		frames := []automation.CachedFrame{old, fresh}
		if conflict {
			frames[0].Window.Process.StartSeconds++
		}
		service := automation.New(automation.Deps{Apps: func(context.Context) ([]domain.App, error) { return p.Apps() }, Windows: func(context.Context) ([]domain.Window, error) { return p.Windows() }, Identities: p, CachedFrames: func(context.Context) ([]automation.CachedFrame, error) { return coalesceAutomationFrames(frames), nil }})
		reply := service.Handle(context.Background(), platform.AutomationRequest{ID: 1, Operation: platform.AutomationQueryWindows, IncludeImages: true})
		if reply.ErrorCode != "" {
			t.Fatal(reply.ErrorCode)
		}
		var payload struct {
			Windows []struct {
				Image       string
				ImageStatus string
			}
		}
		if err := json.Unmarshal(reply.JSON, &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Windows) != 1 {
			t.Fatalf("missing query target %s", reply.JSON)
		}
		if conflict {
			if payload.Windows[0].Image != "" || payload.Windows[0].ImageStatus != "stale" {
				t.Fatalf("conflicting cached identity exported %s", reply.JSON)
			}
		} else if payload.Windows[0].Image == "" || payload.Windows[0].ImageStatus != "cached" {
			t.Fatalf("same-identity duplicate omitted %s", reply.JSON)
		}
	}
}

func TestAutomationPreviewSelectedCaptureCanReplaceFullCache(t *testing.T) {
	a, p := automationPreviewFixture(t)
	session := showAutomationFixture(t, a, p)
	a.viewMu.Lock()
	owner := a.automation.preview
	owner.capture = true
	for id := domain.WindowID(2); id <= 31; id++ {
		owner.identities[id] = platform.AutomationWindowIdentity{ID: id, Process: owner.process}
		owner.state.Entries = append(owner.state.Entries, switcher.Entry{WindowID: id})
	}
	a.publishAutomationPreviewLocked(owner)
	revision := owner.state.Revision
	a.viewMu.Unlock()
	for id := domain.WindowID(1); id <= 30; id++ {
		a.emitAutomationPreviewFrame(id, "cached", platform.AutomationWindowIdentity{ID: id, Process: owner.process})
	}
	if err := a.SelectAutomationPreview(session, revision, 31); err != nil {
		t.Fatal(err)
	}
	a.emitAutomationPreviewFrame(31, "selected", platform.AutomationWindowIdentity{ID: 31, Process: owner.process})
	if got := a.GetAutomationPreviewState(session); got.Frames[31] != "selected" || len(got.Frames) > 30 {
		t.Fatal("new selected capture starved behind full old cache")
	}
}

func TestAutomationPreviewDefaultsToCurrentDisplayWithMainFallback(t *testing.T) {
	for _, tc := range []struct {
		name   string
		active domain.ScreenID
		point  *platform.AutomationPoint
		want   domain.Bounds
	}{
		{name: "current non-main display", active: 2, want: domain.Bounds{X: 290, Y: 170, W: 620, H: 460}},
		{name: "disconnected current display falls back to main", active: 99, want: domain.Bounds{X: -1030, Y: 120, W: 620, H: 460}},
		{name: "explicit point overrides current display", active: 2, point: &platform.AutomationPoint{X: -1400, Y: 0}, want: domain.Bounds{X: -1400, Y: 0, W: 620, H: 460}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, p := automationPreviewFixture(t)
			// Keep main last: current-display preference must not depend on inventory order.
			p.ScreenList = append([]domain.Screen{{ID: 2, Visible: domain.Bounds{X: 0, Y: 0, W: 1200, H: 800}}}, p.ScreenList...)
			p.ActiveScreenID = tc.active
			windows, _ := p.Windows()
			result, err := a.showAutomationPreviews(context.Background(), platform.ProcessIdentity{PID: 1, StartSeconds: 123}, windows, tc.point, func() error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			if result.Bounds != tc.want {
				t.Fatalf("bounds %+v, want %+v", result.Bounds, tc.want)
			}
		})
	}
}
