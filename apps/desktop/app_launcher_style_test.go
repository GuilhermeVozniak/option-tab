package main

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"option-tab/internal/domain"
	"option-tab/internal/platform"
)

func TestLauncherProfileStyleRecreatesExactHost(t *testing.T) {
	a, _, q, panel := launcherIntegrationApp(t)
	first := launcherIntegrationVisible(t, a, q)
	settings := a.settingsSnapshot()
	appearance := &settings.ReplacementDock.Profiles[0].Appearance
	appearance.Material, appearance.Theme, appearance.CornerRadiusPx = "system", "light", 12
	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(string(raw)); err != nil {
		t.Fatal(err)
	}
	if a.GetLauncherState(first.Session).Visible {
		t.Fatal("profile edit retained the old host admission")
	}
	second := launcherIntegrationVisible(t, a, q)
	want := platform.LauncherPanelStyle{Material: "system", Theme: "light", CornerRadiusPx: 12}
	got := panel.style.Load()
	if second.Session == first.Session || second.Appearance != *appearance || got == nil || *got != want {
		t.Fatalf("session %d -> %d, appearance %+v, native style %+v", first.Session, second.Session, second.Appearance, got)
	}
}

type launcherStylePanel struct {
	calls []string
	style platform.LauncherPanelStyle
	err   error
}

func (p *launcherStylePanel) LauncherToken() uint64 { return 1 }
func (p *launcherStylePanel) Show(domain.Bounds) error {
	p.calls = append(p.calls, "show")
	return nil
}
func (p *launcherStylePanel) Hide() error { return nil }
func (p *launcherStylePanel) Close() error {
	p.calls = append(p.calls, "close")
	return nil
}

func (p *launcherStylePanel) SetLauncherStyle(style platform.LauncherPanelStyle) error {
	p.calls = append(p.calls, "style")
	p.style = style
	return p.err
}

type launcherStyleSource struct{ panel *launcherStylePanel }

func (s launcherStyleSource) CreateLauncherPanel(unsafe.Pointer, string) (platform.LauncherPanel, error) {
	s.panel.calls = append(s.panel.calls, "create")
	return s.panel, nil
}

func TestLauncherStylesHostBeforeShowAndClosesOnStyleFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "show", true: "style failure"}[fail], func(t *testing.T) {
			panel := &launcherStylePanel{}
			if fail {
				panel.err = errors.New("fixture style unavailable")
			}
			style := platform.LauncherPanelStyle{Material: "system", Theme: "light", CornerRadiusPx: 12}
			q := make(chan func(), 1)
			w := &dockFakeWindow{}
			w.native = unsafe.Pointer(new(int))
			d := newDockWindow(func(f func()) { q <- f }, func() nativeWindow { return w }, launcherPanelHostAdapter{source: launcherStyleSource{panel}, uuid: integrationDisplay, style: style})
			failed := false
			d.onFailure = func() { failed = true }
			d.show(domain.Bounds{X: 10, Y: 10, W: 400, H: 64})
			select {
			case f := <-q:
				f()
			case <-time.After(time.Second):
				t.Fatal("host work was not scheduled")
			}
			want := []string{"create", "style", "show"}
			if fail {
				want = []string{"create", "style", "close"}
			}
			if !reflect.DeepEqual(panel.calls, want) || panel.style != style || failed != fail {
				t.Fatalf("calls=%v style=%+v failed=%v", panel.calls, panel.style, failed)
			}
		})
	}
}
