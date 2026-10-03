package platform

import (
	"context"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

type DockLockDisplay struct {
	UUID     string          `json:"uuid"`
	ID       domain.ScreenID `json:"id"`
	Name     string          `json:"name"`
	Bounds   domain.Bounds   `json:"bounds"`
	Scale    float64         `json:"scale"`
	Main     bool            `json:"main"`
	Mirrored bool            `json:"mirrored"`
}

// Policy is immutable for one native observation lifetime. The controller owns
// session/revision; native topology or Dock identity changes advance generation.
type DockMonitorLockPolicy struct {
	Session, Revision uint64
	Target            string
	DisplayUUID       string
	Bypass            hotkey.ModSet
}

type DockMonitorLockState struct {
	Session            uint64            `json:"session"`
	Revision           uint64            `json:"revision"`
	Generation         uint64            `json:"generation"`
	Sequence           uint64            `json:"sequence"`
	ObservedAtMs       int64             `json:"observedAtMs"`
	Status             string            `json:"status"`
	Reason             string            `json:"reason"`
	TargetUUID         string            `json:"targetUUID"`
	ActualUUID         string            `json:"actualUUID"`
	Edge               string            `json:"edge"`
	Displays           []DockLockDisplay `json:"displays"`
	PlacementAvailable bool              `json:"placementAvailable"`
}

type DockPlacementRequest struct {
	Session, Revision, Generation, RequestID uint64
}

type DockPlacementResult struct {
	RequestID      uint64 `json:"requestId"`
	Status         string `json:"status"`
	Reason         string `json:"reason"`
	ActualUUID     string `json:"actualUUID"`
	Verified       bool   `json:"verified"`
	CursorRestored bool   `json:"cursorRestored"`
}

// Observe joins native resources and any placement operation on cancellation;
// no callbacks escape its return. Inventory does not install an input filter.
// PlaceDock targets the exact active observation owner and is cancellable.
type DockMonitorLockSource interface {
	DockMonitorLockDisplays(context.Context) ([]DockLockDisplay, error)
	ObserveDockMonitorLock(context.Context, DockMonitorLockPolicy, func(DockMonitorLockState)) error
	PlaceDock(context.Context, DockPlacementRequest) (DockPlacementResult, error)
}

// Placement is separately advertised only after its cursor transport is proven
// on the native backend. Protection can operate with manual Dock placement.
type DockPlacementAvailability interface {
	DockPlacementAvailable() bool
}
