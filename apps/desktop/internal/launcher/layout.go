package launcher

import (
	"math"

	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

type Geometry struct {
	Bounds, RevealBand domain.Bounds
	Status             string
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validBounds(b domain.Bounds) bool {
	return finite(b.X) && finite(b.Y) && finite(b.W) && finite(b.H) && b.W > 0 && b.H > 0 && finite(b.X+b.W) && finite(b.Y+b.H)
}

func contains(b domain.Bounds, x, y float64) bool {
	return validBounds(b) && finite(x) && finite(y) && x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H
}

func overlap(a, b domain.Bounds) bool {
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func expand(b domain.Bounds, n float64) domain.Bounds {
	return domain.Bounds{X: b.X - n, Y: b.Y - n, W: b.W + 2*n, H: b.H + 2*n}
}

// Layout uses global top-left logical points. Reserving each physical edge keeps
// native Dock access available even when its orientation changes between samples.
func Layout(p config.LauncherProfile, d platform.LauncherDisplay, count int, protected []domain.Bounds) Geometry {
	fail := Geometry{Status: "unavailable"}
	if !validBounds(d.Frame) || !validBounds(d.UsableFrame) || !finite(d.Scale) || d.Scale <= 0 || count < 0 || len(protected) > 32 {
		return fail
	}
	if config.ValidateReplacementDock(config.ReplacementDockSettings{Version: 2, Profiles: []config.LauncherProfile{p}}) != nil {
		return fail
	}
	u := d.UsableFrame
	left := math.Max(u.X, d.Frame.X+32)
	right := math.Min(u.X+u.W, d.Frame.X+d.Frame.W-32)
	top := math.Max(u.Y, d.Frame.Y+32)
	bottom := math.Min(u.Y+u.H, d.Frame.Y+d.Frame.H-32)
	inset := float64(p.InsetPx)
	left += inset
	right -= inset
	top += inset
	bottom -= inset
	vertical := p.Edge == "left" || p.Edge == "right"
	alongStart, available := left, right-left
	if vertical {
		alongStart, available = top, bottom-top
	}
	thickness := float64(p.ThicknessPx)
	if available < 48 || right-left < thickness || bottom-top < thickness {
		return fail
	}
	spacing := float64(p.Appearance.ItemSpacingPx)
	count = min(count, 144) // 128 running applications plus 16 configured records.
	n := max(count, 1)
	length := float64(n*p.IconPx) + float64(n-1)*spacing + 24
	length += float64(visibleWidgetSlots(p)) * (160 + spacing)
	if p.Layout == "fullWidth" {
		length = available
	} else {
		length = math.Min(length, math.Min(available, available*p.MaxLengthFraction))
	}
	if length < 48 {
		return fail
	}
	along := alongStart
	switch p.Alignment {
	case "center":
		along += (available - length) / 2
	case "end":
		along += available - length
	}
	var b domain.Bounds
	switch p.Edge {
	case "bottom":
		b = domain.Bounds{X: along, Y: bottom - thickness, W: length, H: thickness}
	case "top":
		b = domain.Bounds{X: along, Y: top, W: length, H: thickness}
	case "left":
		b = domain.Bounds{X: left, Y: along, W: thickness, H: length}
	case "right":
		b = domain.Bounds{X: right - thickness, Y: along, W: thickness, H: length}
	}
	// Move only inward; monotonic movement bounds chains and prevents oscillation.
	for range len(protected) + 1 {
		moved := false
		for _, r := range protected {
			if !validBounds(r) {
				return fail
			}
			if !overlap(b, r) {
				continue
			}
			switch p.Edge {
			case "bottom":
				b.Y = r.Y - inset - b.H
			case "top":
				b.Y = r.Y + r.H + inset
			case "left":
				b.X = r.X + r.W + inset
			case "right":
				b.X = r.X - inset - b.W
			}
			moved = true
		}
		if !moved {
			break
		}
	}
	if !validBounds(b) || b.X < left || b.Y < top || b.X+b.W > right || b.Y+b.H > bottom {
		return fail
	}
	for _, r := range protected {
		if overlap(b, r) {
			return fail
		}
	}
	reveal := b
	switch p.Edge {
	case "bottom":
		reveal.Y = b.Y + b.H - 8
		reveal.H = 8
	case "top":
		reveal.H = 8
	case "left":
		reveal.W = 8
	case "right":
		reveal.X = b.X + b.W - 8
		reveal.W = 8
	}
	return Geometry{Bounds: b, RevealBand: reveal, Status: "ready"}
}

func visibleWidgetSlots(p config.LauncherProfile) int {
	memberStack := map[string]string{}
	for _, stack := range p.Stacks {
		for _, id := range stack.Members {
			memberStack[id] = stack.ID
		}
	}
	count := 0
	seen := map[string]bool{}
	for _, w := range p.Widgets {
		if !w.Enabled {
			continue
		}
		if stack := memberStack[w.ID]; stack != "" {
			if seen[stack] {
				continue
			}
			seen[stack] = true
		}
		count++
	}
	return count
}
