//go:build !darwin

package platform

import (
	"errors"

	"option-tab/internal/domain"
)

func (s *stub) Apps() ([]domain.App, error) {
	return nil, errors.New("application inventory is unsupported on the stub platform")
}

func (s *stub) ActivateApp(domain.AppID) error {
	return errors.New("application activation is unsupported on the stub platform")
}
