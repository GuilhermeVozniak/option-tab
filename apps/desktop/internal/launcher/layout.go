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

func Layout(p config.LauncherProfile, d platform.LauncherDisplay, count int, protected []domain.Bounds) Geometry {
	fail := Geometry{Status: "unavailable"}
	if !validBounds(d.Frame) || !validBounds(d.UsableFrame) || !finite(d.Scale) || d.Scale <= 0 || count < 0 || p.Edge != "bottom" || p.Layout != "floating" || !finite(p.MaxLengthFraction) || p.MaxLengthFraction < .25 || p.MaxLengthFraction > .9 || p.ThicknessPx < 48 || p.ThicknessPx > 112 || p.IconPx < 24 || p.IconPx > 64 || p.InsetPx < 12 || p.InsetPx > 64 {
		return fail
	}
	u := d.UsableFrame
	left := math.Max(u.X, d.Frame.X)
	top := math.Max(u.Y, d.Frame.Y)
	right := math.Min(u.X+u.W, d.Frame.X+d.Frame.W)
	bottom := math.Min(u.Y+u.H, d.Frame.Y+d.Frame.H-32)
	inset := float64(p.InsetPx)
	available := right - left - 2*inset
	if available < 48 {
		return fail
	}
	count = min(count, 128)
	length := float64(max(count, 1)*(p.IconPx+12) + 24)
	for _, w := range p.Widgets {
		if granted(w) {
			length += 88
		}
	}
	width := math.Min(length, math.Min(available, (right-left)*p.MaxLengthFraction))
	if width < 48 {
		return fail
	}
	b := domain.Bounds{X: left + (right-left-width)/2, Y: bottom - inset - float64(p.ThicknessPx), W: width, H: float64(p.ThicknessPx)}
	// At most one upward adjustment per region, repeated to a fixed bound for overlap chains.
	for range len(protected) + 1 {
		moved := false
		for _, r := range protected {
			if !validBounds(r) {
				return fail
			}
			if overlap(b, r) {
				b.Y = r.Y - inset - b.H
				moved = true
			}
		}
		if !moved {
			break
		}
	}
	if b.Y < top+inset || b.X < left || b.X+b.W > right {
		return fail
	}
	for _, r := range protected {
		if overlap(b, r) {
			return fail
		}
	}
	return Geometry{Bounds: b, RevealBand: domain.Bounds{X: b.X, Y: b.Y + b.H - 8, W: b.W, H: 8}, Status: "ready"}
}
