//go:build darwin

package platform

/*
#include <stdlib.h>
#include <string.h>
#include "darwin_dock_input.h"
*/
import "C"

import (
	"context"
	"errors"
	"runtime"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

type inputValidation struct {
	generation, gesture uint64
	valid               bool
	done                chan struct{}
}

func (p *darwinPlatform) DockInputGestureCurrent(generation, gesture uint64) bool {
	return C.ot_dock_input_current(C.uint64_t(generation), C.uint64_t(gesture)) != 0
}

func (p *darwinPlatform) ObserveDockInput(ctx context.Context, policy DockInputPolicy, targets <-chan DockInputTarget, emit func(DockInputEvent)) error {
	return runNativeDockInput(ctx, policy, targets, emit, false)
}

func runNativeDockInput(ctx context.Context, policy DockInputPolicy, targets <-chan DockInputTarget, emit func(DockInputEvent), test bool) error {
	return runNativeDockInputHealth(ctx, policy, targets, emit, test, nil)
}

func runNativeDockInputHealth(ctx context.Context, policy DockInputPolicy, targets <-chan DockInputTarget, emit func(DockInputEvent), test bool, health func() int) error {
	if ctx == nil || targets == nil || emit == nil {
		return errors.New("dock input requires context, targets, and callback")
	}
	if !policy.ClickToHide && !policy.ScrollShowHide && !policy.ModifiedRightClick {
		return nil
	}
	if ctx.Err() != nil {
		return nil
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	raw := make(chan C.OTDockInputRaw, 16)
	acknowledgements := make(chan inputValidation, 1)
	started := make(chan error, 1)
	terminal := make(chan error, 1)
	ownerDone := make(chan struct{})
	workerDone := make(chan struct{})
	base := time.Now().Add(-time.Duration(float64(C.ot_dock_input_now()) * float64(time.Second)))
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(ownerDone)
		defer close(raw)
		owner := C.ot_dock_input_create(inputBool(policy.ClickToHide), inputBool(policy.ScrollShowHide), inputBool(policy.ModifiedRightClick), inputBool(test))
		if owner == nil {
			started <- errors.New("dock input tap unavailable: permission denied or another source is running")
			return
		}
		defer C.ot_dock_input_stop(owner)
		started <- nil
		for child.Err() == nil {
			select {
			case target, ok := <-targets:
				if !ok {
					cancel()
					return
				}
				C.ot_dock_input_target(owner, nativeInputTarget(target, time.Now()))
			case ack := <-acknowledgements:
				C.ot_dock_input_ack(owner, C.uint64_t(ack.generation), C.uint64_t(ack.gesture), inputBool(ack.valid))
				close(ack.done)
			default:
			}
			C.ot_dock_input_pump(owner)
			status := int(C.ot_dock_input_health(owner))
			if test && health != nil {
				status = health()
			}
			if status != 0 {
				terminal <- iconInputHealthError(status)
				cancel()
				return
			}
			for len(raw) < cap(raw) {
				var event C.OTDockInputRaw
				if C.ot_dock_input_pop(owner, &event) == 0 {
					break
				}
				raw <- event
			}
		}
	}()
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(workerDone)
		for event := range raw {
			if child.Err() != nil {
				C.ot_dock_input_release(event.event)
				continue
			}
			mapped := mapNativeInput(event, base)
			if event.cancelled == 0 {
				if C.ot_dock_input_completed(event.target.generation, event.gesture) != 0 {
					C.ot_dock_input_release(event.event)
					continue
				}
				valid := C.ot_dock_input_owned(event.target.generation, event.gesture) != 0
				if valid {
					if event.validated != 0 {
						valid = C.ot_dock_input_validate_app(event) != 0
					} else {
						valid = float64(event.target.expiry) >= float64(C.ot_dock_input_now()) && C.ot_dock_input_validate(event) != 0
					}
				}
				ack := inputValidation{generation: mapped.Generation, gesture: mapped.GestureID, valid: valid, done: make(chan struct{})}
				select {
				case acknowledgements <- ack:
				case <-child.Done():
					C.ot_dock_input_release(event.event)
					continue
				}
				select {
				case <-ack.done:
				case <-child.Done():
					C.ot_dock_input_release(event.event)
					continue
				}
				mapped.Owned = valid && C.ot_dock_input_current(event.target.generation, event.gesture) != 0
				if !mapped.Owned {
					mapped.Kind = DockInputCancelled
					mapped.Reason = "native identity or gesture validation failed"
				}
			}
			C.ot_dock_input_release(event.event)
			if child.Err() == nil {
				emit(mapped)
			}
		}
	}()
	err := <-started
	if err != nil {
		cancel()
	}
	<-ownerDone
	cancel()
	<-workerDone
	if err == nil {
		select {
		case err = <-terminal:
		default:
		}
	}
	return err
}

func inputBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func nativeInputTarget(target DockInputTarget, now time.Time) C.OTDockInputTarget {
	out := C.OTDockInputTarget{generation: C.uint64_t(target.Generation), dock_pid: C.int(target.DockPID)}
	if target.Item == nil || target.ObservedAt.IsZero() || target.ObservedAt.After(now) {
		return out
	}
	age := now.Sub(target.ObservedAt)
	if age > 150*time.Millisecond {
		return out
	}
	item := target.Item
	if !validDockBounds(item.Bounds) || len(item.Path) >= 1024 || len(item.BundleID) >= 256 {
		return out
	}
	out.app_pid = C.int(item.AppID)
	out.expiry = C.double(float64(C.ot_dock_input_now()) + (150*time.Millisecond - age).Seconds())
	out.x = C.double(item.Bounds.X)
	out.y = C.double(item.Bounds.Y)
	out.w = C.double(item.Bounds.W)
	out.h = C.double(item.Bounds.H)
	out.screen = C.uint32_t(item.ScreenID)
	switch item.Edge {
	case "left":
		out.edge = 1
	case "right":
		out.edge = 2
	default:
		out.edge = 0
	}
	path, bundle := C.CString(item.Path), C.CString(item.BundleID)
	defer C.free(unsafe.Pointer(path))
	defer C.free(unsafe.Pointer(bundle))
	C.strlcpy(&out.path[0], path, C.size_t(len(out.path)))
	C.strlcpy(&out.bundle[0], bundle, C.size_t(len(out.bundle)))
	return out
}

func mapNativeInput(raw C.OTDockInputRaw, base time.Time) DockInputEvent {
	t := raw.target
	edge := "bottom"
	switch t.edge {
	case 1:
		edge = "left"
	case 2:
		edge = "right"
	}
	out := DockInputEvent{Generation: uint64(t.generation), GestureID: uint64(raw.gesture), Sequence: uint64(raw.sequence), DockPID: int(t.dock_pid), Timestamp: base.Add(time.Duration(float64(raw.at) * float64(time.Second))), Item: DockItem{AppID: domain.AppID(t.app_pid), Path: C.GoString(&t.path[0]), BundleID: C.GoString(&t.bundle[0]), Bounds: domain.Bounds{X: float64(t.x), Y: float64(t.y), W: float64(t.w), H: float64(t.h)}, ScreenID: domain.ScreenID(t.screen), Edge: edge}}
	if raw.cancelled != 0 {
		out.Kind = DockInputCancelled
		out.Reason = "native ownership cancelled"
		return out
	}
	if raw.terminal != 0 {
		out.Kind = DockInputScroll
		out.Phase = "ended"
		out.MomentumPhase = "none"
		out.PointerX = out.Item.Bounds.X + out.Item.Bounds.W/2
		out.PointerY = out.Item.Bounds.Y + out.Item.Bounds.H/2
		return out
	}
	switch raw._type {
	case 1:
		out.Kind = DockInputLeftDown
	case 2:
		out.Kind = DockInputLeftUp
	case 3:
		out.Kind = DockInputRightDown
	case 4:
		out.Kind = DockInputRightUp
	case 22:
		out.Kind = DockInputScroll
	default:
		out.Kind = DockInputMove
	}
	f := C.ot_dock_input_fields(raw.event)
	out.PointerX = float64(f.x)
	out.PointerY = float64(f.y)
	out.DeltaX = float64(f.dx)
	out.DeltaY = float64(f.dy)
	out.Button = int(f.button)
	out.Modifiers = hotkey.ModSet(f.mods)
	out.Precise = f.precise != 0
	out.DirectionInverted = f.inverted != 0
	phases := []string{"none", "began", "changed", "ended", "cancelled"}
	out.Phase = phases[int(f.phase)]
	out.MomentumPhase = phases[int(f.momentum)]
	return out
}

func nativeDockInputOwnershipProbe() map[string]bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	results := map[string]bool{}
	newOwner := func() unsafe.Pointer {
		p := C.ot_dock_input_create(1, 1, 1, 1)
		target := DockInputTarget{Generation: 77, DockPID: 100, ObservedAt: time.Now(), Item: &DockItem{AppID: 200, Path: "/Fixture.app", BundleID: "fixture.app", ScreenID: 1, Bounds: domain.Bounds{X: 100, Y: 100, W: 50, H: 50}}}
		C.ot_dock_input_target(p, nativeInputTarget(target, time.Now()))
		return p
	}
	p := newOwner()
	results["synthetic pass-through"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 1, 0, 0) == 0
	results["miss pass-through"] = C.ot_dock_input_test_event(p, 1, 20, 20, 0, 0, 0, 0) == 0
	results["qualified down suppressed"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 1
	results["pending validation up forwarded"] = C.ot_dock_input_test_event(p, 2, 120, 120, 0, 0, 0, 0) == 0
	results["pending click original down replayed once"] = C.ot_dock_input_test_replays(p) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	var raw C.OTDockInputRaw
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	results["validated up suppressed"] = C.ot_dock_input_test_event(p, 2, 120, 120, 0, 0, 0, 0) == 1
	results["completed click token valid"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) != 0
	results["completed click no replay"] = C.ot_dock_input_test_replays(p) == 0
	C.ot_dock_input_stop(p)
	results["teardown invalidates token"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) == 0
	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	results["abandoned drag forwarded"] = C.ot_dock_input_test_event(p, 6, 130, 120, 0, 0, 0, 0) == 0
	results["abandoned drag replays original down"] = C.ot_dock_input_test_replays(p) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	results["unsupported right modifiers pass"] = C.ot_dock_input_test_event(p, 3, 120, 120, 12, 0, 0, 0) == 0
	results["command right owned"] = C.ot_dock_input_test_event(p, 3, 120, 120, 8, 0, 0, 0) == 1
	C.ot_dock_input_target(p, C.OTDockInputTarget{})
	results["cache invalidation replays down"] = C.ot_dock_input_test_replays(p) == 1
	C.ot_dock_input_stop(p)

	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_test_event(p, -2, 120, 120, 0, 0, 0, 0)
	results["timeout replays original down"] = C.ot_dock_input_test_replays(p) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	target := nativeInputTarget(DockInputTarget{Generation: 77, DockPID: 100, ObservedAt: time.Now().Add(-time.Second), Item: &DockItem{AppID: 200, Path: "/Fixture.app", BundleID: "fixture.app", ScreenID: 1, Bounds: domain.Bounds{X: 100, Y: 100, W: 50, H: 50}}}, time.Now())
	C.ot_dock_input_target(p, target)
	results["expired target passes through"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 0
	C.ot_dock_input_stop(p)
	p = newOwner()
	for i := 0; i < 63; i++ {
		phase := C.int(2)
		if i == 0 {
			phase = 1
		}
		C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, phase, 0)
	}
	results["queue overload forwards new down"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 0
	results["queue overload never duplicates unsuppressed down"] = C.ot_dock_input_test_replays(p) == 0
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 1, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_release(raw.event)
	C.ot_dock_input_test_event(p, 22, 120, 120, 8, 0, 2, 0)
	results["scroll modifier change cancels token"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) == 0
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	C.ot_dock_input_test_event(p, 2, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_target(p, C.OTDockInputTarget{generation: 77, dock_pid: 100})
	results["completed click survives ordinary pointer exit"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) != 0
	target = nativeInputTarget(DockInputTarget{Generation: 77, DockPID: 100, ObservedAt: time.Now(), Item: &DockItem{AppID: 200, Path: "/Fixture.app", BundleID: "fixture.app", ScreenID: 1, Bounds: domain.Bounds{X: 100, Y: 100, W: 50, H: 50}}}, time.Now())
	C.ot_dock_input_target(p, target)
	results["new click passes while action pending"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 0
	C.ot_dock_input_complete(raw.target.generation, raw.gesture)
	results["acknowledged action token retired"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) == 0
	results["next click admitted after acknowledgement"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 1, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	C.ot_dock_input_complete(raw.target.generation, raw.gesture)
	results["acknowledged scroll remainder stays owned"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 2, 0) == 1
	results["acknowledged scroll terminal stays owned"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 4, 0) == 1
	results["acknowledged momentum begin stays owned"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 0, 1) == 1
	results["acknowledged momentum changed stays owned"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 0, 2) == 1
	results["acknowledged momentum end stays owned"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 0, 3) == 1
	results["momentum completion never restores action token"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) == 0

	results["next phased scroll admitted"] = C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 1, 0) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 22, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	C.ot_dock_input_test_idle(p)
	var ended C.OTDockInputRaw
	results["legacy idle produces terminal"] = C.ot_dock_input_pop(p, &ended) != 0 && ended.terminal != 0
	results["legacy idle preserves pending action token"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) != 0
	C.ot_dock_input_release(ended.event)
	C.ot_dock_input_complete(raw.target.generation, raw.gesture)
	results["legacy no-action acknowledgement allows click"] = C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0) == 1
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	results["captured process lifetime validates"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) != 0
	C.ot_dock_input_test_changed_process(p)
	results["changed process start rejects token"] = C.ot_dock_input_current(raw.target.generation, raw.gesture) == 0
	C.ot_dock_input_stop(p)
	p = newOwner()
	C.ot_dock_input_test_event(p, 1, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_pop(p, &raw)
	C.ot_dock_input_ack(p, raw.target.generation, raw.gesture, 1)
	C.ot_dock_input_release(raw.event)
	for i := 0; i < 64; i++ {
		C.ot_dock_input_test_event(p, 6, 120, 120, 0, 0, 0, 0)
	}
	results["full queue forwards held up"] = C.ot_dock_input_test_event(p, 2, 120, 120, 0, 0, 0, 0) == 0
	results["full queue replays original held down once"] = C.ot_dock_input_test_replays(p) == 1
	C.ot_dock_input_stop(p)
	return results
}

var (
	_ DockInputSource           = (*darwinPlatform)(nil)
	_ DockInputGestureValidator = (*darwinPlatform)(nil)
)

func nativeDockInputTestAlive() int { return int(C.ot_dock_input_test_alive()) }
func nativeDockInputCounters() (uint64, uint64) {
	return uint64(C.ot_dock_input_installations()), uint64(C.ot_dock_input_synthetic_passes())
}

func (p *darwinPlatform) CompleteDockInputGesture(generation, gestureID uint64) {
	C.ot_dock_input_complete(C.uint64_t(generation), C.uint64_t(gestureID))
}

var _ DockInputGestureAcknowledger = (*darwinPlatform)(nil)

func nativeIconHealthProbe(mask int, timeout bool) (int, int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	owner := C.ot_dock_input_create(1, 0, 0, 1)
	defer C.ot_dock_input_stop(owner)
	target := nativeInputTarget(DockInputTarget{Generation: 77, DockPID: 100, ObservedAt: time.Now(), Item: &DockItem{AppID: 200, Path: "/Fixture.app", BundleID: "fixture.app", ScreenID: 1, Bounds: domain.Bounds{X: 100, Y: 100, W: 50, H: 50}}}, time.Now())
	C.ot_dock_input_target(owner, target)
	C.ot_dock_input_test_event(owner, 1, 120, 120, 0, 0, 0, 0)
	C.ot_dock_input_test_health(owner, C.int(mask))
	if timeout {
		C.ot_dock_input_test_event(owner, -2, 0, 0, 0, 0, 0, 0)
	}
	status := C.ot_dock_input_health(owner)
	return int(status), int(C.ot_dock_input_test_replays(owner))
}

func iconInputHealthError(code int) error {
	switch code {
	case 1:
		return errors.New("accessibility permission revoked during dock input observation")
	case 2:
		return errors.New("event listening permission revoked during dock input observation")
	case 3:
		return errors.New("dock input event tap port became invalid")
	default:
		return errors.New("dock input event tap recovery failed")
	}
}
