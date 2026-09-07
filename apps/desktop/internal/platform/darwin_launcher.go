//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_launcher.h"
#include "darwin_dock_panel.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"runtime/cgo"
	"time"
	"unsafe"

	"option-tab/internal/domain"
)

func launcherCStringResult(raw *C.char) []byte {
	if raw == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(raw))
	return []byte(C.GoString(raw))
}

func nativeLauncherSnapshot() LauncherEnvironment {
	var raw struct {
		Complete bool
		Displays []LauncherDisplay
		Dock     struct {
			PID          domain.AppID
			Process      ProcessIdentity
			Edge, Reason string
			Container    domain.Bounds
		}
	}
	state := LauncherEnvironment{Status: "unavailable", Reason: "environmentUnavailable", NativeDock: LauncherNativeDock{Confidence: "unknown", Edge: "unknown", Visibility: "unknown"}}
	if json.Unmarshal(launcherCStringResult(C.ot_launcher_environment()), &raw) != nil {
		return state
	}
	state.Complete = raw.Complete
	state.Displays = raw.Displays
	applyLauncherSpaces(state.Displays, launcherCStringResult(C.ot_launcher_spaces()))
	if raw.Dock.Reason == "" && raw.Dock.PID > 0 && raw.Dock.Container.W > 0 && raw.Dock.Container.H > 0 {
		identity, err := (&darwinPlatform{}).ProcessIdentity(raw.Dock.PID)
		if err == nil && identity == raw.Dock.Process {
			var geometry []DockLockDisplay
			for _, d := range state.Displays {
				geometry = append(geometry, DockLockDisplay{UUID: d.UUID, Bounds: d.Frame, Mirrored: d.MirrorGroup != ""})
			}
			edge := raw.Dock.Edge
			if edge == "" {
				edge = dockLockInferredEdge(geometry, raw.Dock.Container)
			}
			if dockLockActual(geometry, edge, raw.Dock.Container) == "" {
				return state
			}
			state.NativeDock = LauncherNativeDock{Process: identity, Edge: edge, Bounds: raw.Dock.Container, Confidence: "known", Visibility: "visible"}
			if state.NativeDock.Edge == "" {
				state.NativeDock.Edge = "unknown"
				state.NativeDock.Confidence = "unknown"
			}
			// A positive off-display container is hidden; uncertain geometry stays
			// conservative and makes the controller yield rather than invent visibility.
			overlaps := false
			for _, d := range state.Displays {
				b, c := d.Frame, raw.Dock.Container
				if c.X < b.X+b.W && c.X+c.W > b.X && c.Y < b.Y+b.H && c.Y+c.H > b.Y {
					overlaps = true
				}
			}
			if !overlaps {
				state.NativeDock.Visibility = "hidden"
			}
		}
	}
	if state.Complete && state.NativeDock.Confidence == "known" {
		state.Status = "ready"
		state.Reason = ""
	}
	return state
}

func (*darwinPlatform) ObserveLauncherEnvironment(ctx context.Context, emit func(LauncherEnvironment)) error {
	if ctx == nil || emit == nil {
		return errors.New("invalid launcher observer")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	watch := C.ot_launcher_watch_start()
	if watch == nil {
		return errors.New("launcher topology observer unavailable")
	}
	defer C.ot_launcher_watch_stop(watch)
	return observeLauncherEnvironment(ctx, emit, launcherEnvironmentOps{Changed: func() bool { return C.ot_launcher_watch_changed(watch) != 0 }, Snapshot: nativeLauncherSnapshot, Pointer: func() (float64, float64, bool) {
		var x, y C.double
		ok := C.ot_launcher_pointer(&x, &y)
		return float64(x), float64(y), ok != 0
	}}, 50*time.Millisecond)
}

type darwinLauncherPanel struct{ *dockPanel }

func (p *darwinLauncherPanel) LauncherToken() uint64 { return p.token }
func (*darwinPlatform) CreateLauncherPanel(host unsafe.Pointer, display string) (LauncherPanel, error) {
	if host == nil || display == "" {
		return nil, ErrDockPanelHostClosed
	}
	name := C.CString(display)
	defer C.free(unsafe.Pointer(name))
	token := uint64(C.ot_launcher_panel_create(host, name))
	if token == 0 {
		return nil, ErrDockPanelHostClosed
	}
	return &darwinLauncherPanel{newDockPanel(token, darwinDockPanelNative{})}, nil
}

type launcherFinalGuard struct {
	ctx   context.Context
	guard func() error
	err   error
}

//export goLauncherFinalGuard
func goLauncherFinalGuard(token C.uintptr_t) C.int {
	g := cgo.Handle(token).Value().(*launcherFinalGuard)
	g.err = g.ctx.Err()
	if g.err == nil {
		g.err = g.guard()
	}
	if g.err == nil {
		g.err = g.ctx.Err()
	}
	if g.err != nil {
		return 0
	}
	return 1
}

func (*darwinPlatform) ActivateLauncherApp(ctx context.Context, target LauncherAppTarget, guard func() error) error {
	if ctx == nil || guard == nil || target.PanelToken == 0 || target.DisplayUUID == "" || target.BundleID == "" || target.Process.PID <= 0 || target.Process.StartSeconds == 0 {
		return errors.New("invalid launcher target")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	displays := []LauncherDisplay{{UUID: target.DisplayUUID}}
	applyLauncherSpaces(displays, launcherCStringResult(C.ot_launcher_spaces()))
	if displays[0].SpaceKind != "ordinary" {
		return errors.New("launcher display space unavailable")
	}
	g := &launcherFinalGuard{ctx: ctx, guard: guard}
	handle := cgo.NewHandle(g)
	defer handle.Delete()
	display, bundle := C.CString(target.DisplayUUID), C.CString(target.BundleID)
	defer C.free(unsafe.Pointer(display))
	defer C.free(unsafe.Pointer(bundle))
	ok := C.ot_launcher_activate(C.uint64_t(target.PanelToken), display, C.uint64_t(displays[0].SpaceID), C.int(target.Process.PID), C.uint64_t(target.Process.StartSeconds), C.uint64_t(target.Process.StartMicros), bundle, C.uintptr_t(handle))
	if g.err != nil {
		return g.err
	}
	if ok == 0 {
		return errors.New("launcher activation refused")
	}
	return nil
}
