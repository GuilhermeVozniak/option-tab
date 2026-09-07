package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// LauncherItemMutation names existing profile records, never replacement metadata.
type LauncherItemMutation struct {
	Kind     string `json:"kind"`
	ItemID   string `json:"itemID"`
	TargetID string `json:"targetID"`
}

// MutateLauncherItems copies and validates both sides of a bounded item edit.
// Moves stay within one container; add/remove explicitly cross group boundaries.
func MutateLauncherItems(items []LauncherItem, m LauncherItemMutation) ([]LauncherItem, error) {
	refuse := errors.New("invalid launcher item mutation")
	if !validLauncherItems(items) || m.ItemID == m.TargetID {
		return nil, refuse
	}
	out := slices.Clone(items)
	for i := range out {
		out[i].Members = slices.Clone(out[i].Members)
	}
	index := func(id string) int { return slices.IndexFunc(out, func(v LauncherItem) bool { return v.ID == id }) }
	group := func(id string) string {
		for _, v := range out {
			if slices.Contains(v.Members, id) {
				return v.ID
			}
		}
		return ""
	}
	si, ti := index(m.ItemID), index(m.TargetID)
	if si < 0 || ti < 0 || out[si].Kind == "spacer" || out[si].Kind == "separator" {
		return nil, refuse
	}
	sourceGroup, targetGroup := group(m.ItemID), group(m.TargetID)
	detach := func() {
		if sourceGroup == "" {
			return
		}
		i := index(sourceGroup)
		out[i].Members = slices.Delete(out[i].Members, slices.Index(out[i].Members, m.ItemID), slices.Index(out[i].Members, m.ItemID)+1)
		if len(out[i].Members) == 0 {
			out = slices.Delete(out, i, i+1)
		}
	}
	switch m.Kind {
	case "moveBefore", "moveAfter":
		if sourceGroup != targetGroup {
			return nil, refuse
		}
		if sourceGroup != "" {
			i := index(sourceGroup)
			members := out[i].Members
			from := slices.Index(members, m.ItemID)
			members = slices.Delete(members, from, from+1)
			to := slices.Index(members, m.TargetID)
			if m.Kind == "moveAfter" {
				to++
			}
			out[i].Members = slices.Insert(members, to, m.ItemID)
		} else {
			source := out[si]
			out = slices.Delete(out, si, si+1)
			to := index(m.TargetID)
			if m.Kind == "moveAfter" {
				to++
			}
			out = slices.Insert(out, to, source)
		}
	case "addToGroup":
		if out[si].Kind != "app" {
			return nil, refuse
		}
		switch out[ti].Kind {
		case "group":
			if sourceGroup == m.TargetID {
				return nil, refuse
			}
			detach()
			i := index(m.TargetID)
			out[i].Members = append(out[i].Members, m.ItemID)
		case "app":
			if targetGroup != "" {
				return nil, refuse
			}
			detach()
			if len(out) >= 16 {
				return nil, refuse
			}
			id := ""
			for n := 1; n <= 17; n++ {
				candidate := fmt.Sprintf("group-%d", n)
				if index(candidate) < 0 {
					id = candidate
					break
				}
			}
			out = slices.Insert(out, index(m.TargetID), LauncherItem{ID: id, Kind: "group", Label: "Group", Members: []string{m.TargetID, m.ItemID}})
		default:
			return nil, refuse
		}
	case "removeFromGroup":
		if out[si].Kind != "app" || sourceGroup == "" || sourceGroup != m.TargetID {
			return nil, refuse
		}
		source := out[si]
		out = slices.Delete(out, si, si+1)
		// Keep the group's visible position even when its final member leaves.
		gi := index(sourceGroup)
		detach()
		if surviving := index(sourceGroup); surviving >= 0 {
			gi = surviving + 1
		}
		out = slices.Insert(out, gi, source)
	default:
		return nil, refuse
	}
	if !validLauncherItems(out) {
		return nil, refuse
	}
	return out, nil
}

// LauncherItemsRevision hashes the same normalized compact representation used by settings CAS.
func LauncherItemsRevision(items []LauncherItem) string {
	if items == nil {
		items = []LauncherItem{}
	}
	data, _ := json.Marshal(items)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
