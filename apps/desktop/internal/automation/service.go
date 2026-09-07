package automation

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type requestErrKey struct{}

func (s *Service) Handle(parent context.Context, r platform.AutomationRequest) platform.AutomationReply {
	if parent == nil {
		return replyError(failure("invalidArgument", "request context required"))
	}
	if err := validate(r); err != nil {
		return replyError(err)
	}
	limit := 5 * time.Second
	if deadline, ok := parent.Deadline(); ok {
		limit = min(10*time.Second, time.Until(deadline))
	}
	ctx, cancel := context.WithTimeout(parent, limit)
	defer cancel()
	// Preserve custom request admission carried by the native parent. A derived
	// cancelCtx does not call an overridden parent Err method on every check.
	ctx = context.WithValue(ctx, requestErrKey{}, func() error { return parent.Err() })
	// Freeze caller-owned pointers before any injected operation.
	if r.App != nil {
		v := *r.App
		r.App = &v
	}
	if r.Position != nil {
		v := *r.Position
		r.Position = &v
	}
	if r.Fullscreen != nil {
		v := *r.Fullscreen
		r.Fullscreen = &v
	}
	if err := s.admit(ctx, r.Operation); err != nil {
		return replyError(err)
	}
	out := envelope{SchemaVersion: 1, Status: "ok"}
	var err error
	switch r.Operation {
	case platform.AutomationQueryApps:
		err = s.queryApps(ctx, &out)
	case platform.AutomationQueryWindows:
		err = s.queryWindows(ctx, r, &out)
	case platform.AutomationQueryActiveWindow:
		var w domain.Window
		var id platform.AutomationWindowIdentity
		w, id, err = s.active(ctx)
		if err == nil {
			err = s.windowGuard(ctx, r.Operation, id)()
		}
		if err == nil {
			v := windowView(w, id)
			out.Active = &v
			if r.IncludeImages {
				s.images(ctx, r.Operation, []*WindowView{out.Active}, []platform.AutomationWindowIdentity{id}, &out)
			}
		}
	case platform.AutomationWindowAction:
		err = s.action(ctx, r)
	case platform.AutomationOpenSwitcher:
		if s.deps.OpenSwitcher == nil {
			err = failure("unsupported", "switcher presentation unavailable")
			break
		}
		mode := r.Mode
		if mode == "" {
			mode = "windows"
		}
		var p Presentation
		p, err = s.deps.OpenSwitcher(ctx, mode, func() error { return s.admit(ctx, r.Operation) })
		out.Presentation = &p
	case platform.AutomationHidePreviews:
		if s.deps.HidePreviews == nil {
			err = failure("unsupported", "automation previews unavailable")
			break
		}
		var p Presentation
		p, err = s.deps.HidePreviews(ctx, r.PresentationToken, func() error { return s.admit(ctx, r.Operation) })
		out.Presentation = &p
	case platform.AutomationShowPreviews:
		out.Presentation, err = s.show(ctx, r)
	}
	if err != nil {
		if ctx.Err() != nil {
			return replyError(ctx.Err())
		}
		return replyError(err)
	}
	if err = s.admit(ctx, r.Operation); err != nil {
		return replyError(err)
	}
	data, err := encodeBounded(&out)
	if err != nil {
		return replyError(err)
	}
	if err = ctx.Err(); err != nil {
		return replyError(err)
	}
	return platform.AutomationReply{JSON: data}
}

func replyError(err error) platform.AutomationReply {
	code := "unavailable"
	message := err.Error()
	var typed *Error
	var native interface{ AutomationErrorCode() string }
	candidate := ""
	if errors.As(err, &typed) {
		candidate = typed.Code
	} else if errors.As(err, &native) {
		candidate = native.AutomationErrorCode()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		code = "timeout"
	case errors.Is(err, context.Canceled):
		code = "cancelled"
	case candidate != "":
		switch candidate {
		case "invalidArgument", "ambiguous", "notFound", "unavailable", "permissionDenied", "unsupported", "staleIdentity", "retired", "cancelled", "timeout", "busy", "internal":
			code = candidate
		default:
			code = "internal"
		}
	}
	if !utf8.ValidString(message) {
		message = "automation request failed"
	}
	if len(message) > 1024 {
		message = message[:1024]
		for !utf8.ValidString(message) {
			message = message[:len(message)-1]
		}
	}
	return platform.AutomationReply{ErrorCode: code, ErrorMessage: message}
}

func (s *Service) action(ctx context.Context, r platform.AutomationRequest) error {
	if s.deps.Actions == nil {
		return failure("unsupported", "guarded window actions unavailable")
	}
	var w domain.Window
	var id platform.AutomationWindowIdentity
	var err error
	if r.ActiveWindow {
		_, id, err = s.active(ctx)
	} else {
		var windows []domain.Window
		windows, err = s.windows(ctx)
		if err == nil {
			n := 0
			for _, candidate := range windows {
				if candidate.ID == r.WindowID {
					w = candidate
					n++
				}
			}
			switch n {
			case 0:
				err = failure("notFound", "window not found")
			case 1:
				id, err = s.windowIdentity(w)
			default:
				err = failure("ambiguous", "window inventory contains duplicate identity")
			}
		}
	}
	if err != nil {
		return err
	}
	if r.App != nil {
		_, p, e := s.resolveApp(ctx, *r.App)
		if e != nil {
			return e
		}
		if p != id.Process {
			return failure("staleIdentity", "window does not belong to selected application")
		}
	}
	guard := s.windowGuard(ctx, r.Operation, id)
	if err = guard(); err != nil {
		return err
	}
	return s.deps.Actions.PerformAutomationWindowAction(ctx, r.Action, id, r.Fullscreen, guard)
}

func (s *Service) show(ctx context.Context, r platform.AutomationRequest) (*Presentation, error) {
	if s.deps.ShowPreviews == nil {
		return nil, failure("unsupported", "automation previews unavailable")
	}
	app, p, err := s.resolveApp(ctx, *r.App)
	if err != nil {
		return nil, err
	}
	windows, err := s.windows(ctx)
	if err != nil {
		return nil, err
	}
	selected := make([]domain.Window, 0)
	ids := make([]platform.AutomationWindowIdentity, 0)
	for _, w := range windows {
		if w.AppID != app.ID {
			continue
		}
		if len(selected) == MaxEntries {
			return nil, failure("busy", "too many application windows for preview")
		}
		id, e := s.windowIdentity(w)
		if e != nil {
			return nil, e
		}
		if id.Process != p {
			return nil, failure("staleIdentity", "preview window process changed")
		}
		selected = append(selected, w)
		ids = append(ids, id)
	}
	guard := func() error {
		if e := s.processGuard(ctx, r.Operation, p)(); e != nil {
			return e
		}
		for _, id := range ids {
			if e := s.windowGuard(ctx, r.Operation, id)(); e != nil {
				return e
			}
		}
		return s.admit(ctx, r.Operation)
	}
	if err = guard(); err != nil {
		return nil, err
	}
	result, err := s.deps.ShowPreviews(ctx, p, selected, r.Position, guard)
	return &result, err
}
