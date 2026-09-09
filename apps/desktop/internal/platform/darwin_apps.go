//go:build darwin

package platform

/*
#include <stdlib.h>
#include "darwin_apps.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"unsafe"

	"option-tab/internal/domain"
)

const (
	applicationPolicyRegular    = 0
	applicationPolicyAccessory  = 1
	applicationPolicyProhibited = 2
)

type rawApplication struct {
	PID         int    `json:"pid"`
	Name        string `json:"name"`
	BundleID    string `json:"bundleId"`
	Hidden      bool   `json:"hidden"`
	Terminated  bool   `json:"terminated"`
	Policy      int    `json:"policy"`
	WindowCount int    `json:"windowCount,omitempty"`
}

func mapRawApplications(raws []rawApplication, self int) []domain.App {
	out := make([]domain.App, 0, len(raws))
	for _, raw := range raws {
		if raw.PID <= 0 || raw.PID == self || raw.Terminated || raw.Policy != applicationPolicyRegular || raw.BundleID == "" {
			continue
		}
		out = append(out, domain.App{ID: domain.AppID(raw.PID), Name: raw.Name, BundleID: raw.BundleID, Hidden: raw.Hidden})
	}
	return out
}

func (p *darwinPlatform) Apps() ([]domain.App, error) {
	cstr := C.ot_list_apps_json()
	if cstr == nil {
		return nil, errors.New("native application inventory failed")
	}
	defer C.free(unsafe.Pointer(cstr))
	var raws []rawApplication
	if err := json.Unmarshal([]byte(C.GoString(cstr)), &raws); err != nil {
		return nil, fmt.Errorf("decode native application inventory: %w", err)
	}
	return mapRawApplications(raws, os.Getpid()), nil
}

type applicationActivationStatus int

const (
	applicationActivationInvalid  applicationActivationStatus = -1
	applicationActivationRefused  applicationActivationStatus = 0
	applicationActivationAccepted applicationActivationStatus = 1
)

var errApplicationActivationRefused = errors.New("application activation was refused")

func activateApplication(id domain.AppID, self int, request func(int) applicationActivationStatus) error {
	if id <= 0 || int64(id) > math.MaxInt32 || int(id) == self {
		return errors.New("invalid or self application identity")
	}
	switch request(int(id)) {
	case applicationActivationAccepted:
		return nil
	case applicationActivationRefused:
		return errApplicationActivationRefused
	default:
		return errors.New("application is no longer running or is not an actionable app")
	}
}

func (p *darwinPlatform) ActivateApp(id domain.AppID) error {
	return activateApplication(id, os.Getpid(), func(pid int) applicationActivationStatus {
		return applicationActivationStatus(C.ot_activate_app(C.int(pid)))
	})
}

func frontmostApplicationPID() int { return int(C.ot_frontmost_app_pid()) }
