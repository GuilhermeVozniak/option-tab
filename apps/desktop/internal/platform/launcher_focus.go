package platform

import (
	"encoding/json"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Focus uncertainty affects rule selection only; retain valid independent
// display, Space and native Dock evidence unchanged.
func normalizeLauncherFocus(state LauncherEnvironment) LauncherEnvironment {
	p := state.FocusedProcess
	b := state.FocusedBundleID
	valid := state.FocusKnown && p.PID > 0 && p.PID <= math.MaxInt32 && p.StartSeconds > 0 && p.StartMicros < 1000000 && len(b) > 0 && len(b) <= 255 && utf8.ValidString(b) && !strings.ContainsAny(b, "/\\")
	for _, r := range b {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			valid = false
		}
	}
	if !valid {
		state.FocusKnown = false
		state.FocusedProcess = ProcessIdentity{}
		state.FocusedBundleID = ""
	}
	return state
}

// Decode focus separately so malformed optional identity metadata cannot discard
// an otherwise usable topology observation.
func mapLauncherFocus(state LauncherEnvironment, raw []byte) LauncherEnvironment {
	var focus struct {
		Known    bool
		Process  ProcessIdentity
		BundleID string
	}
	state.FocusKnown = false
	state.FocusedProcess = ProcessIdentity{}
	state.FocusedBundleID = ""
	if json.Unmarshal(raw, &focus) == nil {
		state.FocusKnown = focus.Known
		state.FocusedProcess = focus.Process
		state.FocusedBundleID = focus.BundleID
	}
	return normalizeLauncherFocus(state)
}
