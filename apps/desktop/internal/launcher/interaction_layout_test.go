package launcher

import (
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func TestInteractionKeyboardReservesAllEdges(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge = edge
		p.Widgets = nil
		d := env().Displays[0]
		base := Layout(p, d, 1, nil)
		p.Interactions.Enabled = true
		p.Interactions.LetterNavigation = true
		g := Layout(p, d, 1, nil)
		along, original := g.Bounds.W, base.Bounds.W
		if edge == "left" || edge == "right" {
			along, original = g.Bounds.H, base.Bounds.H
		}
		if g.Status != "ready" || along != original+28 {
			t.Fatal("keyboard control has no reserved extent", edge, base, g)
		}
		protected := []domain.Bounds{g.Bounds}
		moved := Layout(p, d, 1, protected)
		if moved.Status != "ready" || overlap(moved.Bounds, g.Bounds) {
			t.Fatal("control reservation escaped protected geometry", edge, moved)
		}
		p.Layout = "fullWidth"
		full := Layout(p, d, 1, nil)
		if full.Status != "ready" {
			t.Fatal(edge, full)
		}
		p.Interactions.Enabled = false
		if without := Layout(p, d, 1, nil); without != full {
			t.Fatal("full-width host changed beyond existing interior")
		}
	}
}

func TestInteractionKeyboardMagnificationFitsRemainingIconArea(t *testing.T) {
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge = edge
		p.Widgets = nil
		p.MaxLengthFraction = .5
		p.Interactions.Enabled = true
		p.Interactions.LetterNavigation = true
		p.Magnification = &config.LauncherMagnification{Enabled: true, Scale: 2, Reach: 4}
		d := env().Displays[0]
		d.Frame = domain.Bounds{X: -200, Y: -300, W: 300, H: 300}
		d.UsableFrame = d.Frame
		g := Layout(p, d, 1, nil)
		along := g.Bounds.W
		if edge == "left" || edge == "right" {
			along = g.Bounds.H
		}
		inset := 12.0
		if g.Magnification.Enabled {
			inset = g.Magnification.PrimaryInset
		}
		if g.Status != "ready" || along < 28+2*inset+float64(p.IconPx) {
			t.Fatal("magnification consumed keyboard or minimum icon slot", edge, g)
		}
		p.MaxLengthFraction = .25
		if tiny := Layout(p, d, 1, nil); tiny.Status == "ready" {
			t.Fatal("impossible keyboard/icon host admitted", edge, tiny)
		}
	}
}
