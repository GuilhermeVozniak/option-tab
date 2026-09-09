package platform

import (
	"bytes"
	"encoding/json"
	"strconv"
)

type (
	launcherSpace struct {
		ID      json.Number `json:"id64"`
		Managed json.Number `json:"ManagedSpaceID"`
		Type    json.Number `json:"type"`
	}
	launcherSpaceRecord struct {
		Display string          `json:"Display Identifier"`
		Current launcherSpace   `json:"Current Space"`
		Spaces  []launcherSpace `json:"Spaces"`
	}
)

func launcherSpaceNumbers(s launcherSpace) (uint64, uint64, bool) {
	id, e := strconv.ParseUint(string(s.ID), 10, 64)
	m, e2 := strconv.ParseUint(string(s.Managed), 10, 64)
	kind, e3 := strconv.ParseUint(string(s.Type), 10, 64)
	return id, kind, e == nil && e2 == nil && e3 == nil && id != 0 && id == m
}

// A private schema is evidence only when the UUID mapping and both current
// identifiers agree with exactly one inventory entry. Unknown types never yield
// ordinary permission. Shared-space aliases cannot bind a physical display.
func applyLauncherSpaces(displays []LauncherDisplay, data []byte) {
	for i := range displays {
		displays[i].SpaceKind = "unknown"
		displays[i].SpaceStatus = "unavailable"
		displays[i].SpaceID = 0
	}
	if len(data) == 0 || len(data) > 256*1024 {
		return
	}
	var records []launcherSpaceRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&records) != nil || len(records) > 32 {
		return
	}
	for i := range displays {
		d := &displays[i]
		if d.MirrorGroup != "" {
			continue
		}
		var matches []launcherSpaceRecord
		for _, r := range records {
			if r.Display == d.UUID {
				matches = append(matches, r)
			}
		}
		if len(matches) != 1 {
			continue
		}
		r := matches[0]
		id, kind, ok := launcherSpaceNumbers(r.Current)
		if !ok || len(r.Spaces) > 128 {
			continue
		}
		count := 0
		for _, s := range r.Spaces {
			sid, sk, valid := launcherSpaceNumbers(s)
			if valid && sid == id && sk == kind {
				count++
			}
		}
		if count != 1 {
			continue
		}
		d.SpaceID = id
		if kind == 0 {
			d.SpaceKind = "ordinary"
			d.SpaceStatus = "known"
		} else {
			d.SpaceStatus = "unsupported"
		}
	}
}
