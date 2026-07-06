//go:build darwin

package platform

import (
	"testing"

	"option-tab/internal/hotkey"
)

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
