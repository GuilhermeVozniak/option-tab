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
	case config.OrderRecentlyCreated:
		// Window-server IDs increase monotonically as windows are created, so a
		// descending ID sort approximates newest-first creation order.
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].ID > out[j].ID
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

// SendToBack stably moves windows whose class is configured "showAtEnd" to the
// end of the list (AltTab's third tristate option for minimized/hidden/
// fullscreen windows). Relative order within both partitions is preserved.
func SendToBack(wins []domain.Window, f config.Filters) []domain.Window {
	atEnd := func(w domain.Window) bool {
		return (f.ShowMinimized == config.VisShowAtEnd && w.Minimized) ||
			(f.ShowHiddenApps == config.VisShowAtEnd && w.Hidden) ||
			(f.ShowFullscreen == config.VisShowAtEnd && w.Fullscreen)
	}
	front := make([]domain.Window, 0, len(wins))
	var back []domain.Window
	for _, w := range wins {
		if atEnd(w) {
			back = append(back, w)
		} else {
			front = append(front, w)
		}
	}
	return append(front, back...)
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
