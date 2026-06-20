package greeter

import "testing"

func TestDefaultGreeter_Greet(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty defaults to World", "", "Hello, World!"},
		{"uses provided name", "Gui", "Hello, Gui!"},
		{"trims whitespace", "  Ana  ", "Hello, Ana!"},
	}
	g := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Greet(tt.input); got != tt.want {
				t.Errorf("Greet(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
