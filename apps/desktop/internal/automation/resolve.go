package automation

import (
	"context"
	"slices"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func (s *Service) admit(ctx context.Context, op platform.AutomationOperation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	requestErr, _ := ctx.Value(requestErrKey{}).(func() error)
	if requestErr != nil {
		if err := requestErr(); err != nil {
			return err
		}
	}
	if s.deps.Admission != nil {
		if err := s.deps.Admission(ctx, op); err != nil {
			return err
		}
	}
	if requestErr != nil {
		if err := requestErr(); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func validProcess(p platform.ProcessIdentity) bool {
	return p.PID > 0 && (p.StartSeconds != 0 || p.StartMicros != 0) && p.StartMicros < 1_000_000
}

func (s *Service) process(id domain.AppID) (platform.ProcessIdentity, error) {
	if s.deps.Identities == nil {
		return platform.ProcessIdentity{}, failure("unsupported", "process identity source unavailable")
	}
	p, err := s.deps.Identities.ProcessIdentity(id)
	if err != nil {
		return p, err
	}
	if !validProcess(p) || p.PID != id {
		return p, failure("staleIdentity", "application identity changed")
	}
	return p, nil
}

func (s *Service) processGuard(ctx context.Context, op platform.AutomationOperation, p platform.ProcessIdentity) func() error {
	return func() error {
		if err := s.admit(ctx, op); err != nil {
			return err
		}
		now, err := s.process(p.PID)
		if err != nil {
			return err
		}
		if now != p {
			return failure("staleIdentity", "application process was replaced")
		}
		return s.admit(ctx, op)
	}
}

func (s *Service) windowGuard(ctx context.Context, op platform.AutomationOperation, w platform.AutomationWindowIdentity) func() error {
	return func() error {
		if err := s.processGuard(ctx, op, w.Process)(); err != nil {
			return err
		}
		if !s.deps.Identities.WindowIdentityCurrent(w) {
			return failure("staleIdentity", "window identity changed")
		}
		return s.admit(ctx, op)
	}
}

func (s *Service) apps(ctx context.Context) ([]domain.App, error) {
	if s.deps.Apps == nil {
		return nil, failure("unsupported", "application inventory unavailable")
	}
	v, e := s.deps.Apps(ctx)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return slices.Clone(v), nil
}

func (s *Service) windows(ctx context.Context) ([]domain.Window, error) {
	if s.deps.Windows == nil {
		return nil, failure("unsupported", "window inventory unavailable")
	}
	v, e := s.deps.Windows(ctx)
	if e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return slices.Clone(v), nil
}

func matches(a domain.App, sel platform.AutomationAppSelector) bool {
	return (sel.Name != "" && a.Name == sel.Name) || (sel.BundleID != "" && a.BundleID == sel.BundleID) || (sel.PID != 0 && a.ID == sel.PID)
}

func (s *Service) resolveApp(ctx context.Context, sel platform.AutomationAppSelector) (domain.App, platform.ProcessIdentity, error) {
	apps, err := s.apps(ctx)
	if err != nil {
		return domain.App{}, platform.ProcessIdentity{}, err
	}
	var found domain.App
	n := 0
	for _, a := range apps {
		if matches(a, sel) {
			n++
			found = a
		}
	}
	if n == 0 {
		return found, platform.ProcessIdentity{}, failure("notFound", "no matching running application")
	}
	if n != 1 {
		return found, platform.ProcessIdentity{}, failure("ambiguous", "application selector matches multiple running instances")
	}
	p, err := s.process(found.ID)
	if err != nil {
		return found, p, err
	}
	// Inventory and process reads are separate. Verify the selected name/bundle
	// still describes the captured process before granting execution authority.
	fresh, err := s.apps(ctx)
	if err != nil {
		return found, p, err
	}
	n = 0
	for _, a := range fresh {
		if matches(a, sel) {
			n++
			if a.ID != found.ID || a.Name != found.Name || a.BundleID != found.BundleID {
				return found, p, failure("staleIdentity", "application changed during resolution")
			}
		}
	}
	if n != 1 {
		return found, p, failure("staleIdentity", "application inventory changed during resolution")
	}
	current, err := s.process(p.PID)
	if err != nil {
		return found, p, err
	}
	if current != p {
		return found, p, failure("staleIdentity", "application process changed during resolution")
	}
	if err = ctx.Err(); err != nil {
		return found, p, err
	}
	return found, p, nil
}

func (s *Service) windowIdentity(w domain.Window) (platform.AutomationWindowIdentity, error) {
	if s.deps.Identities == nil {
		return platform.AutomationWindowIdentity{}, failure("unsupported", "window identity source unavailable")
	}
	id, err := s.deps.Identities.WindowIdentity(w.ID)
	if err != nil {
		return id, err
	}
	if id.ID == 0 || id.ID != w.ID || !validProcess(id.Process) || id.Process.PID != w.AppID {
		return id, failure("staleIdentity", "window owner changed")
	}
	return id, nil
}

func (s *Service) active(ctx context.Context) (domain.Window, platform.AutomationWindowIdentity, error) {
	if s.deps.Active == nil {
		return domain.Window{}, platform.AutomationWindowIdentity{}, failure("unsupported", "authoritative active window source unavailable")
	}
	w, id, err := s.deps.Active.ActiveWindow(ctx)
	if err != nil {
		return w, id, err
	}
	if w.ID == 0 || id.ID != w.ID || id.Process.PID != w.AppID || !validProcess(id.Process) {
		return w, id, failure("unavailable", "no authoritative active window")
	}
	return w, id, nil
}
