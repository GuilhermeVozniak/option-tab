package platform

import "option-tab/internal/domain"

// ApplicationSource includes regular running apps even when they have no windows.
type ApplicationSource interface {
	Apps() ([]domain.App, error)
}

// ApplicationActivator requests activation of an explicitly identified app.
type ApplicationActivator interface {
	ActivateApp(domain.AppID) error
}

// WindowPresence separates a positively empty inventory from unavailable AX
// metadata. Unknown apps can still be activated, but do not acquire invented
// window targets or satisfy the "hide when no windows" rule.
type WindowPresence string

const (
	WindowsPresent WindowPresence = "present"
	WindowsNone    WindowPresence = "none"
	WindowsUnknown WindowPresence = "unknown"
)

type ApplicationWindowPresenceSource interface {
	AppWindowPresence(domain.AppID) WindowPresence
}
