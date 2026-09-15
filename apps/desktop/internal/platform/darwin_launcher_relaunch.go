//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_launcher_items.h"
*/
import "C"

import (
	"context"
	"strings"
	"unsafe"
)

func (*darwinPlatform) RelaunchLauncherApp(ctx context.Context, target LauncherAppTarget, guard func() error) error {
	if ctx == nil || guard == nil || target.PanelToken == 0 || target.DisplayUUID == "" || len(target.BundleID) == 0 || len(target.BundleID) > 1024 || strings.ContainsRune(target.BundleID, 0) || target.Process.PID <= 0 || target.Process.StartSeconds == 0 || target.Process.StartMicros >= 1_000_000 {
		return launcherItemError("invalidArgument")
	}
	if C.ot_launcher_item_main() != 0 {
		return launcherItemError("unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	bundle := C.CString(target.BundleID)
	defer C.free(unsafe.Pointer(bundle))
	var failure *C.char
	pointer := C.ot_launcher_item_capture_running(C.int(target.Process.PID), C.uint64_t(target.Process.StartSeconds), C.uint64_t(target.Process.StartMicros), bundle, &failure)
	if failure != nil {
		code := C.GoString(failure)
		C.free(unsafe.Pointer(failure))
		return launcherItemError(code)
	}
	if pointer == nil {
		return launcherItemError("unavailable")
	}
	scope := &launcherNativeScope{pointer: pointer}
	defer scope.close()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := guard(); err != nil {
		return err
	}
	return openLauncherNative(ctx, scope, LauncherItemAction{Kind: "relaunch", Process: target.Process, BundleID: target.BundleID, DisplayUUID: target.DisplayUUID, PanelToken: target.PanelToken}, "", guard)
}
