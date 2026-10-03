package launcher

import (
	"option-tab/internal/config"
	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

// ResolvedMagnification describes a maximum visual envelope, never transient
// pointer state. Insets are absolute host-to-stable-icon edges in logical px.
// The renderer bounds aggregate extra width by I*(Scale-1)*(2*Reach+1), including
// unsettled springs, and centers cumulative displacement within PrimaryInset.
type ResolvedMagnification struct {
	Enabled      bool    `json:"enabled"`
	Scale        float64 `json:"scale"`
	Reach        int     `json:"reach"`
	PrimaryInset float64 `json:"primaryInset"`
	CrossInset   float64 `json:"crossInset"`
}

// Layout reduces requested magnification conservatively with bounded work.
// Scale1 preserves the pre-H06 bounds and reveal band exactly.
func Layout(p config.LauncherProfile, d platform.LauncherDisplay, count int, protected []domain.Bounds) Geometry {
	m := p.Magnification
	if m == nil || !m.Enabled || m.Scale <= 1 || count == 0 {
		return layoutAtMagnification(p, d, count, protected, 1, 0)
	}
	for step := 0; step <= 64; step++ {
		scale := 1 + (m.Scale-1)*float64(64-step)/64
		g := layoutAtMagnification(p, d, count, protected, scale, m.Reach)
		if g.Status == "ready" {
			return g
		}
	}
	return Geometry{Status: "unavailable"}
}
