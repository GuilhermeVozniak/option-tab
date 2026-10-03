package launcher

import (
	"testing"

	"option-tab/internal/domain"

	"option-tab/internal/config"
)

func TestMagnificationReservesTransformedEnvelope(t *testing.T) {
	p := config.DefaultReplacementDock().Profiles[0]
	p.Magnification = &config.LauncherMagnification{Enabled: true, Scale: 2, Reach: 4}
	d := env().Displays[0]
	g := Layout(p, d, 3, nil)
	if g.Status != "ready" || !g.Magnification.Enabled || g.Magnification.Scale != 2 || g.Magnification.PrimaryInset != 192 || g.Magnification.CrossInset < 24 || g.Bounds.H < 88 {
		t.Fatal("transforms escape native envelope", g)
	}
}

func TestMagnificationAllEdgesNegativeOriginsAndLogicalScale(t *testing.T) {
	d := env().Displays[0]
	d.Frame = domain.Bounds{X: -1200, Y: -700, W: 1000, H: 900}
	d.UsableFrame = domain.Bounds{X: -1200, Y: -675, W: 1000, H: 875}
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge = edge
		p.Magnification = &config.LauncherMagnification{Enabled: true, Scale: 2, Reach: 4}
		g := Layout(p, d, 10, nil)
		if g.Status != "ready" {
			t.Fatal(edge, g)
		}
		b, m := g.Bounds, g.Magnification
		if b.X < d.Frame.X+32 || b.Y < d.Frame.Y+32 || b.X+b.W > d.Frame.X+d.Frame.W-32 || b.Y+b.H > d.Frame.Y+d.Frame.H-32 {
			t.Fatal("recovery edge covered", edge, g)
		}
		primary, cross := b.W, b.H
		if edge == "left" || edge == "right" {
			primary, cross = cross, primary
		}
		if primary < 2*m.PrimaryInset+float64(p.IconPx) || cross < 2*m.CrossInset+float64(p.IconPx) {
			t.Fatal("insufficient transformed envelope", edge, g)
		}
		if !contains(b, g.RevealBand.X+g.RevealBand.W/2, g.RevealBand.Y+g.RevealBand.H/2) {
			t.Fatal("reveal escaped envelope")
		}
		d.Scale = 2.5
		scaled := Layout(p, d, 10, nil)
		d.Scale = 1
		if g != scaled {
			t.Fatal("logical geometry changed with backing scale", edge)
		}
		protected := []domain.Bounds{g.Bounds}
		moved := Layout(p, d, 10, protected)
		if moved.Status != "ready" || overlap(moved.Bounds, g.Bounds) {
			t.Fatal("native Dock protection lost", edge, moved)
		}
	}
}

func TestMagnificationReducesAndFallsBackWithoutClipping(t *testing.T) {
	p := config.DefaultReplacementDock().Profiles[0]
	p.IconPx = 64
	p.ThicknessPx = 72
	p.Magnification = &config.LauncherMagnification{Enabled: true, Scale: 2, Reach: 4}
	d := env().Displays[0]
	d.Frame = domain.Bounds{X: -300, Y: -100, W: 250, H: 250}
	d.UsableFrame = d.Frame
	for _, edge := range []string{"top", "bottom", "left", "right"} {
		p.Edge = edge
		g := Layout(p, d, 5, nil)
		if g.Status != "ready" || g.Magnification.Scale < 1 || g.Magnification.Scale >= 2 {
			t.Fatal("did not reduce", edge, g)
		}
	}
	d.Frame.W = 192
	d.Frame.H = 192
	d.UsableFrame = d.Frame
	reduced := Layout(p, d, 5, nil)
	p.Magnification.Enabled = false
	off := Layout(p, d, 5, nil)
	if reduced.Status != "ready" || reduced.Magnification.Enabled || reduced.Magnification.Scale != 1 || reduced.Bounds != off.Bounds || reduced.RevealBand != off.RevealBand {
		t.Fatal("scale1 did not preserve legacy geometry", reduced, off)
	}
	p.Magnification.Enabled = true
	if Layout(p, d, 5, []domain.Bounds{d.Frame}).Status != "unavailable" {
		t.Fatal("protected whole display accepted")
	}
}

func TestMagnificationReservesCollapsedGroupMembersAndPadding(t *testing.T) {
	p := config.DefaultReplacementDock().Profiles[0]
	p.Magnification.Enabled = true
	d := env().Displays[0]
	plain := Layout(p, d, 1, nil)
	p.Items = []config.LauncherItem{{ID: "a", Kind: "app", Label: "A", ReferenceID: "selection-a"}, {ID: "b", Kind: "app", Label: "B", ReferenceID: "selection-b"}, {ID: "group", Kind: "group", Label: "Group", Members: []string{"a", "b"}}}
	group := Layout(p, d, 1, nil)
	if group.Bounds.W-plain.Bounds.W != 2*float64(p.IconPx+p.Appearance.ItemSpacingPx)+9 {
		t.Fatal("collapsed group expansion omitted", plain, group)
	}
	if a, b := Layout(p, d, 144, nil), Layout(p, d, 10000, nil); a != b {
		t.Fatal("unbounded app count")
	}
}

func TestMagnificationDisabledPreservesOldGeometryAndPresentationResolved(t *testing.T) {
	p := config.DefaultReplacementDock().Profiles[0]
	d := env().Displays[0]
	old := p
	old.Magnification = nil
	if Layout(p, d, 5, nil) != Layout(old, d, 5, nil) {
		t.Fatal("default changed legacy layout")
	}
	c := prepared(t)
	c.settings.Profiles[0].Magnification = &config.LauncherMagnification{Enabled: true, Scale: 1.5, Reach: 1}
	c.reconcileLocked()
	state := c.Snapshot()
	if len(state.Presentations) == 0 || !state.Presentations[0].Magnification.Enabled || state.Presentations[0].Magnification.Scale > 1.5 {
		t.Fatal("resolved DTO absent")
	}
}
