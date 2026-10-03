package main

import (
	"errors"
	"strings"
	"testing"
)

func TestDockFeatureRecoveryPreservesOtherActiveErrors(t *testing.T) {
	a, _, _ := shakeAppFixture(t)
	a.setDockInputError(errors.New("icon input denied"))
	a.setDockShakeError(errors.New("shake observation unavailable"))
	state := a.GetDockState()
	if !strings.Contains(state.Error, "icon input denied") || !strings.Contains(state.Error, "shake observation unavailable") {
		t.Fatalf("missing source errors: %q", state.Error)
	}
	a.setDockInputError(nil)
	if got := a.GetDockState().Error; got != "shake observation unavailable" {
		t.Fatalf("icon recovery erased shake failure: %q", got)
	}
	a.setDockDragError(errors.New("drag position refused"))
	a.setDockShakeError(nil)
	if got := a.GetDockState().Error; got != "drag position refused" {
		t.Fatalf("shake recovery erased drag failure: %q", got)
	}
}

func TestDockFeatureErrorsDeduplicateSharedPermissionMessage(t *testing.T) {
	a, _, _ := shakeAppFixture(t)
	err := errors.New("Accessibility permission required")
	a.setDockInputError(err)
	a.setDockShakeError(err)
	if got := a.GetDockState().Error; got != err.Error() {
		t.Fatalf("duplicate permission messages: %q", got)
	}
	a.setDockShakeError(nil)
	if got := a.GetDockState().Error; got != err.Error() {
		t.Fatalf("remaining source error lost: %q", got)
	}
}
