// Package order sorts windows for display in the switcher according to the
// configured OrderMode. It returns a new slice and never mutates its input.
package order

import (
	"sort"
	"strings"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

// Sort returns a new, ordered copy of wins per mode. Sorting is stable, so
// windows comparing equal keep their original relative order. An unknown mode
// falls back to most-recently-used.
func Sort(wins []domain.Window, mode config.OrderMode) []domain.Window {
	out := make([]domain.Window, len(wins))
	copy(out, wins)

	switch mode {
	case config.OrderAlphabetical:
		sort.SliceStable(out, func(i, j int) bool {
			ai, aj := strings.ToLower(out[i].AppName), strings.ToLower(out[j].AppName)
			if ai != aj {
				return ai < aj
			}
			return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
		})
	case config.OrderSpace:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].SpaceID != out[j].SpaceID {
				return out[i].SpaceID < out[j].SpaceID
			}
			return moreRecent(out[i], out[j])
		})
	default: // OrderRecent and any unknown mode
		sort.SliceStable(out, func(i, j int) bool {
			return moreRecent(out[i], out[j])
		})
	}
	return out
}

// moreRecent reports whether a should sort before b under recency ordering.
// Windows with a zero LastFocused sort after windows with a real timestamp; two
// zero-time windows compare equal (stable sort keeps their input order).
func moreRecent(a, b domain.Window) bool {
	az, bz := a.LastFocused.IsZero(), b.LastFocused.IsZero()
	if az != bz {
		return bz // non-zero (a) comes before zero (b)
	}
	if az && bz {
		return false
	}
	return a.LastFocused.After(b.LastFocused)
}
