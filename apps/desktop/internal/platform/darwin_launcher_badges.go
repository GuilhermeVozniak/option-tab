//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_launcher_badges.h"
*/
import "C"

import (
	"context"
	"encoding/json"
	"errors"
	"runtime/cgo"
	"unsafe"
)

func NewLauncherBadgeSource() LauncherBadgeSource {
	return newLauncherBadgeSource(readNativeLauncherBadges)
}

//export goLauncherBadgeCurrent
func goLauncherBadgeCurrent(token C.uintptr_t) C.int {
	ctx := cgo.Handle(token).Value().(context.Context)
	if ctx.Err() == nil {
		return 1
	}
	return 0
}

func readNativeLauncherBadges(ctx context.Context, targets []LauncherBadgeTarget) (badgeObservation, error) {
	if err := ctx.Err(); err != nil {
		return badgeObservation{}, err
	}
	data, err := json.Marshal(targets)
	if err != nil {
		return badgeObservation{}, errors.New("invalid launcher badge targets")
	}
	text := C.CString(string(data))
	defer C.free(unsafe.Pointer(text))
	handle := cgo.NewHandle(ctx)
	defer handle.Delete()
	raw := C.ot_launcher_badges_read(text, C.uintptr_t(handle))
	if raw == nil {
		return badgeObservation{}, errors.New("launcher badge observation failed")
	}
	defer C.free(unsafe.Pointer(raw))
	if err = ctx.Err(); err != nil {
		return badgeObservation{}, err
	}
	var result struct {
		Dock    ProcessIdentity     `json:"dock"`
		Status  LauncherBadgeStatus `json:"status"`
		Entries []struct {
			ItemKey        string             `json:"itemKey"`
			TargetRevision uint64             `json:"targetRevision"`
			State          LauncherBadgeState `json:"state"`
			Text           *string            `json:"text"`
		} `json:"entries"`
	}
	if err = json.Unmarshal([]byte(C.GoString(raw)), &result); err != nil || len(result.Entries) != len(targets) {
		return badgeObservation{}, errors.New("invalid launcher badge observation")
	}
	observation := badgeObservation{Dock: result.Dock, Status: result.Status, Entries: make([]LauncherBadgeEntry, len(targets))}
	for i, t := range targets {
		entry := result.Entries[i]
		out := LauncherBadgeEntry{ItemKey: t.ItemKey, TargetRevision: t.TargetRevision, State: BadgeUnavailable}
		if entry.ItemKey == t.ItemKey && entry.TargetRevision == t.TargetRevision {
			switch entry.State {
			case BadgeUnsupported:
				out.State = BadgeUnsupported
			case BadgeKnown:
				if entry.Text != nil {
					out.State = BadgeKnown
					out.Kind, out.Count = classifyLauncherBadge(*entry.Text)
				}
			}
		}
		observation.Entries[i] = out
	}
	return observation, nil
}
