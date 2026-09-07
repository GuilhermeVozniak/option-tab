package platform

import (
	"context"
	"time"
	"unsafe"

	"option-tab/internal/domain"
)

// LauncherEnvironment is a read-only observation. Coordinates are global,
// top-left logical points. Failed/incomplete observations never imply that a
// display disconnected or that a Space is ordinary.
type LauncherEnvironment struct {
	Generation, Sequence uint64
	ObservedAt           time.Time
	Complete             bool
	Status, Reason       string
	Displays             []LauncherDisplay
	PointerX, PointerY   float64
	PointerKnown         bool
	NativeDock           LauncherNativeDock
}

type LauncherDisplay struct {
	UUID, Name, MirrorGroup string
	ID                      domain.ScreenID
	Main                    bool
	Frame, UsableFrame      domain.Bounds
	Scale                   float64
	SpaceID                 uint64
	SpaceKind               string // ordinary|fullscreen|system|unknown
	SpaceStatus             string // known|unsupported|unavailable|transition
}

type LauncherNativeDock struct {
	Process                      ProcessIdentity
	Edge, Visibility, Confidence string // bottom|left|right|unknown; visible|hidden|unknown; known|unknown
	Bounds                       domain.Bounds
}

type LauncherEnvironmentSource interface {
	// Observe owns and joins all its observers before returning. Callbacks must
	// not wait for AppKit, actions or controller reconciliation.
	ObserveLauncherEnvironment(context.Context, func(LauncherEnvironment)) error
}

// A token identifies the exact native launcher panel, independently of a
// recycled Wails host pointer. It is never a persisted item or renderer input.
type LauncherPanel interface {
	DockPanel
	LauncherToken() uint64
}

type LauncherPanelHost interface {
	CreateLauncherPanel(unsafe.Pointer, string) (LauncherPanel, error)
}

type LauncherAppTarget struct {
	Process     ProcessIdentity
	Name        string
	BundleID    string
	DisplayUUID string
	PanelToken  uint64
}

type LauncherAppActivator interface {
	// Revalidate the exact running process, bundle, native panel and ordinary
	// display Space on the main thread, and invoke the bounded final Go guard
	// after preparation. No launch-by-path or PID-only activation fallback.
	ActivateLauncherApp(context.Context, LauncherAppTarget, func() error) error
}
