//go:build darwin

package platform

import (
	"testing"

	"option-tab/internal/hotkey"
)

func TestNativeWindowIDArrayPreserves32BitID(t *testing.T) {
	const id = uint32(0x89abcdef)
	if got := nativeWindowIDArrayValue(id); got != uintptr(id) {
		t.Fatalf("native window-ID array value = 0x%x, want 0x%x", got, id)
	}
}

func TestFindPIDMatchesListedWindowOwner(t *testing.T) {
	p := &darwinPlatform{hotkeys: newDarwinHotkeys()}
	windows, err := p.Windows()
	if err != nil {
		t.Fatalf("Windows: %v", err)
	}
	for _, w := range windows {
		if w.ID == 0 || w.PID <= 0 {
			continue
		}
		if got := p.findPID(w.ID); got != w.PID {
			t.Fatalf("findPID(%d) = %d, want listed owner %d", w.ID, got, w.PID)
		}
		return
	}
	t.Skip("no owned windows available for native PID lookup")
}

// TestChordFromCapture locks the native shortcut-recorder translation: the
// event-tap capture (CGEvent modifier flags + virtual keycode) must map to the
// same canonical chord string the rest of the app uses. This is the fix that
// lets Command+Tab and the switcher's own chord be recorded.
func TestChordFromCapture(t *testing.T) {
	const (
		tabKey   = 48 // macKeycodes["tab"]
		graveKey = 50 // macKeycodes["grave"]
	)
	cases := []struct {
		name    string
		flags   uint64
		keycode uint16
		want    string
	}{
		{"command+tab", cgFlagCommand, tabKey, "command+tab"},
		{"option+tab", cgFlagOption, tabKey, "option+tab"},
		{"shift+command+grave", cgFlagShift | cgFlagCommand, graveKey, "shift+command+grave"},
		{"control+option+tab canonical order", cgFlagControl | cgFlagOption, tabKey, "control+option+tab"},
		{"escape cancel sentinel", cgFlagCommand, 0xFFFF, ""},
		{"no modifier is not a chord", 0, tabKey, ""},
		{"unknown keycode", cgFlagCommand, 999, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := chordFromCapture(c.flags, c.keycode); got != c.want {
				t.Errorf("chordFromCapture(0x%x, %d) = %q, want %q", c.flags, c.keycode, got, c.want)
			}
		})
	}
}

// TestModMask locks the chord-modifier -> CGEventFlags translation used when
// the app synthesizes/matches the global hotkey: each modifier must map to its
// corresponding CGEvent flag bit (option -> Alternate), and combinations OR
// together. Expected masks are built from the capture-side cgFlag* constants,
// which equal the kCGEventFlagMask* values modMask emits.
func TestModMask(t *testing.T) {
	mods := func(ms ...hotkey.Modifier) hotkey.ModSet {
		var s hotkey.ModSet
		for _, m := range ms {
			s = s.With(m)
		}
		return s
	}
	cases := []struct {
		name string
		set  hotkey.ModSet
		want uint64
	}{
		{"no modifiers", mods(), 0},
		{"option", mods(hotkey.ModOption), cgFlagOption},
		{"command+shift", mods(hotkey.ModCommand, hotkey.ModShift), cgFlagCommand | cgFlagShift},
		{"control+option", mods(hotkey.ModControl, hotkey.ModOption), cgFlagControl | cgFlagOption},
		{"all four", mods(hotkey.ModControl, hotkey.ModOption, hotkey.ModShift, hotkey.ModCommand), cgFlagControl | cgFlagOption | cgFlagShift | cgFlagCommand},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := modMask(hotkey.Chord{Mods: c.set}); got != c.want {
				t.Errorf("modMask(%s) = 0x%x, want 0x%x", c.name, got, c.want)
			}
		})
	}
}

func TestNativeHotkeyMatchRequiresExactModifiersAndPrefersExplicitChord(t *testing.T) {
	resetNativeHotkeys(t)
	const tabKey = uint16(48)

	registerNativeHotkey(1, cgFlagCommand, tabKey, false)
	registerNativeHotkey(1, cgFlagCommand, tabKey, true)
	registerNativeHotkey(2, cgFlagCommand|cgFlagShift, tabKey, false)
	registerNativeHotkey(2, cgFlagCommand|cgFlagShift, tabKey, true)

	if action, id := nativeHotkeyDecision(cgFlagCommand|cgFlagControl, tabKey, false, false); action != nativeHotkeyPass || id != 0 {
		t.Fatalf("modifier superset decision = (%d, %d), want pass", action, id)
	}
	if action, id := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyActivate || id != 1 {
		t.Fatalf("command+tab decision = (%d, %d), want activate shortcut 1", action, id)
	}
	if action, id := nativeHotkeyDecision(cgFlagCommand|cgFlagShift, tabKey, false, false); action != nativeHotkeyActivate || id != 2 {
		t.Fatalf("explicit shift chord decision = (%d, %d), want activate shortcut 2", action, id)
	}
}

func TestNativeHotkeyMatchUsesSyntheticShiftVariantForReverse(t *testing.T) {
	resetNativeHotkeys(t)
	const tabKey = uint16(48)
	registerNativeHotkey(1, cgFlagCommand, tabKey, false)
	registerNativeHotkey(1, cgFlagCommand, tabKey, true)

	if action, id := nativeHotkeyDecision(cgFlagCommand|cgFlagShift, tabKey, true, true); action != nativeHotkeyReverse || id != 1 {
		t.Fatalf("synthetic shift decision = (%d, %d), want reverse shortcut 1", action, id)
	}
}

func TestNativeHotkeyReleaseUsesBaseModifiersForSyntheticReverse(t *testing.T) {
	resetNativeHotkeys(t)
	const tabKey = uint16(48)
	registerNativeHotkey(1, cgFlagCommand, tabKey, false)
	registerNativeHotkey(1, cgFlagCommand, tabKey, true)

	action, id, holdMask := nativeHotkeyPressDecision(
		cgFlagCommand|cgFlagShift, tabKey, false, false,
	)
	if action != nativeHotkeyActivate || id != 1 {
		t.Fatalf("synthetic reverse press = (%d, %d), want activate shortcut 1", action, id)
	}
	if nativeHotkeyShouldRelease(cgFlagCommand, holdMask) {
		t.Fatal("releasing synthetic Shift must not release while Command remains held")
	}
	if !nativeHotkeyShouldRelease(cgFlagShift, holdMask) {
		t.Fatal("releasing the base Command modifier must release the shortcut")
	}

	unregisterNativeHotkey(1)
	registerNativeHotkey(2, cgFlagCommand|cgFlagShift, tabKey, false)
	_, _, holdMask = nativeHotkeyPressDecision(
		cgFlagCommand|cgFlagShift, tabKey, false, false,
	)
	if !nativeHotkeyShouldRelease(cgFlagCommand, holdMask) {
		t.Fatal("an explicit Shift chord must release when Shift is released")
	}
}

func TestNativeHotkeyPolicyPassesThroughDisabledAndIgnoredApps(t *testing.T) {
	resetNativeHotkeys(t)
	const tabKey = uint16(48)
	registerNativeHotkey(1, cgFlagCommand, tabKey, false)
	h := newDarwinHotkeys()

	h.setFrontApp("com.example.Editor", "Editor")
	h.SetHotkeyPolicy(HotkeyPolicy{Enabled: false})
	if action, _ := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyPass {
		t.Fatalf("disabled policy action = %d, want pass", action)
	}

	h.SetHotkeyPolicy(HotkeyPolicy{Enabled: true, IgnoredApps: []string{"com.game", "Virtual Machine"}})
	h.setFrontApp("COM.GAME", "Game")
	if action, _ := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyPass {
		t.Fatalf("bundle blacklist action = %d, want pass", action)
	}
	h.setFrontApp("org.example.vm", "virtual machine")
	if action, _ := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyPass {
		t.Fatalf("app-name blacklist action = %d, want pass", action)
	}

	h.setFrontApp("com.example.Editor", "Editor")
	if action, id := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyActivate || id != 1 {
		t.Fatalf("eligible policy decision = (%d, %d), want activate shortcut 1", action, id)
	}

	h.SetHotkeyPolicy(HotkeyPolicy{Enabled: false})
	if action, id := nativeHotkeyDecision(cgFlagCommand, tabKey, false, true); action != nativeHotkeyActivate || id != 1 {
		t.Fatalf("open switcher decision = (%d, %d), want existing session consumed", action, id)
	}
}

func TestNativeHotkeyPolicyUsesUnicodeCaseFolding(t *testing.T) {
	resetNativeHotkeys(t)
	const tabKey = uint16(48)
	registerNativeHotkey(1, cgFlagCommand, tabKey, false)
	h := newDarwinHotkeys()
	h.SetHotkeyPolicy(HotkeyPolicy{Enabled: true, IgnoredApps: []string{"Éditeur"}})
	h.setFrontApp("com.example.editor", "éditeur")

	if action, _ := nativeHotkeyDecision(cgFlagCommand, tabKey, false, false); action != nativeHotkeyPass {
		t.Fatalf("Unicode case-folded app-name match action = %d, want pass", action)
	}
}

func resetNativeHotkeys(t *testing.T) {
	t.Helper()
	for id := 1; id <= 9; id++ {
		unregisterNativeHotkey(id)
	}
	setNativeHotkeyEligibility(true)
	t.Cleanup(func() {
		for id := 1; id <= 9; id++ {
			unregisterNativeHotkey(id)
		}
		setNativeHotkeyEligibility(true)
	})
}
