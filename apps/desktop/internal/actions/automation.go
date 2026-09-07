package actions

import (
	"context"

	"option-tab/internal/platform"
)

// PerformAutomationWindowAction is a separate captured-identity route: never
// substitute the legacy toggling minimize/fullscreen or an unguarded performer.
func (s *Service) PerformAutomationWindowAction(ctx context.Context, kind string, id platform.AutomationWindowIdentity, fullscreen *bool, guard func() error) error {
	if err := platform.ValidateAutomationWindowAction(ctx, kind, id, fullscreen, guard); err != nil {
		return err
	}
	native, ok := s.backend.(platform.GuardedAutomationWindowPerformer)
	if !ok {
		return &platform.AutomationNativeError{Code: "unsupported", Message: "guarded automation window actions unavailable"}
	}
	final := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		return ctx.Err()
	}
	if err := final(); err != nil {
		return err
	}
	if fullscreen != nil {
		value := *fullscreen
		fullscreen = &value
	}
	return native.PerformAutomationWindowAction(ctx, kind, id, fullscreen, final)
}
