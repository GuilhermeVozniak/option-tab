//go:build darwin

package platform

/*
#include "darwin_window_drag.h"
*/
import "C"

import (
	"context"
	"errors"
	"runtime"
	"runtime/cgo"
	"sync/atomic"
	"time"

	"option-tab/internal/domain"
)

func (p *darwinPlatform) ObserveWindowDrags(ctx context.Context, emit func(WindowDragEvent)) error {
	return runNativeWindowDrags(ctx, emit, false)
}

func runNativeWindowDrags(ctx context.Context, emit func(WindowDragEvent), test bool) error {
	return runWindowDragObservation(ctx, emit, test, func() bool { return test || C.ot_window_drag_trusted() != 0 }, time.Second)
}

func runWindowDragObservation(ctx context.Context, emit func(WindowDragEvent), test bool, allowed func() bool, interval time.Duration) error {
	health := func() error {
		if !test && C.ot_window_drag_listening_allowed() == 0 {
			return errors.New("input listening permission unavailable for window drag observation")
		}
		return nil
	}
	return runWindowDragObservationWithHealth(ctx, emit, test, allowed, interval, health)
}

func runWindowDragObservationWithHealth(ctx context.Context, emit func(WindowDragEvent), test bool, allowed func() bool, interval time.Duration, health func() error) error {
	if ctx == nil || emit == nil {
		return errors.New("window drag requires context and callback")
	}
	if ctx.Err() != nil {
		return nil
	}
	if !allowed() {
		return errors.New("accessibility permission unavailable for window drag observation")
	}
	if err := health(); err != nil {
		return err
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	raw := make(chan C.OTWindowDragRaw, 16)
	var generation atomic.Uint64
	done := make(chan error, 1)
	base := time.Now().Add(-time.Duration(float64(C.ot_window_drag_now()) * float64(time.Second)))
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(raw)
		mode := C.int(0)
		if test {
			mode = 1
		}
		owner := C.ot_window_drag_create(mode)
		if owner == nil {
			done <- errors.New("passive window drag tap unavailable")
			return
		}
		generation.Store(uint64(C.ot_window_drag_generation(owner)))
		defer C.ot_window_drag_stop(owner)
		nextHealth := time.Now()
		for child.Err() == nil {
			if !time.Now().Before(nextHealth) {
				if err := health(); err != nil {
					done <- err
					return
				}
				if C.ot_window_drag_healthy(owner) == 0 {
					done <- errors.New("passive window drag tap disabled or invalid after recovery")
					return
				}
				nextHealth = time.Now().Add(interval)
			}
			C.ot_window_drag_pump(owner)
			for len(raw) < cap(raw) {
				var e C.OTWindowDragRaw
				if C.ot_window_drag_pop(owner, &e) == 0 {
					break
				}
				raw <- e
			}
		}
		done <- nil
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() { C.ot_window_drag_clear(C.uint64_t(generation.Load())) }()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var permissionErr error
	for {
		select {
		case <-ticker.C:
			if child.Err() == nil && !allowed() {
				permissionErr = errors.New("accessibility permission revoked during window drag observation")
				cancel()
			}
			continue
		case e, ok := <-raw:
			if !ok {
				err := <-done
				if permissionErr != nil {
					return permissionErr
				}
				return err
			}
			if child.Err() != nil {
				continue
			}
			var out C.OTWindowDragSample
			if C.ot_window_drag_sample(e, &out) == 0 {
				continue
			}
			kinds := []string{"", "candidate", "moved", "up", "cancelled"}
			event := WindowDragEvent{Generation: uint64(e.generation), GestureID: uint64(e.gesture), Sequence: uint64(e.sequence), Timestamp: base.Add(time.Duration(float64(e.at) * float64(time.Second))), Kind: kinds[int(out.raw.kind)], Window: WindowRole{WindowID: domain.WindowID(out.window), AppID: domain.AppID(out.app), Role: C.GoString(&out.role[0]), Subrole: C.GoString(&out.subrole[0])}, PointerX: float64(e.x), PointerY: float64(e.y), WindowX: float64(out.wx), WindowY: float64(out.wy)}
			if child.Err() == nil {
				emit(event)
			}
		}
	}
}

func (p *darwinPlatform) WindowDragGestureCurrent(v WindowDragValidation) bool {
	if v.ObservedAt.IsZero() || v.ObservedAt.After(time.Now()) {
		return false
	}
	at := float64(C.ot_window_drag_now()) - time.Since(v.ObservedAt).Seconds()
	return C.ot_window_drag_current(C.uint64_t(v.Generation), C.uint64_t(v.GestureID), C.uint32_t(v.WindowID), C.int(v.AppID), C.double(v.WindowX), C.double(v.WindowY), C.double(at)) != 0
}

func (p *darwinPlatform) ActionWindowRoles() ([]ActionWindowRole, error) {
	windows, err := p.Windows()
	if err != nil {
		return nil, err
	}
	result := make([]ActionWindowRole, 0, len(windows))
	deadline := time.Now().Add(2 * time.Second)
	for _, w := range windows {
		r := ActionWindowRole{WindowRole: WindowRole{WindowID: w.ID, AppID: w.AppID}, Reason: "classification budget exhausted"}
		if time.Now().Before(deadline) {
			v := C.ot_window_drag_role(C.uint32_t(w.ID), C.int(w.AppID))
			r.SelfApplication = v.self != 0
			r.Role = C.GoString(&v.role[0])
			r.Subrole = C.GoString(&v.subrole[0])
			r.RootConfirmed = v.root != 0
			r.RelationshipsKnown = v.known != 0
			r.Modal = v.modal != 0
			r.HasAttachedSheet = v.sheet != 0
			r.HasModalChild = v.child != 0
			r.ParentWindowID = domain.WindowID(v.parent)
			r.Bounds = domain.Bounds{X: float64(v.x), Y: float64(v.y), W: float64(v.w), H: float64(v.h)}
			r.Reason = C.GoString(&v.reason[0])
		}
		result = append(result, r)
	}
	return result, nil
}

func (p *darwinPlatform) PerformOtherWindowAction(kind string, id domain.WindowID, app domain.AppID) error {
	return performOtherWindowAction(kind, id, app, nil)
}

func (p *darwinPlatform) PerformOtherWindowActionGuarded(kind string, id domain.WindowID, app domain.AppID, guard func() error) error {
	if guard == nil {
		return errors.New("guarded other-window action requires a guard")
	}
	return performOtherWindowAction(kind, id, app, guard)
}

func performOtherWindowAction(kind string, id domain.WindowID, app domain.AppID, guard func() error) error {
	action := 0
	switch kind {
	case "close", "closeOthers":
		action = 1
	case "setMinimized", "minimizeOthers":
		action = 2
	default:
		return errors.New("unsupported other-window action")
	}
	if id == 0 || app <= 0 {
		return errors.New("invalid other-window identity")
	}
	if guard != nil {
		return withOtherWindowGuard(guard, func(token C.uintptr_t) C.int {
			return C.ot_window_drag_perform_guarded(C.int(action), C.uint32_t(id), C.int(app), token)
		})
	}
	if C.ot_window_drag_perform(C.int(action), C.uint32_t(id), C.int(app)) == 0 {
		return errors.New("other-window identity, role, relationship or action refused")
	}
	return nil
}

func nativeWindowDragProbe() map[string]bool {
	bits := uint64(C.ot_window_drag_probe())
	names := []string{"listen-only original event", "pointer-only does not validate", "up immediately retires", "bounded queue invalidates", "correlated movement", "stationary AX rejects", "vertical movement rejects", "stationary threshold retains root for later movement", "discontinuity retires old intent"}
	out := map[string]bool{}
	for i, name := range names {
		out[name] = bits&(1<<i) != 0
	}
	return out
}

func nativeWindowDragActive() bool { return C.ot_window_drag_active() != 0 }

func nativeWindowDragEvidenceEpoch() uint64 { return uint64(C.ot_window_drag_evidence_epoch()) }

type otherWindowGuard struct {
	guard func() error
	err   error
}

//export goWindowDragFinalGuard
func goWindowDragFinalGuard(token C.uintptr_t) C.int {
	state := cgo.Handle(token).Value().(*otherWindowGuard)
	state.err = state.guard()
	if state.err != nil {
		return 0
	}
	return 1
}

func withOtherWindowGuard(guard func() error, call func(C.uintptr_t) C.int) error {
	state := &otherWindowGuard{guard: guard}
	handle := cgo.NewHandle(state)
	defer handle.Delete()
	result := call(C.uintptr_t(handle))
	if state.err != nil {
		return state.err
	}
	if result == 0 {
		return errors.New("other-window identity, role, relationship or action refused")
	}
	return nil
}

func nativeGuardedOtherWindowProbe(guard func() error) error {
	return withOtherWindowGuard(guard, func(token C.uintptr_t) C.int { return C.ot_window_drag_guard_probe(token) })
}
