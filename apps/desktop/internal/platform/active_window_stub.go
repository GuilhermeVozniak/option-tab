//go:build !darwin

package platform

import (
	"context"

	"option-tab/internal/domain"
)

func (*stub) ProcessIdentity(domain.AppID) (ProcessIdentity, error) {
	return ProcessIdentity{}, &AutomationNativeError{"unsupported", "native process identity unavailable"}
}

func (*stub) WindowIdentity(domain.WindowID) (AutomationWindowIdentity, error) {
	return AutomationWindowIdentity{}, &AutomationNativeError{"unsupported", "native window identity unavailable"}
}
func (*stub) WindowIdentityCurrent(AutomationWindowIdentity) bool { return false }
func (*stub) ActiveWindow(context.Context) (domain.Window, AutomationWindowIdentity, error) {
	return domain.Window{}, AutomationWindowIdentity{}, &AutomationNativeError{"unsupported", "authoritative active window unavailable"}
}

func (*stub) PerformAutomationWindowAction(context.Context, string, AutomationWindowIdentity, *bool, func() error) error {
	return &AutomationNativeError{"unsupported", "native automation actions unavailable"}
}
