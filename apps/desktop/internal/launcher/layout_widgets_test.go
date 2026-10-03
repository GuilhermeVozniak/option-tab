package launcher

import (
	"strings"
	"testing"

	"option-tab/internal/config"
	"option-tab/internal/domain"
)

func TestLayoutReservesEnabledWidgetSlotsOnBothAxes(t *testing.T) {
	d := env().Displays[0]
	d.Frame, d.UsableFrame = domain.Bounds{W: 3440, H: 3440}, domain.Bounds{Y: 25, W: 3440, H: 3415}
	for _, edge := range []string{"bottom", "top", "left", "right"} {
		p := config.DefaultReplacementDock().Profiles[0]
		p.Edge, p.Widgets = edge, nil
		base := Layout(p, d, 1, nil)
		p.Widgets = []config.WidgetInstance{{ID: "audio", PackageID: "com.fixture.audio", Digest: strings.Repeat("a", 64), Enabled: true}, {ID: "power", PackageID: "com.fixture.power", Digest: strings.Repeat("b", 64), Enabled: false}}
		one := Layout(p, d, 1, nil)
		length := func(g Geometry) float64 {
			if edge == "left" || edge == "right" {
				return g.Bounds.H
			}
			return g.Bounds.W
		}
		if got := length(one) - length(base); got != 160+float64(p.Appearance.ItemSpacingPx) {
			t.Fatalf("%s widget space=%v", edge, got)
		}
		p.Widgets[1].Enabled = true
		two := Layout(p, d, 1, nil)
		if length(two)-length(one) != length(one)-length(base) {
			t.Fatal("second widget slot not reserved")
		}
		p.Stacks = []config.WidgetStack{{ID: "status", Name: "Status", Members: []string{"audio", "power"}, ActiveID: "audio"}}
		stack := Layout(p, d, 1, nil)
		if stack.Bounds != one.Bounds {
			t.Fatal("hidden stack member reserved an additional slot")
		}
	}
}
