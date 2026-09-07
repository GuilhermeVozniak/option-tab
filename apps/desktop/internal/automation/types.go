// Package automation implements the bounded, typed local automation service.
// It has no native event registration, permission, capture, or UI dependencies.
package automation

import (
	"context"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

const (
	MaxEntries    = 500
	MaxReplyBytes = 4 * 1024 * 1024
	MaxImages     = 8
	MaxImageBytes = 512 * 1024 // Complete encoded PNG data URL, not raw pixel bytes.
	MaxImageAge   = 30 * time.Second
)

type Presentation struct {
	Token  string        `json:"token,omitempty"`
	Status string        `json:"status"`
	Bounds domain.Bounds `json:"bounds"`
}
type CachedFrame struct {
	Window     platform.AutomationWindowIdentity
	CapturedAt time.Time
	PNG        []byte
}

// Deps callbacks receive copied inventories. Presentation callbacks must run the
// supplied guard after preparation, immediately before admitting a UI change.
// Nil Admission means no additional App policy (useful for pure fixtures).
// CachedFrames returns owned copies without capture, refresh, permission or I/O.
type Deps struct {
	Apps         func(context.Context) ([]domain.App, error)
	Windows      func(context.Context) ([]domain.Window, error)
	Identities   platform.AutomationIdentitySource
	Active       platform.ActiveWindowSource
	Actions      platform.GuardedAutomationWindowPerformer
	OpenSwitcher func(context.Context, string, func() error) (Presentation, error)
	ShowPreviews func(context.Context, platform.ProcessIdentity, []domain.Window, *platform.AutomationPoint, func() error) (Presentation, error)
	HidePreviews func(context.Context, string, func() error) (Presentation, error)
	CachedFrames func(context.Context) ([]CachedFrame, error)
	Admission    func(context.Context, platform.AutomationOperation) error
	Now          func() time.Time
}
type Service struct{ deps Deps }

func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{deps: d}
}

type Error struct{ Code, Message string }

func (e *Error) Error() string           { return e.Message }
func failure(code, message string) error { return &Error{code, message} }

type ProcessView struct {
	PID          domain.AppID `json:"pid"`
	StartSeconds string       `json:"startSeconds"`
	StartMicros  uint64       `json:"startMicros"`
}
type AppView struct {
	PID      domain.AppID `json:"pid"`
	Name     string       `json:"name"`
	BundleID string       `json:"bundleID"`
	Hidden   bool         `json:"hidden"`
	Process  ProcessView  `json:"process"`
}
type WindowView struct {
	WindowID    string          `json:"windowID"`
	AppID       domain.AppID    `json:"appID"`
	AppName     string          `json:"appName"`
	BundleID    string          `json:"bundleID"`
	Title       string          `json:"title"`
	Bounds      domain.Bounds   `json:"bounds"`
	ScreenID    domain.ScreenID `json:"screenID"`
	SpaceID     string          `json:"spaceID"`
	Minimized   bool            `json:"minimized"`
	Hidden      bool            `json:"hidden"`
	Fullscreen  bool            `json:"fullscreen"`
	OnScreen    bool            `json:"onScreen"`
	Process     ProcessView     `json:"process"`
	ImageStatus string          `json:"imageStatus,omitempty"`
	Image       string          `json:"image,omitempty"`
	CapturedAt  *time.Time      `json:"capturedAt,omitempty"`
}
type envelope struct {
	SchemaVersion int           `json:"schemaVersion"`
	Status        string        `json:"status"`
	Apps          *[]AppView    `json:"apps,omitempty"`
	Windows       *[]WindowView `json:"windows,omitempty"`
	Active        *WindowView   `json:"active,omitempty"`
	Presentation  *Presentation `json:"presentation,omitempty"`
	Truncated     bool          `json:"truncated"`
	Omitted       int           `json:"omitted"`
	ImagesOmitted int           `json:"imagesOmitted,omitempty"`
}
