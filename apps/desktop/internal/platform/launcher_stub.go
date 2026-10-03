//go:build !darwin

package platform

import (
	"context"
	"errors"
	"unsafe"
)

func (*stub) ObserveLauncherEnvironment(context.Context, func(LauncherEnvironment)) error {
	return errors.New("launcher environment unsupported")
}

func (*stub) CreateLauncherPanel(unsafe.Pointer, string) (LauncherPanel, error) {
	return nil, errors.New("launcher panel unsupported")
}

func (*stub) ActivateLauncherApp(context.Context, LauncherAppTarget, func() error) error {
	return errors.New("launcher activation unsupported")
}
