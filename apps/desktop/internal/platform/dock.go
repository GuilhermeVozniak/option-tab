package platform

import (
	"context"

	"option-tab/internal/domain"
)

// DockObservation uses global top-left coordinates measured in points.
// Generation changes when the native Dock identity or environment is invalidated.
type DockObservation struct {
	Sequence, Generation uint64
	PointerX, PointerY   float64
	Item                 *DockItem
	Status               string
}

type DockItem struct {
	AppID                 domain.AppID
	BundleID, Path, Title string
	Bounds                domain.Bounds
	ScreenID              domain.ScreenID
	Edge                  string
}

// DockObservationSource delivers coalesced observations until cancellation.
type DockObservationSource interface {
	ObserveDock(context.Context, func(DockObservation)) error
}
