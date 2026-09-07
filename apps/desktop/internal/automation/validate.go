package automation

import (
	"math"
	"strconv"
	"unicode/utf8"

	"option-tab/internal/platform"
)

func validText(s string, max int) bool { return len(s) <= max && utf8.ValidString(s) }
func validate(r platform.AutomationRequest) error {
	invalid := func() error { return failure("invalidArgument", "invalid parameters for automation operation") }
	if r.ID == 0 {
		return invalid()
	}
	if r.App != nil {
		n := 0
		if r.App.Name != "" {
			n++
		}
		if r.App.BundleID != "" {
			n++
		}
		if r.App.PID != 0 {
			n++
		}
		if n != 1 || r.App.PID < 0 || r.App.PID > math.MaxInt32 || !validText(r.App.Name, 1024) || !validText(r.App.BundleID, 1024) {
			return invalid()
		}
	}
	if !validText(r.PresentationToken, 1024) || !validText(r.Mode, 32) || !validText(r.Action, 32) {
		return invalid()
	}
	if r.Position != nil && (math.IsNaN(r.Position.X) || math.IsNaN(r.Position.Y) || math.IsInf(r.Position.X, 0) || math.IsInf(r.Position.Y, 0)) {
		return invalid()
	}
	if r.Operation != platform.AutomationShowPreviews && r.Position != nil {
		return invalid()
	}
	if r.Operation != platform.AutomationOpenSwitcher && r.Mode != "" {
		return invalid()
	}
	if r.Operation != platform.AutomationHidePreviews && r.PresentationToken != "" {
		return invalid()
	}
	if r.Operation != platform.AutomationWindowAction && (r.WindowID != 0 || r.ActiveWindow || r.Action != "" || r.Fullscreen != nil) {
		return invalid()
	}
	if r.IncludeImages && r.Operation != platform.AutomationQueryWindows && r.Operation != platform.AutomationQueryActiveWindow {
		return invalid()
	}
	if r.App != nil && r.Operation != platform.AutomationShowPreviews && r.Operation != platform.AutomationQueryWindows && r.Operation != platform.AutomationWindowAction {
		return invalid()
	}
	switch r.Operation {
	case platform.AutomationOpenSwitcher:
		if r.Mode != "" && r.Mode != "windows" && r.Mode != "apps" {
			return invalid()
		}
	case platform.AutomationShowPreviews:
		if r.App == nil {
			return invalid()
		}
	case platform.AutomationHidePreviews:
		token, err := strconv.ParseUint(r.PresentationToken, 10, 64)
		if err != nil || token == 0 {
			return invalid()
		}
	case platform.AutomationWindowAction:
		if (r.WindowID != 0) == r.ActiveWindow || r.WindowID > math.MaxUint32 {
			return invalid()
		}
		switch r.Action {
		case "focus", "close", "minimize", "hide":
			if r.Fullscreen != nil {
				return invalid()
			}
		case "fullscreen":
			if r.Fullscreen == nil {
				return invalid()
			}
		default:
			return invalid()
		}
	case platform.AutomationQueryApps, platform.AutomationQueryWindows, platform.AutomationQueryActiveWindow:
	default:
		return failure("unsupported", "unsupported automation operation")
	}
	return nil
}
