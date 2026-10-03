package launcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func TestProfileGeometryAllEdgesAlignmentsAndFullWidth(t *testing.T) {
	d := env().Displays[0]
	d.Frame = domain.Bounds{X: -700, Y: -100, W: 700, H: 1100}
	d.UsableFrame = domain.Bounds{X: -700, Y: -76, W: 700, H: 1076}
	for _, edge := range []string{"bottom", "top", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge = edge
		vertical := edge == "left" || edge == "right"
		last := -1e9
		for _, alignment := range []string{"start", "center", "end"} {
			p.Alignment = alignment
			g := Layout(p, d, 2, nil)
			if g.Status != "ready" {
				t.Fatalf("%s/%s unavailable", edge, alignment)
			}
			along := g.Bounds.X
			if vertical {
				along = g.Bounds.Y
			}
			if along <= last {
				t.Fatalf("%s alignment not ordered", edge)
			}
			last = along
			if g.Bounds.X < d.UsableFrame.X || g.Bounds.Y < d.UsableFrame.Y || g.Bounds.X+g.Bounds.W > d.UsableFrame.X+d.UsableFrame.W || g.Bounds.Y+g.Bounds.H > d.UsableFrame.Y+d.UsableFrame.H {
				t.Fatal("escaped usable frame")
			}
			if !contains(g.Bounds, g.RevealBand.X+g.RevealBand.W/2, g.RevealBand.Y+g.RevealBand.H/2) {
				t.Fatal("reveal outside host")
			}
		}
		p.Layout = "fullWidth"
		a := Layout(p, d, 1, nil)
		b := Layout(p, d, 128, nil)
		if a.Status != "ready" || a.Bounds != b.Bounds {
			t.Fatal("full width depends on content")
		}
	}
}

func TestProfileGeometryProtectedBoundsAndSpacing(t *testing.T) {
	d := env().Displays[0]
	for _, edge := range []string{"bottom", "top", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge = edge
		first := Layout(p, d, 2, nil)
		if first.Status != "ready" {
			t.Fatal(edge)
		}
		next := Layout(p, d, 2, []domain.Bounds{first.Bounds})
		if next.Status != "ready" || overlap(next.Bounds, first.Bounds) {
			t.Fatalf("protected %s overlapped", edge)
		}
		p.Appearance.ItemSpacingPx = 20
		spaced := Layout(p, d, 2, nil)
		if edge == "top" || edge == "bottom" {
			if spaced.Bounds.W <= first.Bounds.W {
				t.Fatal("spacing ignored")
			}
		} else if spaced.Bounds.H <= first.Bounds.H {
			t.Fatal("vertical spacing ignored")
		}
		if Layout(p, d, 2, []domain.Bounds{{W: -1, H: 2}}).Status == "ready" {
			t.Fatal("invalid protection accepted")
		}
	}
}

func TestResolvedProfileAppearanceIsCopiedToPresentation(t *testing.T) {
	c := prepared(t)
	c.settings.Profiles[0].Edge = "left"
	c.settings.Profiles[0].Layout = "fullWidth"
	c.settings.Profiles[0].Appearance.Theme = "dark"
	c.reconcileLocked()
	p := c.Snapshot().Presentations[0]
	if p.Edge != "left" || p.Layout != "fullWidth" || p.Appearance.Theme != "dark" {
		t.Fatal("profile styling not resolved")
	}
}

func TestProfileGeometryLogicalScaleOverflowAndImpossibleProtection(t *testing.T) {
	d := env().Displays[0]
	p := config.DefaultReplacementDock().Profiles[0]
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		p.Edge = edge
		d.Scale = 1
		one := Layout(p, d, 128, nil)
		d.Scale = 2.5
		scaled := Layout(p, d, 128, nil)
		if one.Status != "ready" || one.Bounds != scaled.Bounds {
			t.Fatalf("%s logical coordinates depend on scale", edge)
		}
		b, r := one.Bounds, one.RevealBand
		switch edge {
		case "top":
			if r.Y != b.Y || r.H != 8 {
				t.Fatal("wrong top reveal")
			}
		case "bottom":
			if r.Y+r.H != b.Y+b.H || r.H != 8 {
				t.Fatal("wrong bottom reveal")
			}
		case "left":
			if r.X != b.X || r.W != 8 {
				t.Fatal("wrong left reveal")
			}
		case "right":
			if r.X+r.W != b.X+b.W || r.W != 8 {
				t.Fatal("wrong right reveal")
			}
		}
		if Layout(p, d, 128, []domain.Bounds{d.Frame}).Status == "ready" {
			t.Fatal("impossible protection admitted")
		}
	}
}
