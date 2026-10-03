//go:build !darwin

package platform

import (
	"context"
	"errors"

	"option-tab/internal/domain"
)

func (s *stub) ObserveWindowDrags(ctx context.Context, _ func(WindowDragEvent)) error {
	if ctx != nil && ctx.Err() != nil {
		return nil
	}
	return errors.New("passive window drag is unsupported on this platform")
}
func (s *stub) WindowDragGestureCurrent(WindowDragValidation) bool { return false }
func (s *stub) ActionWindowRoles() ([]ActionWindowRole, error) {
	return nil, errors.New("window role discovery is unsupported on this platform")
}

func (s *stub) PerformOtherWindowAction(string, domain.WindowID, domain.AppID) error {
	return errors.New("other-window action is unsupported on this platform")
}

func (s *stub) PerformOtherWindowActionGuarded(string, domain.WindowID, domain.AppID, func() error) error {
	return errors.New("guarded other-window action is unsupported on this platform")
}
