package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type dragPlatform struct {
	*fake.Fake
	requests       chan platform.PreviewDragRequest
	windowsEntered chan struct{}
	windowsRelease chan struct{}
}

func (p *dragPlatform) Windows() ([]domain.Window, error) {
	if p.windowsEntered != nil {
		close(p.windowsEntered)
		<-p.windowsRelease
	}
	return p.Fake.Windows()
}

func (p *dragPlatform) DragPreview(ctx context.Context, request platform.PreviewDragRequest) (platform.PreviewDragResult, error) {
	p.requests <- request
	<-ctx.Done()
	return platform.PreviewDragResult{}, ctx.Err()
}

func dragApp(t *testing.T) (*App, *dragPlatform) {
	t.Helper()
	p := &dragPlatform{Fake: fake.New(), requests: make(chan platform.PreviewDragRequest, 2)}
	p.SetWindows([]domain.Window{{ID: 101, AppID: 10, Title: "Exact", OnScreen: true, SpaceID: 1, ScreenID: 1}})
	s := dockEnabledSettings()
	s.Dock.Input.PreviewDrag = true
	a := newApp(p, s, "")
	a.showDock(dockFixtureState(7, 101, 10), true)
	t.Cleanup(a.stopCapture)
	return a, p
}

func TestDockPreviewDragUsesExactTargetAndEarlyCancel(t *testing.T) {
	a, p := dragApp(t)
	done := make(chan error, 1)
	go func() { done <- a.BeginDockPreviewDrag(7, 1, 101, 10, 500, 300, 1.4, -.2) }()
	select {
	case request := <-p.requests:
		if request.WindowID != 101 || request.AppID != 10 || request.PointerX != 500 || request.PointerY != 300 || request.GrabX != 1 || request.GrabY != 0 {
			t.Fatalf("request=%+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("native drag was not started")
	}
	a.CancelDockPreviewDrag(7, 1)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
}

func TestDockPreviewDragSurvivesHoverHideButFeatureDisableCancels(t *testing.T) {
	a, p := dragApp(t)
	done := make(chan error, 1)
	go func() { done <- a.BeginDockPreviewDrag(7, 2, 101, 10, 1, 2, .5, .5) }()
	<-p.requests
	a.hideDock(7)
	select {
	case err := <-done:
		t.Fatalf("ordinary hide cancelled drag: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	s := a.settingsSnapshot()
	s.Dock.Input.PreviewDrag = false
	a.syncDockPreviewDrag(s)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("disable error=%v", err)
	}
}

func TestDockPreviewDragDefaultsDisabled(t *testing.T) {
	a, _ := dragApp(t)
	a.settings.Dock.Input = config.Default().Dock.Input
	if err := a.BeginDockPreviewDrag(7, 3, 101, 10, 1, 2, .5, .5); err == nil {
		t.Fatal("disabled drag admitted")
	}
}

func TestDockPreviewDragSettingsChangeCancelsAdmittedOwner(t *testing.T) {
	a, p := dragApp(t)
	done := make(chan error, 1)
	go func() { done <- a.BeginDockPreviewDrag(7, 20, 101, 10, 1, 2, .5, .5) }()
	<-p.requests
	s := a.settingsSnapshot()
	s.Filters.AppBlacklist = []config.BlacklistEntry{{Match: "fixture.app", Hide: config.HideAlways}}
	a.configureDock(s)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("settings cancellation=%v", err)
		}
	case <-time.After(time.Second):
		a.CancelDockPreviewDrag(7, 20)
		<-done
		t.Fatal("settings change left the admitted drag active")
	}
}

func TestDockPreviewDragCancelTombstoneWinsBeforeAsyncBegin(t *testing.T) {
	a, p := dragApp(t)
	a.CancelDockPreviewDrag(7, 9)
	if err := a.BeginDockPreviewDrag(7, 9, 101, 10, 1, 2, .5, .5); err == nil {
		t.Fatal("cancelled gesture revived")
	}
	select {
	case request := <-p.requests:
		t.Fatalf("cancelled request reached native: %+v", request)
	default:
	}
}

func TestDockPreviewDragFloorIncludesCurrentSessionCancellation(t *testing.T) {
	a, _ := dragApp(t)
	a.CancelDockPreviewDrag(7, 44)
	if floor := a.GetDockState().DragGestureFloor; floor != 44 {
		t.Fatalf("floor=%d", floor)
	}
}

func TestDockPreviewDragDisableWhileFreshLookupBlockedPreventsNativeStart(t *testing.T) {
	a, p := dragApp(t)
	p.windowsEntered, p.windowsRelease = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- a.BeginDockPreviewDrag(7, 10, 101, 10, 1, 2, .5, .5) }()
	<-p.windowsEntered
	s := a.settingsSnapshot()
	s.Dock.Input.PreviewDrag = false
	a.syncDockPreviewDrag(s)
	close(p.windowsRelease)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked prepare error=%v", err)
	}
	select {
	case request := <-p.requests:
		t.Fatalf("cancelled prepare reached native: %+v", request)
	default:
	}
}

func TestDockPreviewDragAcceptedBeforeHoverHideContinuesNativeHandoff(t *testing.T) {
	a, p := dragApp(t)
	p.windowsEntered, p.windowsRelease = make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- a.BeginDockPreviewDrag(7, 11, 101, 10, 1, 2, .5, .5) }()
	<-p.windowsEntered
	a.hideDock(7)
	close(p.windowsRelease)
	select {
	case request := <-p.requests:
		if request.Session != 7 || request.GestureID != 11 {
			t.Fatalf("request=%+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted off-panel drag did not reach native")
	}
	a.CancelDockPreviewDrag(7, 11)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
}
