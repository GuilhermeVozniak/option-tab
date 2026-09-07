//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_active_window.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"math"
	"runtime/cgo"
	"unsafe"

	"option-tab/internal/domain"
)

func nativeAutomationError(status int) error {
	if status == 0 {
		return nil
	}
	code, message := "unavailable", "native automation target unavailable"
	switch status {
	case 1:
		code, message = "invalidArgument", "invalid native automation request"
	case 2:
		code, message = "permissionDenied", "accessibility permission is required"
	case 3:
		code, message = "staleIdentity", "native process or window identity changed"
	case 4:
		code, message = "unavailable", "authoritative AX window evidence unavailable"
	case 5:
		code, message = "unsupported", "requested AX action is unsupported"
	case 6:
		code, message = "unavailable", "native application rejected the action"
	case 7:
		code, message = "retired", "automation request retired"
	case 8:
		code, message = "timeout", "native automation lookup deadline exceeded"
	}
	return &AutomationNativeError{code, message}
}

func nativeIdentity(id AutomationWindowIdentity) C.OTAutomationIdentity {
	return C.OTAutomationIdentity{window: C.uint32_t(id.ID), pid: C.int(id.Process.PID), sec: C.uint64_t(id.Process.StartSeconds), usec: C.uint64_t(id.Process.StartMicros)}
}

func goIdentity(id C.OTAutomationIdentity) AutomationWindowIdentity {
	return AutomationWindowIdentity{ID: domain.WindowID(id.window), Process: ProcessIdentity{PID: domain.AppID(id.pid), StartSeconds: uint64(id.sec), StartMicros: uint64(id.usec)}}
}

func (*darwinPlatform) ProcessIdentity(pid domain.AppID) (ProcessIdentity, error) {
	if pid <= 0 || pid > math.MaxInt32 {
		return ProcessIdentity{}, nativeAutomationError(1)
	}
	var result C.OTAutomationIdentity
	status := C.ot_automation_process_identity(C.int(pid), &result)
	return goIdentity(result).Process, nativeAutomationError(int(status))
}

func (*darwinPlatform) WindowIdentity(id domain.WindowID) (AutomationWindowIdentity, error) {
	if id == 0 || uint64(id) > math.MaxUint32 {
		return AutomationWindowIdentity{}, nativeAutomationError(1)
	}
	var result C.OTAutomationIdentity
	status := C.ot_automation_window_identity(C.uint32_t(id), &result)
	return goIdentity(result), nativeAutomationError(int(status))
}

func (*darwinPlatform) WindowIdentityCurrent(id AutomationWindowIdentity) bool {
	if id.ID == 0 || uint64(id.ID) > math.MaxUint32 || id.Process.PID <= 0 || id.Process.PID > math.MaxInt32 || id.Process.StartSeconds == 0 {
		return false
	}
	return C.ot_automation_window_current(nativeIdentity(id)) != 0
}

func (*darwinPlatform) ActiveWindow(ctx context.Context) (domain.Window, AutomationWindowIdentity, error) {
	if ctx == nil {
		return domain.Window{}, AutomationWindowIdentity{}, nativeAutomationError(1)
	}
	if err := ctx.Err(); err != nil {
		return domain.Window{}, AutomationWindowIdentity{}, err
	}
	raw := C.ot_automation_active_window()
	if raw == nil {
		return domain.Window{}, AutomationWindowIdentity{}, nativeAutomationError(4)
	}
	defer C.free(unsafe.Pointer(raw))
	var result struct {
		Status   int
		Window   domain.Window
		Identity AutomationWindowIdentity
	}
	if err := json.Unmarshal([]byte(C.GoString(raw)), &result); err != nil {
		return domain.Window{}, AutomationWindowIdentity{}, nativeAutomationError(4)
	}
	if err := ctx.Err(); err != nil {
		return domain.Window{}, AutomationWindowIdentity{}, err
	}
	return result.Window, result.Identity, nativeAutomationError(result.Status)
}

type automationWindowGuard struct {
	ctx   context.Context
	guard func() error
	err   error
}

//export goAutomationWindowFinalGuard
func goAutomationWindowFinalGuard(token C.uintptr_t) C.int {
	g := cgo.Handle(token).Value().(*automationWindowGuard)
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

func (*darwinPlatform) PerformAutomationWindowAction(ctx context.Context, kind string, id AutomationWindowIdentity, fullscreen *bool, guard func() error) error {
	if err := ValidateAutomationWindowAction(ctx, kind, id, fullscreen, guard); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	action := map[string]int{"focus": 1, "close": 2, "minimize": 3, "hide": 4, "fullscreen": 5}[kind]
	desired := 0
	if fullscreen != nil && *fullscreen {
		desired = 1
	}
	g := &automationWindowGuard{ctx: ctx, guard: guard}
	token := cgo.NewHandle(g)
	defer token.Delete()
	status := C.ot_automation_window_action(C.int(action), nativeIdentity(id), C.int(desired), C.uintptr_t(token))
	if g.err != nil {
		return g.err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nativeAutomationError(int(status))
}
