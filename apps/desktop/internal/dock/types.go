package dock

import (
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type State struct {
	AdmissionEpoch   uint64
	Session          uint64
	Item             Item
	Windows          []domain.Window
	SelectedWindowID domain.WindowID
	Bounds           domain.Bounds
	Appearance       config.Appearance
	CardSpacingPx    int
	EmptyReason      string
}

type View interface {
	Show(State)
	Update(State)
	Hide(session uint64)
}

type Deps struct {
	Observations platform.DockObservationSource
	Windows      platform.WindowSource
	Apps         platform.ApplicationSource
	AppWindows   platform.ApplicationWindowPresenceSource
	Env          platform.Environment
	View         View
	SelfBundleID string
}

// PointerState is a session-scoped, panel-local pointer sample in logical points.
// Sequence increases within a presentation, including when panel geometry changes.
type PointerState struct {
	Session        uint64  `json:"session"`
	AdmissionEpoch uint64  `json:"admissionEpoch"`
	Sequence       uint64  `json:"sequence"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Inside         bool    `json:"inside"`
}

// PointerView optionally receives native pointer samples after panel publication.
type PointerView interface{ Pointer(PointerState) }

// InputTargetView optionally receives exact native Dock observations for the
// input source cache. The admission epoch lets an outer owner reject a target
// delivered after suspension or reconfiguration.
type InputTargetView interface {
	InputTarget(admissionEpoch uint64, target platform.DockInputTarget)
}
