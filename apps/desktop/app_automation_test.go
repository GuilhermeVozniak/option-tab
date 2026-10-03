package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/platform/fake"
)

type appAutomationServer struct {
	started             chan struct{}
	runs, stops, drains atomic.Int32
}

func (s *appAutomationServer) Run(ctx context.Context, handler platform.AutomationHandler) error {
	s.runs.Add(1)
	close(s.started)
	<-ctx.Done()
	return ctx.Err()
}
func (s *appAutomationServer) Stop()              { s.stops.Add(1) }
func (s *appAutomationServer) DrainOnMainThread() { s.drains.Add(1) }

func TestAppAutomationShutdownRetiresAdmissionAndJoinsOffMain(t *testing.T) {
	a := newApp(fake.New(), config.Default(), "")
	s := &appAutomationServer{started: make(chan struct{})}
	a.wireAutomation(s)
	a.startAutomation()
	a.startAutomation()
	select {
	case <-s.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start")
	}
	a.stopCapture()
	select {
	case <-a.automation.done:
	case <-time.After(time.Second):
		t.Fatal("server worker did not join")
	}
	if s.runs.Load() != 1 || s.stops.Load() != 1 || s.drains.Load() != 0 {
		t.Fatalf("incorrect shutdown ownership: runs %d stops %d drains %d", s.runs.Load(), s.stops.Load(), s.drains.Load())
	}
	result := a.automation.service.Handle(context.Background(), platform.AutomationRequest{ID: 1, Operation: platform.AutomationQueryApps})
	if result.ErrorCode != "retired" {
		t.Fatalf("shutdown admitted query: %+v", result)
	}
	a.drainAutomationOnMainThread()
	if s.drains.Load() != 1 {
		t.Fatal("native cleanup was not explicitly drained")
	}
}

type appAutomationPlatform struct{ *fake.Fake }

func (p *appAutomationPlatform) Apps() ([]domain.App, error) {
	return []domain.App{{ID: 1, Name: "Fixture", BundleID: "test.fixture"}}, nil
}

func (p *appAutomationPlatform) ProcessIdentity(pid domain.AppID) (platform.ProcessIdentity, error) {
	return platform.ProcessIdentity{PID: pid, StartSeconds: 123}, nil
}

func (p *appAutomationPlatform) WindowIdentity(id domain.WindowID) (platform.AutomationWindowIdentity, error) {
	return platform.AutomationWindowIdentity{ID: id, Process: platform.ProcessIdentity{PID: 1, StartSeconds: 123}}, nil
}

func (p *appAutomationPlatform) WindowIdentityCurrent(id platform.AutomationWindowIdentity) bool {
	return id.Process == (platform.ProcessIdentity{PID: 1, StartSeconds: 123}) && id.ID == 1
}

func TestAppAutomationQueriesRemainReadonlyAndOpenIsIdempotent(t *testing.T) {
	p := &appAutomationPlatform{fake.New()}
	p.ActiveAppID = 1
	p.SetWindows([]domain.Window{{ID: 1, AppID: 1, PID: 1, AppName: "Fixture", BundleID: "test.fixture", Title: "Original", Bounds: domain.Bounds{W: 800, H: 600}, OnScreen: true, ScreenID: 1, SpaceID: 1}})
	a := newApp(p, config.Default(), "")
	a.wireAutomation(&appAutomationServer{})
	defer a.stopCapture()
	for _, op := range []platform.AutomationOperation{platform.AutomationQueryApps, platform.AutomationQueryWindows} {
		result := a.automation.service.Handle(context.Background(), platform.AutomationRequest{ID: 1, Operation: op})
		if result.ErrorCode != "" || len(result.JSON) == 0 {
			t.Fatalf("query failed: %+v", result)
		}
	}
	if len(p.RequestCalls) != 0 || len(p.FocusCalls) != 0 || a.controller.IsOpen() {
		t.Fatal("query changed presentation or permissions")
	}
	r := platform.AutomationRequest{ID: 1, Operation: platform.AutomationOpenSwitcher, Mode: "windows"}
	if result := a.automation.service.Handle(context.Background(), r); result.ErrorCode != "" {
		t.Fatalf("open failed: %+v", result)
	}
	before := a.controller.State()
	if result := a.automation.service.Handle(context.Background(), r); result.ErrorCode != "" {
		t.Fatalf("repeat open failed: %+v", result)
	}
	after := a.controller.State()
	if before.Session != after.Session || before.Selected != after.Selected {
		t.Fatal("automation cycled existing switcher")
	}
}
