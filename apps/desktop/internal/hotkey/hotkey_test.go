package hotkey

import "testing"

func TestParse_Valid(t *testing.T) {
	tests := []struct {
		in      string
		wantStr string
		mods    []Modifier
		key     Key
	}{
		{"option+tab", "option+tab", []Modifier{ModOption}, "tab"},
		{"Option+Tab", "option+tab", []Modifier{ModOption}, "tab"},
		{" cmd + shift + grave ", "shift+command+grave", []Modifier{ModCommand, ModShift}, "grave"},
		{"control+option+a", "control+option+a", []Modifier{ModControl, ModOption}, "a"},
		{"alt+tab", "option+tab", []Modifier{ModOption}, "tab"},           // alt is an alias for option
		{"super+space", "command+space", []Modifier{ModCommand}, "space"}, // super alias for command
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			c, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.in, err)
			}
			if c.Key != tt.key {
				t.Errorf("key = %q, want %q", c.Key, tt.key)
			}
			for _, m := range tt.mods {
				if !c.Mods.Has(m) {
					t.Errorf("expected modifier %v in %v", m, c.Mods)
				}
			}
			if got := c.String(); got != tt.wantStr {
				t.Errorf("String() = %q, want %q", got, tt.wantStr)
			}
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	for _, in := range []string{"", "tab", "option+", "option+option", "ctrl+notakey", "+tab", "option+a+b"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil error, want error", in)
		}
	}
}

func TestString_CanonicalModifierOrder(t *testing.T) {
	// Regardless of input order, modifiers serialize control+option+shift+command.
	c, err := Parse("command+shift+option+control+a")
	if err != nil {
		t.Fatal(err)
	}
	if got := c.String(); got != "control+option+shift+command+a" {
		t.Errorf("String() = %q, want canonical order", got)
	}
}

func TestChord_RequiresModifier(t *testing.T) {
	if _, err := Parse("a"); err == nil {
		t.Error("a chord without a modifier should be rejected")
	}
}

func TestModSet_Ops(t *testing.T) {
	var s ModSet
	s = s.With(ModOption).With(ModShift)
	if !s.Has(ModOption) || !s.Has(ModShift) || s.Has(ModCommand) {
		t.Errorf("ModSet ops wrong: %v", s)
	}
	if s.Without(ModShift).Has(ModShift) {
		t.Error("Without did not clear modifier")
	}
}

func TestParse_RoundTripsThroughString(t *testing.T) {
	for _, in := range []string{"option+tab", "control+option+shift+command+grave", "command+space"} {
		c, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		c2, err := Parse(c.String())
		if err != nil {
			t.Fatalf("re-parse(%q): %v", c.String(), err)
		}
		if c.String() != c2.String() {
			t.Errorf("round-trip mismatch %q -> %q -> %q", in, c.String(), c2.String())
		}
	}
}
