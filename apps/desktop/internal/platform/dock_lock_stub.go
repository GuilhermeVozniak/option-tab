//go:build !darwin

package platform

import (
	"context"
	"errors"
)

func (s *stub) DockMonitorLockDisplays(context.Context) ([]DockLockDisplay, error) {
	return nil, errors.New("dock monitor locking unsupported")
}

func (s *stub) ObserveDockMonitorLock(context.Context, DockMonitorLockPolicy, func(DockMonitorLockState)) error {
	return errors.New("dock monitor locking unsupported")
}

func (s *stub) PlaceDock(context.Context, DockPlacementRequest) (DockPlacementResult, error) {
	return DockPlacementResult{}, errors.New("dock monitor locking unsupported")
}

func (s *stub) DockPlacementAvailable() bool { return false }
