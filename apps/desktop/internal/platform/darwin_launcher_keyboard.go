//go:build darwin

package platform

/*
#include "darwin_launcher_keyboard.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

type darwinLauncherKeyboardNative struct{}

func keyboardNativePolicy(p LauncherKeyboardPolicy) C.OTLauncherKeyboardPolicy {
	var v C.OTLauncherKeyboardPolicy
	v.epoch = C.uint64_t(p.Epoch)
	v.session = C.uint64_t(p.Session)
	v.revision = C.uint64_t(p.Revision)
	v.admission = C.uint64_t(p.Admission)
	if p.Enabled {
		v.enabled = 1
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&v.display[0])), 128), p.DisplayUUID)
	return v
}

func (darwinLauncherKeyboardNative) Set(t uint64, p LauncherKeyboardPolicy, current func() bool) error {
	h := cgo.NewHandle(current)
	defer h.Delete()
	if C.ot_launcher_keyboard_set(C.uint64_t(t), keyboardNativePolicy(p), C.uintptr_t(h)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func (darwinLauncherKeyboardNative) Valid(t uint64, p LauncherKeyboardPolicy) bool {
	return C.ot_launcher_keyboard_valid(C.uint64_t(t), keyboardNativePolicy(p)) != 0
}

//export ot_go_launcher_keyboard_current
func ot_go_launcher_keyboard_current(h C.uintptr_t) C.int {
	if cgo.Handle(h).Value().(func() bool)() {
		return 1
	}
	return 0
}

func (p *darwinLauncherPanel) SetLauncherKeyboardPolicy(v LauncherKeyboardPolicy, guard func() bool) error {
	return p.keyboard.set(v, guard)
}

func (p *darwinLauncherPanel) ValidateLauncherKeyboard(e, s, r, a uint64) bool {
	p.keyboard.mu.Lock()
	v := p.keyboard.policy
	p.keyboard.mu.Unlock()
	if v.Epoch != e || v.Session != s || v.Revision != r || v.Admission != a {
		return false
	}
	return p.keyboard.valid(v)
}
