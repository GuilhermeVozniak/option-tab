//go:build darwin

package platform

/*
#include "darwin.h"
#include "darwin_actions.h"
*/
import "C"

import (
	"fmt"
	"math"
	"os"

	"option-tab/internal/domain"
)

func (p *darwinPlatform) PerformTargetAction(kind string, id domain.WindowID, app domain.AppID) error {
	if app <= 0 || int64(app) > math.MaxInt32 || int(app) == os.Getpid() || uint64(id) > math.MaxUint32 {
		return fmt.Errorf("invalid or self target identity")
	}
	pid := C.int(app)
	if C.ot_action_app_valid(pid) != 1 {
		return fmt.Errorf("application is no longer running or is not an actionable app")
	}
	window := kind == "focus" || kind == "close" || kind == "minimize" || kind == "setMinimized" || kind == "fullscreen"
	if window && (id == 0 || C.ot_window_pid(C.uint32_t(id)) != pid) {
		return fmt.Errorf("window no longer exists or application identity mismatches")
	}
	var result C.int
	switch kind {
	case "focus", "minimize", "setMinimized":
		return performNativeWindowAction(kind, id, app, nativeWindowOps{
			prepare: func(id domain.WindowID, app domain.AppID) { C.ot_action_prepare_focus(C.uint32_t(id), C.int(app)) },
			raise: func(id domain.WindowID, app domain.AppID) bool {
				return C.ot_action_raise_window(C.uint32_t(id), C.int(app)) == 1
			},
			toggleMinimize: func(id domain.WindowID, app domain.AppID) bool {
				return C.ot_minimize_window(C.uint32_t(id), C.int(app)) == 1
			},
			setMinimized: func(id domain.WindowID, app domain.AppID) bool {
				return C.ot_action_set_minimized(C.uint32_t(id), C.int(app)) == 1
			},
		})
	case "close":
		result = C.ot_action_close_window(C.uint32_t(id), pid)
	case "fullscreen":
		result = C.ot_fullscreen_window(C.uint32_t(id), pid)
	case "hide":
		result = C.ot_hide_app(pid)
	case "quit":
		result = C.ot_quit_app(pid)
	case "forceQuit":
		result = C.ot_action_force_quit(pid)
	case "newWindow":
		result = C.ot_action_new_window(pid)
		if result == -1 {
			return fmt.Errorf("application does not expose a supported, enabled New Window menu command")
		}
	default:
		return fmt.Errorf("unsupported action %q", kind)
	}
	if result != 1 {
		return fmt.Errorf("%s was refused or could not be completed by the application", kind)
	}
	return nil
}

func (p *darwinPlatform) NewWindow(app domain.AppID) error {
	return p.PerformTargetAction("newWindow", 0, app)
}

func (p *darwinPlatform) ForceQuitApp(app domain.AppID) error {
	return p.PerformTargetAction("forceQuit", 0, app)
}

// Preparation preserves cross-Space fronting, but is not evidence that the
// requested window accepted focus. Only a successful AX raise is acknowledged.
type nativeWindowOps struct {
	prepare        func(domain.WindowID, domain.AppID)
	raise          func(domain.WindowID, domain.AppID) bool
	toggleMinimize func(domain.WindowID, domain.AppID) bool
	setMinimized   func(domain.WindowID, domain.AppID) bool
}

func performNativeWindowAction(kind string, id domain.WindowID, app domain.AppID, ops nativeWindowOps) error {
	accepted := false
	switch kind {
	case "focus":
		ops.prepare(id, app)
		accepted = ops.raise(id, app)
	case "minimize":
		accepted = ops.toggleMinimize(id, app)
	case "setMinimized":
		accepted = ops.setMinimized(id, app)
	}
	if !accepted {
		return fmt.Errorf("%s was refused or could not be acknowledged by the target window", kind)
	}
	return nil
}

// ActionWindows snapshots AXWindow roots rather than all layer-zero CG
// surfaces. It intentionally does not filter by title, visibility, or Space.
func (p *darwinPlatform) ActionWindows(app domain.AppID) ([]domain.Window, error) {
	if app <= 0 || int64(app) > math.MaxInt32 || int(app) == os.Getpid() {
		return nil, fmt.Errorf("invalid or self application identity")
	}
	return snapshotActionWindows(app, nativeActionSourceOps{
		available: func() bool {
			return C.ot_action_app_valid(C.int(app)) == 1 && C.ot_action_windows_available(C.int(app)) == 1
		},
		windows: p.Windows,
		classify: func(id domain.WindowID, app domain.AppID) int {
			return int(C.ot_action_window_is_root(C.uint32_t(id), C.int(app)))
		},
	})
}

type nativeActionSourceOps struct {
	available func() bool
	windows   func() ([]domain.Window, error)
	classify  func(domain.WindowID, domain.AppID) int
}

// Classification is tri-state: root=1, confirmed non-window=0, and negative
// means unresolved/refused. Unknown candidates never become clean omissions.
func snapshotActionWindows(app domain.AppID, ops nativeActionSourceOps) ([]domain.Window, error) {
	if !ops.available() {
		return nil, fmt.Errorf("application window enumeration is unavailable or refused")
	}
	windows, err := ops.windows()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Window, 0)
	unresolved := 0
	for _, w := range windows {
		if w.AppID != app {
			continue
		}
		switch ops.classify(w.ID, app) {
		case 1:
			out = append(out, w)
		case 0: // Positively classified non-window surface.
		default:
			unresolved++
		}
	}
	if unresolved > 0 {
		return out, fmt.Errorf("window enumeration incomplete: %d candidate(s) could not be classified because AX lookup was unavailable, refused, or timed out", unresolved)
	}
	return out, nil
}
