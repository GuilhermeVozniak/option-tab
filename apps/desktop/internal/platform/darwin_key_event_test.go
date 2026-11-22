//go:build darwin

package platform

import "testing"

// Navigation and function keys arrive from the tap with a private-use or
// control character as their text. Forwarding that as a one-character key made
// the frontend append it to the search, which then matched nothing.
func TestKeyEventFromTapNamesNonPrintableKeys(t *testing.T) {
	for _, tc := range []struct {
		keycode uint16
		text    string
		want    string
	}{
		{115, "", "Home"},
		{119, "", "End"},
		{116, "", "PageUp"},
		{121, "", "PageDown"},
		{117, "", "Delete"},
		{97, "", "F6"},
		{122, "", "F1"},
		{76, "\x03", "Enter"},
		{114, "", "Unidentified"},
		{200, "\x7f", "Unidentified"},
		{200, "\x1b", "Unidentified"},
	} {
		if got := keyEventFromTap(tc.keycode, 0, tc.text).Key; got != tc.want {
			t.Errorf("keycode %d text %q: key = %q, want %q", tc.keycode, tc.text, got, tc.want)
		}
	}
}

func TestKeyEventFromTapKeepsPrintableText(t *testing.T) {
	for _, text := range []string{"a", "Z", " ", "é", "∑", "日", "😀", ""} {
		if got := keyEventFromTap(0, 0, text).Key; got != text {
			t.Errorf("text %q: key = %q", text, got)
		}
	}
}
