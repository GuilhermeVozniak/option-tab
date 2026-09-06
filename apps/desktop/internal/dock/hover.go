// Package dock manages native Dock hover previews independently of the UI host.
package dock

import (
	"time"

	"option-tab/internal/domain"
)

// Item identifies an app or folder represented by a native Dock icon.
type Item struct {
	Kind     string          `json:"kind"`
	AppID    domain.AppID    `json:"appId"`
	BundleID string          `json:"bundleId"`
	Path     string          `json:"path"`
	Title    string          `json:"title"`
	Bounds   domain.Bounds   `json:"bounds"`
	ScreenID domain.ScreenID `json:"screenId"`
	Edge     string          `json:"edge"`
}

func (i Item) same(other *Item) bool {
	return other != nil && i.Kind == other.Kind && i.AppID == other.AppID && i.Path == other.Path
}

// Change describes only a transition; a zero value preserves the shown panel.
type Change struct {
	Show *Item
	Hide bool
}

// Hover runs on one owner goroutine. Native observations and panel entry share
// this state so crossing the gap between icon and preview cannot close it early.
type Hover struct {
	openDelay, closeDelay time.Duration
	pending, shown        *Item
	pendingAt, leaveAt    time.Time
}

func NewHover(openDelay, closeDelay time.Duration) *Hover {
	return &Hover{openDelay: max(0, openDelay), closeDelay: max(0, closeDelay)}
}

func copyItem(i *Item) *Item {
	if i == nil {
		return nil
	}
	out := *i
	return &out
}

func (h *Hover) Reset() Change {
	wasOpen := h.shown != nil
	h.pending = nil
	h.shown = nil
	h.pendingAt = time.Time{}
	h.leaveAt = time.Time{}
	return Change{Hide: wasOpen}
}

func (h *Hover) Step(at time.Time, item *Item, overPanel bool) Change {
	if item != nil {
		h.leaveAt = time.Time{}
		if item.same(h.shown) {
			h.pending = nil
			h.shown = copyItem(item)
			return Change{}
		}
		if !item.same(h.pending) {
			h.pending = copyItem(item)
			h.pendingAt = at
		}
		if at.Sub(h.pendingAt) >= h.openDelay {
			h.shown = copyItem(item)
			h.pending = nil
			return Change{Show: copyItem(item)}
		}
		return Change{}
	}
	h.pending = nil
	if h.shown == nil {
		return Change{}
	}
	if overPanel {
		h.leaveAt = time.Time{}
		return Change{}
	}
	if h.leaveAt.IsZero() {
		h.leaveAt = at
	}
	if at.Sub(h.leaveAt) >= h.closeDelay {
		return h.Reset()
	}
	return Change{}
}

// PlacePanel anchors a preview outside the Dock and keeps it within the monitor
// in top-left global point coordinates, including monitors left of the primary.
func PlacePanel(icon, screen domain.Bounds, edge string, width, height, gap float64) domain.Bounds {
	width = max(0, min(width, screen.W))
	height = max(0, min(height, screen.H))
	gap = max(0, gap)
	x, y := icon.X+(icon.W-width)/2, icon.Y-height-gap
	switch edge {
	case "left":
		x, y = icon.X+icon.W+gap, icon.Y+(icon.H-height)/2
	case "right":
		x, y = icon.X-width-gap, icon.Y+(icon.H-height)/2
	}
	return domain.Bounds{X: max(screen.X, min(x, screen.X+screen.W-width)), Y: max(screen.Y, min(y, screen.Y+screen.H-height)), W: width, H: height}
}
