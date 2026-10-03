//go:build !darwin

package platform

import (
	"context"
	"errors"
)

type unsupportedAutomationServer struct{}

func NewAutomationServer() AutomationServer { return unsupportedAutomationServer{} }
func (unsupportedAutomationServer) Run(context.Context, AutomationHandler) error {
	return errors.New("AppleScript automation is unavailable on this platform")
}
func (unsupportedAutomationServer) Stop()              {}
func (unsupportedAutomationServer) DrainOnMainThread() {}
