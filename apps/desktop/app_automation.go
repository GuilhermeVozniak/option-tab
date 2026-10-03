package main

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"

	"option-tab/internal/preview"

	"option-tab/internal/actions"
	"option-tab/internal/automation"
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
	"option-tab/internal/switcher"
)

type appAutomationRuntime struct {
	ctx                                        context.Context
	cancel                                     context.CancelFunc
	server                                     platform.AutomationServer
	service                                    *automation.Service
	startOnce, stopOnce                        sync.Once
	done                                       chan struct{}
	previewFactory                             func(uint64, func(platform.MediaPanelEvent)) *dockWindow
	preview                                    *automationPreviewOwner
	peer                                       *preview.Manager
	nextPreview, previewRevision, showSequence uint64
	frameLease                                 atomic.Pointer[automationPreviewFrameLease]
	frameMu                                    sync.Mutex
	frames                                     map[domain.WindowID]string
	frameSequence                              uint64
}

func (a *App) wireAutomation(server platform.AutomationServer) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &appAutomationRuntime{ctx: ctx, cancel: cancel, server: server, done: make(chan struct{})}
	a.automation = r
	if a.captures != nil {
		r.peer = a.captures.NewPeerWithIdentity(a.emitAutomationPreviewFrame)
	}
	identities, _ := a.platform.(platform.AutomationIdentitySource)
	active, _ := a.platform.(platform.ActiveWindowSource)
	performer, _ := any(actions.New(a.platform)).(platform.GuardedAutomationWindowPerformer)
	r.service = automation.New(automation.Deps{
		Apps: func(ctx context.Context) ([]domain.App, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if source, ok := a.platform.(platform.ApplicationSource); ok {
				return source.Apps()
			}
			return nil, &automation.Error{Code: "unsupported", Message: "application inventory unavailable"}
		},
		Windows: func(ctx context.Context) ([]domain.Window, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return a.platform.Windows()
		},
		Identities: identities, Active: active, Actions: performer,
		Admission:    a.automationAdmission,
		OpenSwitcher: a.openAutomationSwitcher,
		CachedFrames: a.cachedAutomationFrames,
		ShowPreviews: a.showAutomationPreviews,
		HidePreviews: a.hideAutomationPreviews,
	})
}

func (a *App) startAutomation() {
	r := a.automation
	if r == nil || r.server == nil {
		return
	}
	r.startOnce.Do(func() {
		go func() {
			defer close(r.done)
			if err := r.server.Run(r.ctx, r.service.Handle); err != nil && r.ctx.Err() == nil {
				slog.Error("Local automation unavailable", "error", err)
			}
		}()
	})
}

// Stop never joins workers or dispatches synchronously to AppKit. The native
// reply drain is called separately by Wails' main-thread shutdown hook.
func (a *App) stopAutomation() {
	if r := a.automation; r != nil {
		r.stopOnce.Do(func() {
			r.cancel()
			if r.server != nil {
				r.server.Stop()
			}
		})
	}
}

func (a *App) drainAutomationOnMainThread() {
	if r := a.automation; r != nil && r.server != nil {
		r.server.DrainOnMainThread()
	}
}

func (a *App) automationAdmission(ctx context.Context, op platform.AutomationOperation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.viewMu.Lock()
	defer a.viewMu.Unlock()
	if a.automation == nil || a.automation.ctx.Err() != nil {
		return &automation.Error{Code: "retired", Message: "automation is shutting down"}
	}
	select {
	case <-a.captureStop:
		return &automation.Error{Code: "retired", Message: "application is shutting down"}
	default:
	}
	if a.sessionInactive {
		return &automation.Error{Code: "unavailable", Message: "user session is inactive"}
	}
	readonly := op == platform.AutomationQueryApps || op == platform.AutomationQueryWindows || op == platform.AutomationQueryActiveWindow
	if !readonly && a.settingsSnapshot().Behavior.Paused {
		return &automation.Error{Code: "unavailable", Message: "Option Tab is paused"}
	}
	return ctx.Err()
}

func (a *App) openAutomationSwitcher(ctx context.Context, mode string, guard func() error) (automation.Presentation, error) {
	if a.controller == nil {
		return automation.Presentation{}, &automation.Error{Code: "unsupported", Message: "switcher is unavailable"}
	}
	st, err := a.controller.OpenGuarded(config.SwitcherMode(mode), guard)
	if err != nil {
		code := "unavailable"
		if errors.Is(err, switcher.ErrUnsupportedMode) {
			code = "unsupported"
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return automation.Presentation{}, err
		}
		var typed *automation.Error
		if errors.As(err, &typed) {
			return automation.Presentation{}, err
		}
		return automation.Presentation{}, &automation.Error{Code: code, Message: err.Error()}
	}
	return automation.Presentation{Token: strconv.FormatUint(st.Session, 10), Status: "accepted"}, nil
}

func (a *App) cachedAutomationFrames(ctx context.Context) ([]automation.CachedFrame, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := []automation.CachedFrame{}
	if a.captures != nil {
		for _, frame := range a.captures.CachedFrames() {
			result = append(result, automation.CachedFrame{Window: frame.Window, CapturedAt: frame.CapturedAt, PNG: frame.PNG})
		}
	}
	if a.automation != nil && a.automation.peer != nil {
		for _, frame := range a.automation.peer.CachedFrames() {
			result = append(result, automation.CachedFrame{Window: frame.Window, CapturedAt: frame.CapturedAt, PNG: frame.PNG})
		}
	}
	return coalesceAutomationFrames(result), nil
}

// Multiple presentation owners may capture the same exact window. Coalesce
// only complete identities; conflicting generations remain visible to the
// query service, which refuses ambiguous cached bytes.
func coalesceAutomationFrames(frames []automation.CachedFrame) []automation.CachedFrame {
	result := make([]automation.CachedFrame, 0, len(frames))
	indices := make(map[platform.AutomationWindowIdentity]int, len(frames))
	for _, frame := range frames {
		if index, ok := indices[frame.Window]; ok {
			if !frame.CapturedAt.Before(result[index].CapturedAt) {
				result[index] = frame
			}
			continue
		}
		indices[frame.Window] = len(result)
		result = append(result, frame)
	}
	return result
}
