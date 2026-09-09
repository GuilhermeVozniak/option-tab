package platform

import (
	"context"
	"math"
)

// AutomationNativeError keeps stable wire semantics without importing the
// orchestration package into its platform dependency.
type AutomationNativeError struct{ Code, Message string }

func (e *AutomationNativeError) Error() string               { return e.Message }
func (e *AutomationNativeError) AutomationErrorCode() string { return e.Code }
func ValidateAutomationWindowAction(ctx context.Context, kind string, id AutomationWindowIdentity, fullscreen *bool, guard func() error) error {
	if ctx == nil || guard == nil {
		return &AutomationNativeError{"invalidArgument", "automation action requires context and final guard"}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if id.ID == 0 || uint64(id.ID) > math.MaxUint32 || id.Process.PID <= 0 || id.Process.PID > math.MaxInt32 || id.Process.StartSeconds == 0 || id.Process.StartMicros >= 1_000_000 {
		return &AutomationNativeError{"invalidArgument", "invalid automation window identity"}
	}
	switch kind {
	case "focus", "close", "minimize", "hide":
		if fullscreen != nil {
			return &AutomationNativeError{"invalidArgument", "fullscreen state supplied for another action"}
		}
	case "fullscreen":
		if fullscreen == nil {
			return &AutomationNativeError{"invalidArgument", "fullscreen requires an explicit desired state"}
		}
	default:
		return &AutomationNativeError{"unsupported", "unsupported automation window action"}
	}
	return nil
}
