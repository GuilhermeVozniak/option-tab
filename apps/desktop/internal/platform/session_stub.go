//go:build !darwin

package platform

import (
	"context"
	"errors"
)

func (s *stub) ObserveSession(context.Context, func(SessionState)) error {
	return errors.New("native session observation is unsupported on the stub platform")
}
