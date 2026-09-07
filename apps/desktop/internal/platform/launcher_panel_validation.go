package platform

import (
	"context"
	"errors"
	"strings"
)

func validateLauncherPanel(ctx context.Context, display string, closed func() bool, native func(string) bool) error {
	if ctx == nil || display == "" || len(display) > 128 || strings.ContainsRune(display, 0) {
		return errors.New("invalid launcher panel validation")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if closed() {
		return ErrDockPanelClosed
	}
	valid := native(display)
	if err := ctx.Err(); err != nil {
		return err
	}
	if closed() {
		return ErrDockPanelClosed
	}
	if !valid {
		return ErrDockPanelHostClosed
	}
	return nil
}
