//go:build darwin

package platform

/*
#include "darwin_launcher_gestures.h"
#include <string.h>
*/
import "C"

import (
	"runtime/cgo"
	"time"
	"unsafe"
)

type darwinLauncherGestureNative struct{}

func (darwinLauncherGestureNative) Policy(token uint64, p LauncherGesturePolicy, current func() bool) error {
	guard := cgo.NewHandle(current)
	defer guard.Delete()
	var v C.OTLauncherGesturePolicy
	v.epoch = C.uint64_t(p.Epoch)
	v.session = C.uint64_t(p.Session)
	v.revision = C.uint64_t(p.Revision)
	v.admission = C.uint64_t(p.Admission)
	v.enabled = C.int(launcherGestureBool(p.Enabled))
	v.scroll = C.int(launcherGestureBool(p.Scroll))
	v.magnify = C.int(launcherGestureBool(p.Magnify))
	v.swipe = C.int(launcherGestureBool(p.Swipe))
	v.w = C.double(p.Bounds.W)
	v.h = C.double(p.Bounds.H)
	copy(unsafe.Slice((*byte)(unsafe.Pointer(&v.display[0])), 128), p.DisplayUUID)
	if C.ot_launcher_gesture_policy_guarded(C.uint64_t(token), v, C.uintptr_t(guard)) == 0 {
		return ErrDockPanelHostClosed
	}
	return nil
}

func gesturePhaseName(p int) string {
	switch p {
	case 1:
		return "began"
	case 2:
		return "changed"
	case 3:
		return "ended"
	case 4:
		return "cancelled"
	}
	return "none"
}

func (darwinLauncherGestureNative) Next(token uint64) (LauncherGestureEvent, int) {
	var e C.OTLauncherGestureEvent
	status := int(C.ot_launcher_gesture_next(C.uint64_t(token), &e))
	if status != 1 {
		return LauncherGestureEvent{}, status
	}
	kind := "scroll"
	switch e.kind {
	case 2:
		kind = "magnify"
	case 3:
		kind = "swipe"
	}
	return LauncherGestureEvent{Epoch: uint64(e.policy.epoch), Session: uint64(e.policy.session), Revision: uint64(e.policy.revision), Admission: uint64(e.policy.admission), DisplayUUID: C.GoString(&e.policy.display[0]), Sequence: uint64(e.sequence), GestureID: uint64(e.gesture), Timestamp: time.Unix(0, int64(float64(e.time)*1e9)), Kind: kind, Phase: gesturePhaseName(int(e.phase)), MomentumPhase: gesturePhaseName(int(e.momentum)), PanelX: float64(e.x), PanelY: float64(e.y), DeltaX: float64(e.dx), DeltaY: float64(e.dy), Magnification: float64(e.magnification), Precise: e.precise != 0, Owned: e.owned != 0}, status
}

func (darwinLauncherGestureNative) Valid(t, e, s, r, a, g uint64) bool {
	return C.ot_launcher_gesture_valid(C.uint64_t(t), C.uint64_t(e), C.uint64_t(s), C.uint64_t(r), C.uint64_t(a), C.uint64_t(g)) != 0
}

func (darwinLauncherGestureNative) Complete(t, e, s, r, a, g uint64) {
	C.ot_launcher_gesture_complete(C.uint64_t(t), C.uint64_t(e), C.uint64_t(s), C.uint64_t(r), C.uint64_t(a), C.uint64_t(g))
}

func (p *darwinLauncherPanel) SetLauncherGesturePolicy(policy LauncherGesturePolicy, emit func(LauncherGestureEvent)) error {
	return p.gestures.set(policy, emit)
}

func (p *darwinLauncherPanel) ValidateLauncherGesture(e, s, r, a, g uint64) bool {
	p.gestures.mu.Lock()
	closed := p.gestures.closed
	policy := p.gestures.policy
	p.gestures.mu.Unlock()
	return !closed && policy.Enabled && p.gestures.native.Valid(p.token, e, s, r, a, g)
}

func (p *darwinLauncherPanel) CompleteLauncherGesture(e, s, r, a, g uint64) {
	p.gestures.native.Complete(p.token, e, s, r, a, g)
}
func (p *darwinLauncherPanel) LauncherGestureDone() <-chan struct{} { return p.gestures.done }
func (p *darwinLauncherPanel) Hide() error {
	p.keyboard.retire(false)
	p.gestures.retire(false)
	return p.dockPanel.Hide()
}

func (p *darwinLauncherPanel) Close() error {
	p.keyboard.retire(true)
	p.gestures.retire(true)
	return p.dockPanel.Close()
}

func launcherGestureBool(v bool) int {
	if v {
		return 1
	}
	return 0
}

//export ot_go_launcher_gesture_policy_current
func ot_go_launcher_gesture_policy_current(handle C.uintptr_t) C.int {
	current := cgo.Handle(handle).Value().(func() bool)
	if current() {
		return 1
	}
	return 0
}
