package platform

import "testing"

func TestApplicationPresenceRequiresCorroboratedAbsence(t *testing.T) {
	for _, test := range []struct {
		name            string
		valid, readable bool
		ax, cg          int
		want            WindowPresence
	}{
		{"actual AX root", true, true, 1, 9, WindowsPresent},
		{"empty AX with unresolved off-Space or menu surface", true, true, 0, 8, WindowsUnknown},
		{"both inventories empty", true, true, 0, 0, WindowsNone},
		{"AX denial", true, false, 0, 0, WindowsUnknown},
		{"CG failure", true, true, 0, -1, WindowsUnknown},
		{"identity changed", false, true, 1, 1, WindowsUnknown},
		{"malformed AX", true, true, -1, 0, WindowsUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := applicationWindowPresence(test.valid, test.readable, test.ax, test.cg)
			if got != test.want {
				t.Fatalf("presence=%q want %q", got, test.want)
			}
		})
	}
}
