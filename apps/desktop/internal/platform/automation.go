package platform

import (
	"context"

	"option-tab/internal/domain"
)

type AutomationOperation string

const (
	AutomationOpenSwitcher      AutomationOperation = "openSwitcher"
	AutomationShowPreviews      AutomationOperation = "showPreviews"
	AutomationHidePreviews      AutomationOperation = "hidePreviews"
	AutomationWindowAction      AutomationOperation = "windowAction"
	AutomationQueryApps         AutomationOperation = "queryApps"
	AutomationQueryWindows      AutomationOperation = "queryWindows"
	AutomationQueryActiveWindow AutomationOperation = "queryActiveWindow"
)

// Selectors match exactly one field against a fresh running-app inventory.
type AutomationAppSelector struct {
	Name     string
	BundleID string
	PID      domain.AppID
}

type AutomationPoint struct{ X, Y float64 }

// ID belongs to the server. Native descriptor objects never cross this port.
type AutomationRequest struct {
	ID                uint64
	Operation         AutomationOperation
	App               *AutomationAppSelector
	WindowID          domain.WindowID
	ActiveWindow      bool
	Action            string
	Fullscreen        *bool
	Mode              string
	Position          *AutomationPoint
	PresentationToken string
	IncludeImages     bool
}

type AutomationReply struct {
	JSON         []byte
	ErrorCode    string
	ErrorMessage string
}

type AutomationHandler func(context.Context, AutomationRequest) AutomationReply

// Run owns and joins workers off the UI thread. Stop closes admission without
// joining; DrainOnMainThread only retires native handlers and suspended replies.
type AutomationServer interface {
	Run(context.Context, AutomationHandler) error
	Stop()
	DrainOnMainThread()
}

type ProcessIdentity struct {
	PID          domain.AppID
	StartSeconds uint64
	StartMicros  uint64
}

type AutomationWindowIdentity struct {
	ID      domain.WindowID
	Process ProcessIdentity
}

type AutomationIdentitySource interface {
	ProcessIdentity(domain.AppID) (ProcessIdentity, error)
	WindowIdentity(domain.WindowID) (AutomationWindowIdentity, error)
	WindowIdentityCurrent(AutomationWindowIdentity) bool
}

type ActiveWindowSource interface {
	ActiveWindow(context.Context) (domain.Window, AutomationWindowIdentity, error)
}

// The native implementation calls guard after preparation, immediately before
// dispatch, then verifies the captured process/window identity again.
type GuardedAutomationWindowPerformer interface {
	PerformAutomationWindowAction(context.Context, string, AutomationWindowIdentity, *bool, func() error) error
}
