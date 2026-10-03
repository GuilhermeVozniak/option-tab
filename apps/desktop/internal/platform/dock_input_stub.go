//go:build !darwin

package platform

import (
	"context"
	"errors"
)

func (s *stub) ObserveDockInput(_ context.Context, policy DockInputPolicy, _ <-chan DockInputTarget, _ func(DockInputEvent)) error {
	if !policy.ClickToHide && !policy.ScrollShowHide && !policy.ModifiedRightClick {
		return nil
	}
	return errors.New("native Dock input is unsupported on this platform")
}
func (s *stub) DockInputGestureCurrent(uint64, uint64) bool { return false }
func (s *stub) CompleteDockInputGesture(uint64, uint64)     {}
