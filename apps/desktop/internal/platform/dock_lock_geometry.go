package platform

import (
	"math"
	"reflect"

	"option-tab/internal/domain"
)

type dockLockSegment struct{ X, Y, W, H, DX, DY float64 }

func dockLockEdges(displays []DockLockDisplay, edge string) (map[string][]dockLockSegment, bool) {
	if len(displays) == 0 || len(displays) > 32 || (edge != "bottom" && edge != "left" && edge != "right") {
		return nil, false
	}
	out := map[string][]dockLockSegment{}
	seen := map[string]bool{}
	for _, d := range displays {
		b := d.Bounds
		if d.UUID == "" || seen[d.UUID] || d.Mirrored || !dockLockFinite(b.X, b.Y, b.W, b.H) || b.W <= 0 || b.H <= 0 {
			return nil, false
		}
		seen[d.UUID] = true
	}
	for _, d := range displays {
		b := d.Bounds
		lo, hi := b.X, b.X+b.W
		if edge != "bottom" {
			lo, hi = b.Y, b.Y+b.H
		}
		spans := [][2]float64{{lo, hi}}
		for _, other := range displays {
			if other.UUID == d.UUID {
				continue
			}
			o := other.Bounds
			covers := false
			cutLo, cutHi := o.X, o.X+o.W
			switch edge {
			case "bottom":
				covers = o.Y <= b.Y+b.H+0.5 && o.Y+o.H > b.Y+b.H+0.5
			case "left":
				covers = o.X < b.X-0.5 && o.X+o.W >= b.X-0.5
				cutLo, cutHi = o.Y, o.Y+o.H
			case "right":
				covers = o.X <= b.X+b.W+0.5 && o.X+o.W > b.X+b.W+0.5
				cutLo, cutHi = o.Y, o.Y+o.H
			}
			if !covers {
				continue
			}
			next := [][2]float64{}
			for _, v := range spans {
				if cutHi <= v[0] || cutLo >= v[1] {
					next = append(next, v)
					continue
				}
				if cutLo > v[0] {
					next = append(next, [2]float64{v[0], cutLo})
				}
				if cutHi < v[1] {
					next = append(next, [2]float64{cutHi, v[1]})
				}
			}
			spans = next
		}
		for _, v := range spans {
			if v[1]-v[0] < 8 {
				continue
			}
			var seg dockLockSegment
			switch edge {
			case "bottom":
				seg = dockLockSegment{X: v[0] + 1, Y: b.Y + b.H - 3, W: v[1] - v[0] - 2, H: 4, DY: -6}
			case "left":
				seg = dockLockSegment{X: b.X - 1, Y: v[0] + 1, W: 4, H: v[1] - v[0] - 2, DX: 6}
			case "right":
				seg = dockLockSegment{X: b.X + b.W - 3, Y: v[0] + 1, W: 4, H: v[1] - v[0] - 2, DX: -6}
			}
			out[d.UUID] = append(out[d.UUID], seg)
		}
	}
	return out, true
}

func dockLockActual(displays []DockLockDisplay, edge string, b domain.Bounds) string {
	if !dockLockFinite(b.X, b.Y, b.W, b.H) || b.W <= 0 || b.H <= 0 {
		return ""
	}
	found := ""
	for _, d := range displays {
		if d.Mirrored {
			return ""
		}
		s := d.Bounds
		matches := false
		switch edge {
		case "bottom":
			matches = b.Y <= s.Y+s.H+3 && b.Y+b.H >= s.Y+s.H-3 && b.X >= s.X-3 && b.X+b.W <= s.X+s.W+3
		case "left":
			matches = b.X <= s.X+3 && b.X+b.W >= s.X-3 && b.Y >= s.Y-3 && b.Y+b.H <= s.Y+s.H+3
		case "right":
			matches = b.X <= s.X+s.W+3 && b.X+b.W >= s.X+s.W-3 && b.Y >= s.Y-3 && b.Y+b.H <= s.Y+s.H+3
		}
		if matches {
			if found != "" {
				return ""
			}
			found = d.UUID
		}
	}
	return found
}

func dockLockFinite(values ...float64) bool {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func dockLockInferredEdge(displays []DockLockDisplay, b domain.Bounds) string {
	edges := []string{"left", "right"}
	if b.W > b.H {
		edges = []string{"bottom"}
	}
	found := ""
	for _, edge := range edges {
		if dockLockActual(displays, edge, b) != "" {
			if found != "" {
				return ""
			}
			found = edge
		}
	}
	return found
}

func dockLockStateChanged(a, b DockMonitorLockState) bool {
	a.Sequence, b.Sequence = 0, 0
	a.ObservedAtMs, b.ObservedAtMs = 0, 0
	return !reflect.DeepEqual(a, b)
}

func dockLockPlacementPoint(s dockLockSegment, phase int) (x, y, dx, dy float64) {
	x, y = s.X+s.W/2, s.Y+s.H/2
	if phase == 0 {
		return x + s.DX*2, y + s.DY*2, 0, 0
	}
	return x, y, -s.DX / 2, -s.DY / 2
}
