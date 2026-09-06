package dock

import (
	"math"
	"testing"
	"time"

	"option-tab/internal/domain"
	"option-tab/internal/hotkey"
	"option-tab/internal/platform"
)

var inputStart = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func iconEvent(sequence uint64, kind platform.DockInputKind) platform.DockInputEvent {
	return platform.DockInputEvent{Sequence: sequence, Generation: 1, GestureID: 1, Timestamp: inputStart.Add(time.Duration(sequence-1) * 50 * time.Millisecond), Kind: kind, Owned: true, Item: platform.DockItem{AppID: 10, BundleID: "example.app", Path: "/Applications/Example.app", Bounds: domain.Bounds{X: 100, Y: 100, W: 48, H: 48}, ScreenID: 1, Edge: "bottom"}, PointerX: 120, PointerY: 120}
}

func iconReducer(policy platform.DockInputPolicy) *InputReducer {
	r := NewInputReducer(policy, 999)
	r.SetGeneration(1)
	return r
}

func TestIconInputExactOwnedClickActions(t *testing.T) {
	cmd := hotkey.ModSet(0).With(hotkey.ModCommand)
	for _, tc := range []struct {
		name     string
		down, up platform.DockInputKind
		button   int
		mods     hotkey.ModSet
		want     string
	}{
		{"left hide", platform.DockInputLeftDown, platform.DockInputLeftUp, 0, 0, "hide"},
		{"command quit", platform.DockInputRightDown, platform.DockInputRightUp, 1, cmd, "quit"},
		{"command option force quit", platform.DockInputRightDown, platform.DockInputRightUp, 1, cmd.With(hotkey.ModOption), "forceQuit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := iconReducer(platform.DockInputPolicy{ClickToHide: true, ModifiedRightClick: true})
			down := iconEvent(1, tc.down)
			down.Button = tc.button
			down.Modifiers = tc.mods
			if got := r.Step(down); len(got) != 0 {
				t.Fatal("action before release")
			}
			up := iconEvent(2, tc.up)
			up.Button = tc.button
			up.Modifiers = tc.mods
			got := r.Step(up)
			if len(got) != 1 || got[0].Kind != tc.want || got[0].AppID != 10 || got[0].Generation != 1 || got[0].GestureID != 1 {
				t.Fatalf("wrong exact action %+v", got)
			}
			up.Sequence++
			up.Timestamp = up.Timestamp.Add(time.Millisecond)
			if got := r.Step(up); len(got) != 0 {
				t.Fatal("duplicate release acted")
			}
		})
	}
}

func TestIconInputRejectsUnownedAndUnsafeIdentity(t *testing.T) {
	cases := map[string]func(*platform.DockInputEvent){
		"unowned":          func(e *platform.DockInputEvent) { e.Owned = false },
		"self":             func(e *platform.DockInputEvent) { e.Item.AppID = 999 },
		"invalid PID":      func(e *platform.DockInputEvent) { e.Item.AppID = 0 },
		"Finder":           func(e *platform.DockInputEvent) { e.Item.BundleID = "com.apple.finder" },
		"missing bundle":   func(e *platform.DockInputEvent) { e.Item.BundleID = "" },
		"missing path":     func(e *platform.DockInputEvent) { e.Item.Path = "" },
		"outside":          func(e *platform.DockInputEvent) { e.PointerX = 200 },
		"nonfinite":        func(e *platform.DockInputEvent) { e.PointerX = math.NaN() },
		"invalid bounds":   func(e *platform.DockInputEvent) { e.Item.Bounds.W = 0 },
		"wrong generation": func(e *platform.DockInputEvent) { e.Generation = 2 },
		"zero gesture":     func(e *platform.DockInputEvent) { e.GestureID = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := iconReducer(platform.DockInputPolicy{ClickToHide: true})
			d, u := iconEvent(1, platform.DockInputLeftDown), iconEvent(2, platform.DockInputLeftUp)
			mutate(&d)
			mutate(&u)
			if len(r.Step(d))+len(r.Step(u)) != 0 {
				t.Fatal("unsafe gesture acted")
			}
		})
	}
}

func TestIconInputExactModifiersAndDisabledPolicy(t *testing.T) {
	cmd := hotkey.ModSet(0).With(hotkey.ModCommand)
	for _, mods := range []hotkey.ModSet{0, hotkey.ModSet(0).With(hotkey.ModOption), cmd.With(hotkey.ModShift), cmd.With(hotkey.ModControl), cmd | 128} {
		r := iconReducer(platform.DockInputPolicy{ModifiedRightClick: true})
		d, u := iconEvent(1, platform.DockInputRightDown), iconEvent(2, platform.DockInputRightUp)
		d.Modifiers = mods
		u.Modifiers = mods
		d.Button = 1
		u.Button = 1
		if len(r.Step(d))+len(r.Step(u)) != 0 {
			t.Fatalf("unsupported modifiers %d acted", mods)
		}
	}
	r := iconReducer(platform.DockInputPolicy{})
	if len(r.Step(iconEvent(1, platform.DockInputLeftDown)))+len(r.Step(iconEvent(2, platform.DockInputLeftUp))) != 0 {
		t.Fatal("disabled click acted")
	}
	r = iconReducer(platform.DockInputPolicy{ClickToHide: true})
	d, u := iconEvent(1, platform.DockInputLeftDown), iconEvent(2, platform.DockInputLeftUp)
	d.Modifiers = cmd
	u.Modifiers = cmd
	if len(r.Step(d))+len(r.Step(u)) != 0 {
		t.Fatal("modified left click acted")
	}
}

func TestIconInputClickCancellationCannotBeUndone(t *testing.T) {
	for _, reason := range []string{"slop", "outside", "cancelled", "unowned", "identity", "path", "bundle", "modifiers", "button", "timestamp"} {
		t.Run(reason, func(t *testing.T) {
			r := iconReducer(platform.DockInputPolicy{ClickToHide: true})
			r.Step(iconEvent(1, platform.DockInputLeftDown))
			e := iconEvent(2, platform.DockInputMove)
			switch reason {
			case "slop":
				e.PointerX += 7
			case "outside":
				e.PointerY = 200
			case "cancelled":
				e.Kind = platform.DockInputCancelled
			case "unowned":
				e.Owned = false
			case "identity":
				e.Item.AppID = 11
			case "path":
				e.Item.Path = "/Other.app"
			case "bundle":
				e.Item.BundleID = "other.app"
			case "modifiers":
				e.Modifiers = hotkey.ModSet(0).With(hotkey.ModShift)
			case "button":
				e.Button = 1
			case "timestamp":
				e.Timestamp = inputStart.Add(-time.Second)
			}
			if len(r.Step(e)) != 0 || len(r.Step(iconEvent(3, platform.DockInputLeftUp))) != 0 {
				t.Fatal("cancelled click acted")
			}
		})
	}
}

func scrollEvent(sequence uint64, delta float64, precise bool) platform.DockInputEvent {
	e := iconEvent(sequence, platform.DockInputScroll)
	e.DeltaY = delta
	e.Precise = precise
	return e
}

func TestIconInputScrollThresholdDirectionAndMomentum(t *testing.T) {
	for _, direction := range []float64{1, -1} {
		for _, precise := range []bool{true, false} {
			r := iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
			delta := 0.25 * direction
			if precise {
				delta = 20 * direction
			}
			for i := 1; i <= 4; i++ {
				e := scrollEvent(uint64(i), delta, precise)
				got := r.Step(e)
				if i < 4 && len(got) != 0 {
					t.Fatal("below threshold acted")
				}
				if i == 4 {
					want := "show"
					if direction < 0 {
						want = "hide"
					}
					if len(got) != 1 || got[0].Kind != want || got[0].AppID != 10 {
						t.Fatalf("wrong scroll action %+v", got)
					}
				}
			}
			e := scrollEvent(5, 100*direction, precise)
			e.MomentumPhase = "changed"
			if len(r.Step(e)) != 0 {
				t.Fatal("momentum repeat acted")
			}
			e.Sequence++
			e.Timestamp = e.Timestamp.Add(time.Second)
			if len(r.Step(e)) != 0 {
				t.Fatal("old gesture revived after idle")
			}
		}
	}
}

func TestIconInputScrollReversalAndVisualNormalization(t *testing.T) {
	r := iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
	for i, delta := range []float64{60, -20, -59} {
		if len(r.Step(scrollEvent(uint64(i+1), delta, true))) != 0 {
			t.Fatal("reversal retained prior accumulation")
		}
	}
	e := scrollEvent(4, -1, true)
	e.DirectionInverted = true // Source already normalized: no second inversion.
	got := r.Step(e)
	if len(got) != 1 || got[0].Kind != "hide" {
		t.Fatalf("visual delta inverted twice: %+v", got)
	}
}

func TestIconInputScrollRearmAndTerminalPhases(t *testing.T) {
	for _, terminal := range []string{"ended", "cancelled", "momentum"} {
		t.Run(terminal, func(t *testing.T) {
			r := iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
			r.Step(scrollEvent(1, 60, true))
			e := scrollEvent(2, 0, true)
			if terminal == "momentum" {
				e.MomentumPhase = "began"
			} else {
				e.Phase = terminal
			}
			r.Step(e)
			if len(r.Step(scrollEvent(3, 100, true))) != 0 {
				t.Fatal("terminal stream revived")
			}
			e = scrollEvent(4, 80, true)
			e.GestureID = 2
			e.Phase = "began"
			if len(r.Step(e)) != 1 {
				t.Fatal("fresh phased stream failed")
			}
		})
	}
	r := iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
	if len(r.Step(scrollEvent(1, 1, false))) != 1 {
		t.Fatal("coarse notch failed")
	}
	e := scrollEvent(2, 1, false)
	e.GestureID = 2
	if len(r.Step(e)) != 0 {
		t.Fatal("legacy stream rearmed before idle")
	}
	e = scrollEvent(3, 1, false)
	e.GestureID = 3
	e.Timestamp = inputStart.Add(301 * time.Millisecond)
	if len(r.Step(e)) != 1 {
		t.Fatal("legacy idle rearm failed")
	}
}

func TestIconInputConfigurationGenerationAndStaleReplay(t *testing.T) {
	policy := platform.DockInputPolicy{ClickToHide: true}
	r := iconReducer(policy)
	r.Step(iconEvent(1, platform.DockInputLeftDown))
	r.Configure(platform.DockInputPolicy{})
	r.Configure(policy)
	if len(r.Step(iconEvent(2, platform.DockInputLeftUp))) != 0 {
		t.Fatal("configuration revived click")
	}
	r.SetGeneration(2)
	r.SetGeneration(1)
	if len(r.Step(iconEvent(3, platform.DockInputLeftDown))) != 0 || len(r.Step(iconEvent(4, platform.DockInputLeftUp))) != 0 {
		t.Fatal("old generation acted")
	}
	d, u := iconEvent(1, platform.DockInputLeftDown), iconEvent(2, platform.DockInputLeftUp)
	d.Generation = 2
	u.Generation = 2
	r.Step(d)
	if len(r.Step(u)) != 1 {
		t.Fatal("new generation failed")
	}
	d.Sequence = 3
	u.Sequence = 4
	r.Step(d)
	if len(r.Step(u)) != 0 {
		t.Fatal("same gesture replayed")
	}
	d.GestureID = 2
	d.Sequence = 5
	r.Step(d)
	r.Reset()
	u.GestureID = 2
	u.Sequence = 6
	if len(r.Step(u)) != 0 {
		t.Fatal("reset revived click")
	}
}

func TestIconInputReleasedCancellationAndScrollIdleTimer(t *testing.T) {
	r := iconReducer(platform.DockInputPolicy{ClickToHide: true})
	r.Step(iconEvent(1, platform.DockInputLeftDown))
	e := iconEvent(2, platform.DockInputCancelled)
	e.Owned = false
	e.Item = platform.DockItem{}
	r.Step(e)
	if len(r.Step(iconEvent(3, platform.DockInputLeftUp))) != 0 {
		t.Fatal("released cancellation ignored")
	}
	r = iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
	r.Step(scrollEvent(1, 1, false))
	e = scrollEvent(2, 1, false)
	e.Timestamp = inputStart.Add(240 * time.Millisecond)
	e.MomentumPhase = "changed"
	r.Step(e)
	e = scrollEvent(3, 1, false)
	e.GestureID = 2
	e.Timestamp = inputStart.Add(300 * time.Millisecond)
	if len(r.Step(e)) != 0 {
		t.Fatal("momentum failed to refresh idle timer")
	}
	e = scrollEvent(4, 1, false)
	e.GestureID = 3
	e.Timestamp = inputStart.Add(551 * time.Millisecond)
	if len(r.Step(e)) != 1 {
		t.Fatal("fresh stream after idle failed")
	}
}

func TestIconInputClickSlopBoundaryAndScrollInvalidSamples(t *testing.T) {
	r := iconReducer(platform.DockInputPolicy{ClickToHide: true})
	r.Step(iconEvent(1, platform.DockInputLeftDown))
	e := iconEvent(2, platform.DockInputLeftUp)
	e.PointerX += platform.DockInputClickSlop
	if len(r.Step(e)) != 1 {
		t.Fatal("click at slop boundary rejected")
	}
	for _, mutate := range []func(*platform.DockInputEvent){
		func(e *platform.DockInputEvent) { e.DeltaY = math.Inf(1) },
		func(e *platform.DockInputEvent) { e.DeltaX = 100 },
		func(e *platform.DockInputEvent) { e.Modifiers = hotkey.ModSet(0).With(hotkey.ModCommand) },
		func(e *platform.DockInputEvent) { e.Phase = "unknown" },
		func(e *platform.DockInputEvent) { e.Owned = false },
	} {
		r = iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
		r.Step(scrollEvent(1, 60, true))
		e = scrollEvent(2, 40, true)
		mutate(&e)
		if len(r.Step(e))+len(r.Step(scrollEvent(3, 80, true))) != 0 {
			t.Fatal("invalid scroll evidence acted")
		}
	}
	r = iconReducer(platform.DockInputPolicy{ScrollShowHide: true})
	r.Step(scrollEvent(1, 60, true))
	e = scrollEvent(2, 20, true)
	e.Timestamp = inputStart.Add(251 * time.Millisecond)
	if len(r.Step(e)) != 0 {
		t.Fatal("expired partial accumulation retained")
	}
}
