//go:build darwin

package platform

import "testing"

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
