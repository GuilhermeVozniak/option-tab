//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_launcher_items.h"
#include "darwin_launcher_badges.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"runtime/cgo"
	"unsafe"
)

func (s *darwinLauncherReferenceSource) ResolveLauncherBadgeTarget(ctx context.Context, expected LauncherReference) (LauncherBadgeTarget, error) {
	ctx, end, err := s.life.begin(ctx)
	if err != nil {
		return LauncherBadgeTarget{}, err
	}
	defer end()
	return resolveReferenceBadgeTarget(ctx, expected, func() (badgeReferenceResolution, error) {
		r, err := s.store.Get(expected.ID)
		if err != nil {
			return badgeReferenceResolution{}, launcherItemError("changed")
		}
		if r.Kind != "app" || r.BundleID != expected.BundleID {
			return badgeReferenceResolution{}, launcherItemError("changed")
		}
		scope, err := resolveLauncherNative(nativeRecord(r))
		if err != nil {
			return badgeReferenceResolution{}, err
		}
		dto := referenceDTO(r, "ready")
		dto.Revision = expected.Revision
		dto.Process = scope.process
		dto.Label = scope.label
		return badgeReferenceResolution{Reference: dto, Path: scope.path, Close: scope.close, Current: func() bool {
			if s.current(ctx, r) != nil || !s.authority.current(r.ID, expected.Revision, r.Revision, scope.fingerprint) || !scope.current() {
				return false
			}
			raw := C.ot_launcher_item_scope_info(scope.pointer)
			if raw == nil {
				return false
			}
			defer C.free(unsafe.Pointer(raw))
			var latest struct {
				Process                          ProcessIdentity
				ProcessState, Fingerprint, Label string
			}
			if json.Unmarshal([]byte(C.GoString(raw)), &latest) != nil || latest.ProcessState == "ambiguous" || latest.ProcessState == "unavailable" || latest.Process != expected.Process || latest.Fingerprint != scope.fingerprint || boundedLauncherLabel(latest.Label, "Application") != expected.Label {
				return false
			}
			return scope.current() && s.current(ctx, r) == nil && s.authority.current(r.ID, expected.Revision, r.Revision, scope.fingerprint)
		}}, nil
	})
}

func (*darwinPlatform) ResolveRunningLauncherBadgeTarget(ctx context.Context, process ProcessIdentity, bundle string) (LauncherBadgeTarget, error) {
	return resolveRunningBadgeTarget(ctx, process, bundle, readRunningLauncherBadgeTarget)
}

func readRunningLauncherBadgeTarget(ctx context.Context, p ProcessIdentity, bundle string) (LauncherBadgeTarget, error) {
	h := cgo.NewHandle(ctx)
	defer h.Delete()
	b := C.CString(bundle)
	defer C.free(unsafe.Pointer(b))
	raw := C.ot_launcher_badge_running_target(C.int(p.PID), C.uint64_t(p.StartSeconds), C.uint64_t(p.StartMicros), b, C.uintptr_t(h))
	if raw == nil {
		return LauncherBadgeTarget{}, launcherItemError("unavailable")
	}
	defer C.free(unsafe.Pointer(raw))
	var v LauncherBadgeTarget
	if json.Unmarshal([]byte(C.GoString(raw)), &v) != nil {
		return LauncherBadgeTarget{}, launcherItemError("unavailable")
	}
	return v, ctx.Err()
}
