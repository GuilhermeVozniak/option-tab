//go:build darwin

package platform

/*
#include "darwin_switcher_wheel.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"

	"option-tab/internal/domain"
)

type darwinSwitcherWheelNative struct{}

func (*darwinPlatform) CreateSwitcherWheel(host unsafe.Pointer) (SwitcherWheel, error) {
	if host == nil {
		return nil, ErrDockPanelHostClosed
	}
	token := uint64(C.ot_switcher_wheel_create(host))
	if token == 0 {
		return nil, ErrDockPanelHostClosed
	}
	native := darwinSwitcherWheelNative{}
	return newDarwinWheelPanel(newDockPanel(token, native), native), nil
}

// The shared wheel worker only needs Close; no caller can show the overlay
// through the narrower SwitcherWheel interface.
func (darwinSwitcherWheelNative) Show(uint64, domain.Bounds) error {
	return errors.New("switcher wheel cannot show its host")
}

func (n darwinSwitcherWheelNative) Hide(token uint64) error {
	return n.SetPolicy(token, DockPanelWheelPolicy{})
}

func (darwinSwitcherWheelNative) Close(token uint64) error {
	C.ot_switcher_wheel_close(C.uint64_t(token))
	return nil
}

func (darwinSwitcherWheelNative) SetPolicy(token uint64, policy DockPanelWheelPolicy) error {
	payload, err := marshalPanelWheelPolicy(policy)
	if err != nil {
		return err
	}
	value := C.CString(string(payload))
	defer C.free(unsafe.Pointer(value))
	switch C.ot_switcher_wheel_policy(C.uint64_t(token), value) {
	case -1:
		return ErrDockPanelHostClosed
	case 0:
		return errors.New("switcher wheel policy rejected")
	}
	return nil
}

func (darwinSwitcherWheelNative) Next(token uint64) (DockPanelWheelEvent, int) {
	var value C.OTDockPanelWheelEvent
	status := int(C.ot_switcher_wheel_next(C.uint64_t(token), &value))
	if status != 1 {
		return DockPanelWheelEvent{}, status
	}
	return decodePanelWheelEvent(value), status
}

func (darwinSwitcherWheelNative) Valid(token, session, revision, gesture uint64) bool {
	return C.ot_switcher_wheel_valid(C.uint64_t(token), C.uint64_t(session), C.uint64_t(revision), C.uint64_t(gesture)) != 0
}

func (darwinSwitcherWheelNative) Complete(token, session, revision, gesture uint64) {
	C.ot_switcher_wheel_complete(C.uint64_t(token), C.uint64_t(session), C.uint64_t(revision), C.uint64_t(gesture))
}
