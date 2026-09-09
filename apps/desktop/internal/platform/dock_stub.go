//go:build !darwin

package platform

import (
	"context"
	"errors"
)

func (s *stub) ObserveDock(context.Context, func(DockObservation)) error {
	return errors.New("native Dock observation is unsupported on the stub platform")
}
