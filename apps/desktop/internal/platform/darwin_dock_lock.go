//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework ColorSync
#include <stdlib.h>
#include "darwin_dock_lock.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
)

type (
	dockLockRaw struct {
		Generation uint64            `json:"generation"`
		PID        int               `json:"pid"`
		Edge       string            `json:"edge"`
		Reason     string            `json:"reason"`
		Container  domain.Bounds     `json:"container"`
		Displays   []DockLockDisplay `json:"displays"`
	}
	lockPlaceJob struct {
		ctx     context.Context
		request DockPlacementRequest
		reply   chan lockPlaceReply
	}
	lockPlaceReply struct {
		result DockPlacementResult
		err    error
	}
	dockLockRuntime struct {
		policy DockMonitorLockPolicy
		jobs   chan lockPlaceJob
		done   chan struct{}
	}
)

var dockLockActive struct {
	sync.Mutex
	owner *dockLockRuntime
}

func readLockJSON(ptr *C.char, out any) error {
	if ptr == nil {
		return errors.New("native Dock lock snapshot unavailable")
	}
	defer C.free(unsafe.Pointer(ptr))
	return json.Unmarshal([]byte(C.GoString(ptr)), out)
}

func (p *darwinPlatform) DockMonitorLockDisplays(ctx context.Context) ([]DockLockDisplay, error) {
	if ctx == nil {
		return nil, errors.New("missing context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var displays []DockLockDisplay
	err := readLockJSON(C.ot_lock_displays(), &displays)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return displays, err
}

func lockBypass(mod hotkey.ModSet) uint64 {
	switch {
	case mod == hotkey.ModSet(0).With(hotkey.ModOption):
		return 1 << 19
	case mod == hotkey.ModSet(0).With(hotkey.ModCommand):
		return 1 << 20
	case mod == hotkey.ModSet(0).With(hotkey.ModControl):
		return 1 << 18
	case mod == hotkey.ModSet(0).With(hotkey.ModShift):
		return 1 << 17
	}
	return 0
}

// DockPlacementAvailable remains false until real Dock relocation is proven.
func (p *darwinPlatform) DockPlacementAvailable() bool { return false }

func (p *darwinPlatform) PlaceDock(_ context.Context, request DockPlacementRequest) (DockPlacementResult, error) {
	const reason = "automatic Dock placement unavailable: native relocation is unverified; move the Dock manually"
	return DockPlacementResult{RequestID: request.RequestID, Status: "unavailable", Reason: reason}, errors.New(reason)
}

// placeDockOnOwner exercises experimental transport only from isolated native tests.
func placeDockOnOwner(ctx context.Context, request DockPlacementRequest) (DockPlacementResult, error) {
	if ctx == nil {
		return DockPlacementResult{}, errors.New("missing context")
	}
	dockLockActive.Lock()
	owner := dockLockActive.owner
	dockLockActive.Unlock()
	if owner == nil || request.Session != owner.policy.Session || request.Revision != owner.policy.Revision || request.RequestID == 0 {
		return DockPlacementResult{}, errors.New("dock lock placement owner retired")
	}
	job := lockPlaceJob{ctx: ctx, request: request, reply: make(chan lockPlaceReply, 1)}
	select {
	case owner.jobs <- job:
	case <-owner.done:
		return DockPlacementResult{}, errors.New("dock lock owner retired")
	case <-ctx.Done():
		return DockPlacementResult{}, ctx.Err()
	default:
		return DockPlacementResult{}, errors.New("dock placement already pending")
	}
	select {
	case reply := <-job.reply:
		return reply.result, reply.err
	case <-owner.done:
		select {
		case reply := <-job.reply:
			return reply.result, reply.err
		default:
			return DockPlacementResult{}, errors.New("dock lock owner retired")
		}
	}
}

func (p *darwinPlatform) ObserveDockMonitorLock(ctx context.Context, policy DockMonitorLockPolicy, emit func(DockMonitorLockState)) error {
	return runDockMonitorLock(ctx, policy, emit, false, nil)
}

func runDockMonitorLock(ctx context.Context, policy DockMonitorLockPolicy, emit func(DockMonitorLockState), test bool, snapshot func(int) (dockLockRaw, error)) error {
	if ctx == nil || emit == nil || policy.Session == 0 || policy.Revision == 0 || lockBypass(policy.Bypass) == 0 || (policy.Target != "main" && policy.Target != "display") {
		return errors.New("invalid Dock lock policy")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	owner := &dockLockRuntime{policy: policy, jobs: make(chan lockPlaceJob, 1), done: make(chan struct{})}
	dockLockActive.Lock()
	if dockLockActive.owner != nil {
		dockLockActive.Unlock()
		return errors.New("another Dock lock owner is active")
	}
	dockLockActive.owner = owner
	dockLockActive.Unlock()
	defer func() {
		dockLockActive.Lock()
		if dockLockActive.owner == owner {
			dockLockActive.owner = nil
		}
		dockLockActive.Unlock()
		close(owner.done)
	}()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	native := C.ot_lock_create(inputBool(test))
	if native == nil {
		return errors.New("dock lock allocation failed")
	}
	defer C.ot_lock_destroy(native)
	cancellationStop, cancellationDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(cancellationDone)
		select {
		case <-child.Done():
			C.ot_lock_retire(native)
		case <-cancellationStop:
		}
	}()
	defer func() { close(cancellationStop); <-cancellationDone }()
	ready := make(chan error, 1)
	tapDone := make(chan struct{})
	tapStop := make(chan struct{})
	tapError := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(tapDone)
		defer C.ot_lock_stop(native)
		if child.Err() != nil {
			ready <- child.Err()
			return
		}
		code := C.ot_lock_start(native)
		if code != 0 {
			ready <- fmt.Errorf("dock movement tap unavailable (%d)", code)
			return
		}
		ready <- nil
		healthAt := time.Now()
		for {
			select {
			case <-tapStop:
				return
			default:
			}
			C.ot_lock_pump(native)
			if child.Err() == nil && time.Since(healthAt) >= 100*time.Millisecond {
				healthAt = time.Now()
				if C.ot_lock_health(native) != 0 {
					tapError <- errors.New("dock movement tap permission or health unavailable")
					cancel()
				}
			}
		}
	}()
	if err := <-ready; err != nil {
		<-tapDone
		return err
	}
	defer func() { cancel(); close(tapStop); <-tapDone }()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	readRaw := func() (dockLockRaw, error) {
		if test && snapshot != nil {
			return snapshot(int(C.ot_lock_test_moves(native)))
		}
		var raw dockLockRaw
		err := readLockJSON(C.ot_lock_snapshot(native), &raw)
		return raw, err
	}
	bypass := func() bool { return !test && C.ot_lock_bypass(C.uint64_t(lockBypass(policy.Bypass))) != 0 }
	buttons := func() bool { return !test && C.ot_lock_buttons() != 0 }
	var state DockMonitorLockState
	var edges map[string][]dockLockSegment
	var active *lockPlacement
	var sequence uint64
	var fingerprint string
	var emitted *DockMonitorLockState
	poll := func() {
		at := time.Now()
		stamp := float64(C.ot_lock_now())
		raw, err := readRaw()
		if raw.Edge == "" && raw.Reason == "" {
			raw.Edge = dockLockInferredEdge(raw.Displays, raw.Container)
		}
		nextFingerprint := dockLockFingerprint(raw)
		if fingerprint != "" && fingerprint != nextFingerprint {
			raw.Generation = uint64(C.ot_lock_advance_environment())
		}
		fingerprint = nextFingerprint
		sequence++
		state = DockMonitorLockState{Session: policy.Session, Revision: policy.Revision, Sequence: sequence, Generation: raw.Generation, ObservedAtMs: at.UnixMilli(), Displays: raw.Displays, Edge: raw.Edge, Status: "unavailable", Reason: raw.Reason}
		state.TargetUUID = policy.DisplayUUID
		if policy.Target == "main" {
			state.TargetUUID = ""
			for _, d := range raw.Displays {
				if d.Main {
					if state.TargetUUID != "" {
						state.TargetUUID = ""
						break
					}
					state.TargetUUID = d.UUID
				}
			}
		}
		exists := false
		for _, d := range raw.Displays {
			if d.UUID == state.TargetUUID {
				exists = true
			}
		}
		var valid bool
		edges, valid = dockLockEdges(raw.Displays, raw.Edge)
		state.ActualUUID = dockLockActual(raw.Displays, raw.Edge, raw.Container)
		switch {
		case err != nil:
			state.Reason = err.Error()
		case raw.Reason != "":
			state.Reason = raw.Reason
		case len(raw.Displays) == 0:
			state.Reason = "display inventory unavailable"
		case !exists:
			state.Status = "disconnected"
			state.Reason = "selected display is disconnected"
		case !valid:
			state.Reason = "display or Dock edge geometry is ambiguous"
		case len(edges[state.TargetUUID]) == 0:
			state.Status = "unreachable"
			state.Reason = "selected display has no exposed Dock edge"
		case raw.Reason != "" || state.ActualUUID == "":
			state.Reason = "Dock container location is unverified"
		case bypass():
			state.Status = "bypassed"
			state.Reason = "bypass modifier held"
		case state.ActualUUID != state.TargetUUID:
			state.Status = "awaitingPlacement"
			state.Reason = "move Dock to selected display to activate protection"
		default:
			state.Status = "protected"
			state.Reason = ""
		}
		segments := []C.OTLockSegment{}
		if state.Status == "protected" && active == nil {
			for uuid, list := range edges {
				if uuid == state.TargetUUID {
					continue
				}
				for _, s := range list {
					segments = append(segments, C.OTLockSegment{x: C.double(s.X), y: C.double(s.Y), w: C.double(s.W), h: C.double(s.H), dx: C.double(s.DX), dy: C.double(s.DY)})
				}
			}
		}
		if child.Err() != nil {
			C.ot_lock_disable(native)
			return
		}
		var ptr *C.OTLockSegment
		if len(segments) > 0 {
			ptr = &segments[0]
		}
		C.ot_lock_publish(native, C.uint64_t(state.Generation), C.double(stamp+.15), C.uint64_t(lockBypass(policy.Bypass)), ptr, C.int(len(segments)))
		if active != nil {
			state.Status = "placing"
			state.Reason = ""
		}
		if child.Err() == nil && (emitted == nil || dockLockStateChanged(*emitted, state)) {
			snapshot := state
			snapshot.Displays = append([]DockLockDisplay(nil), state.Displays...)
			emitted = &snapshot
			delivery := state
			delivery.Displays = append([]DockLockDisplay(nil), state.Displays...)
			emit(delivery)
		}
	}
	finish := func(reason string) {
		if active == nil {
			return
		}
		result := DockPlacementResult{RequestID: active.job.request.RequestID, Status: "unavailable", Reason: reason, ActualUUID: state.ActualUUID}
		stillOwned := uint64(C.ot_lock_physical(native)) == active.physical && uint64(C.ot_lock_generation()) == active.job.request.Generation
		if stillOwned && !active.lastMove.IsZero() {
			result.CursorRestored = C.ot_lock_restore(native, C.uint64_t(active.job.request.Generation), C.uint64_t(active.physical), C.uint64_t(active.job.request.RequestID), C.double(active.savedX), C.double(active.savedY)) != 0
		}
		// Fresh actual-container read only after the restoration has been delivered.
		if result.CursorRestored && reason == "" && child.Err() == nil && active.job.ctx.Err() == nil {
			C.ot_lock_disable(native)
			time.Sleep(80 * time.Millisecond)
			observed, readErr := readRaw()
			if readErr == nil && observed.Generation == active.job.request.Generation && uint64(C.ot_lock_physical(native)) == active.physical {
				if observed.Edge == "" && observed.Reason == "" {
					observed.Edge = dockLockInferredEdge(observed.Displays, observed.Container)
				}
				if dockLockFingerprint(observed) != fingerprint {
					fingerprint = dockLockFingerprint(observed)
					C.ot_lock_advance_environment()
				}
				result.ActualUUID = dockLockActual(observed.Displays, observed.Edge, observed.Container)
				result.Verified = result.ActualUUID == state.TargetUUID && result.ActualUUID != "" && child.Err() == nil && active.job.ctx.Err() == nil && uint64(C.ot_lock_generation()) == active.job.request.Generation
			}
		}
		if result.Verified {
			result.Status = "protected"
		} else if result.Reason == "" {
			result.Reason = "Dock placement was not verified after cursor restoration"
		}
		var err error
		if !result.Verified {
			err = errors.New(result.Reason)
		}
		close(active.watchStop)
		<-active.watchDone
		C.ot_lock_cancel_placement(native, C.uint64_t(active.job.request.RequestID))
		active.job.reply <- lockPlaceReply{result, err}
		active = nil
	}
	defer func() {
		finish("Dock lock owner cancelled")
		for {
			select {
			case job := <-owner.jobs:
				job.reply <- lockPlaceReply{err: errors.New("dock lock owner retired")}
			default:
				return
			}
		}
	}()
	poll()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	nextPoll := time.Now().Add(75 * time.Millisecond)
	for {
		select {
		case <-child.Done():
			select {
			case err := <-tapError:
				return err
			default:
				return child.Err()
			}
		case job := <-owner.jobs:
			if active != nil {
				job.reply <- lockPlaceReply{err: errors.New("dock placement already active")}
				continue
			}
			physical := uint64(C.ot_lock_physical(native))
			poll() // Explicit placement admission never relies on the cached observation.
			if child.Err() != nil || job.ctx.Err() != nil || physical != uint64(C.ot_lock_physical(native)) || C.ot_lock_health(native) != 0 || job.request.Generation != state.Generation || uint64(C.ot_lock_generation()) != state.Generation || (state.Status != "awaitingPlacement" && state.Status != "protected") || len(edges[state.TargetUUID]) == 0 || buttons() {
				job.reply <- lockPlaceReply{err: errors.New("dock placement admission unavailable")}
				continue
			}
			if state.ActualUUID == state.TargetUUID {
				job.reply <- lockPlaceReply{result: DockPlacementResult{RequestID: job.request.RequestID, Status: "protected", ActualUUID: state.ActualUUID, Verified: true}}
				continue
			}
			segment := edges[state.TargetUUID][0]
			for _, s := range edges[state.TargetUUID] {
				if s.W*s.H > segment.W*segment.H {
					segment = s
				}
			}
			var x, y C.double
			if test {
				x, y = 100, 200
			} else {
				C.ot_lock_pointer(&x, &y)
			}
			if !dockLockFinite(float64(x), float64(y)) || physical != uint64(C.ot_lock_physical(native)) {
				job.reply <- lockPlaceReply{err: errors.New("cursor unavailable")}
				continue
			}
			active = &lockPlacement{job: job, savedX: float64(x), savedY: float64(y), physical: physical, watchStop: make(chan struct{}), watchDone: make(chan struct{}), started: time.Now(), segment: segment}
			C.ot_lock_disable(native)
			C.ot_lock_begin_placement(native, C.uint64_t(job.request.RequestID))
			watching := active
			go func() {
				defer close(watching.watchDone)
				select {
				case <-watching.watchStop:
					return
				case <-watching.job.ctx.Done():
				case <-child.Done():
				}
				C.ot_lock_cancel_placement(native, C.uint64_t(watching.job.request.RequestID))
			}()
		case <-ticker.C:
			if time.Now().After(nextPoll) {
				poll()
				nextPoll = time.Now().Add(75 * time.Millisecond)
			}
			if active == nil {
				continue
			}
			reason := ""
			switch {
			case active.job.ctx.Err() != nil:
				reason = "Dock placement cancelled"
			case uint64(C.ot_lock_physical(native)) != active.physical:
				reason = "physical input took over cursor"
			case uint64(C.ot_lock_generation()) != active.job.request.Generation:
				reason = "display or Dock identity changed"
			case bypass():
				reason = "bypass modifier held"
			case time.Since(active.started) > 2*time.Second:
				reason = "Dock placement timed out"
			}
			if reason != "" {
				finish(reason)
				continue
			}
			if state.ActualUUID == state.TargetUUID {
				finish("")
				continue
			}
			if time.Since(active.lastMove) < 100*time.Millisecond {
				continue
			}
			x, y, dx, dy := dockLockPlacementPoint(active.segment, active.moves)
			if active.job.ctx.Err() != nil || C.ot_lock_move(native, C.uint64_t(active.job.request.Generation), C.uint64_t(active.physical), C.uint64_t(active.job.request.RequestID), C.double(x), C.double(y), C.double(dx), C.double(dy)) == 0 {
				finish("cursor ownership or movement refused")
				continue
			}
			active.lastMove = time.Now()
			active.moves++
		}
	}
}

type lockPlacement struct {
	job                  lockPlaceJob
	watchStop, watchDone chan struct{}
	savedX, savedY       float64
	physical             uint64
	started, lastMove    time.Time
	segment              dockLockSegment
	moves                int
}

func nativeDockLockProbe() uint64 { return uint64(C.ot_lock_probe()) }
func nativeDockLockSnapshot() (dockLockRaw, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var raw dockLockRaw
	err := readLockJSON(C.ot_lock_snapshot(nil), &raw)
	return raw, err
}

func nativeDockLockPlacementProbe() uint64 { return uint64(C.ot_lock_placement_probe()) }

func nativeDockLockGeneration() uint64 { return uint64(C.ot_lock_generation()) }

func dockLockFingerprint(raw dockLockRaw) string {
	encoded, _ := json.Marshal(raw.Displays)
	return fmt.Sprintf("%d|%s|%s", raw.PID, raw.Edge, encoded)
}
func nativeDockLockDeliveryProbe() uint64 { return uint64(C.ot_lock_delivery_probe()) }
func nativeDockLockNullTransport() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	owner := C.ot_lock_create(0)
	if owner == nil {
		return errors.New("allocation")
	}
	defer C.ot_lock_destroy(owner)
	C.ot_lock_transport_only(owner)
	if code := C.ot_lock_start(owner); code != 0 {
		C.ot_lock_stop(owner)
		return fmt.Errorf("transport tap startup %d", code)
	}
	defer C.ot_lock_stop(owner)
	var x, y C.double
	C.ot_lock_pointer(&x, &y)
	foreground := C.ot_lock_foreground()
	generation := C.ot_lock_generation()
	physical := C.ot_lock_physical(owner)
	C.ot_lock_begin_placement(owner, 1)
	done := make(chan C.int, 1)
	go func() { done <- C.ot_lock_move(owner, generation, physical, 1, x+10, y+10, 0, 3) }()
	var result C.int
waiting:
	for {
		select {
		case result = <-done:
			break waiting
		default:
			C.ot_lock_pump(owner)
		}
	}
	var afterX, afterY C.double
	C.ot_lock_pointer(&afterX, &afterY)
	if result != 1 {
		return fmt.Errorf("inert carrier not acknowledged: seen=%d physicalBefore=%d after=%d pointerBefore=(%.1f,%.1f) after=(%.1f,%.1f) foregroundBefore=%d after=%d", C.ot_lock_carriers_seen(owner), physical, C.ot_lock_physical(owner), float64(x), float64(y), float64(afterX), float64(afterY), foreground, C.ot_lock_foreground())
	}
	if x != afterX || y != afterY || foreground != C.ot_lock_foreground() {
		return errors.New("cursor or foreground changed during inert carrier smoke")
	}
	return nil
}
