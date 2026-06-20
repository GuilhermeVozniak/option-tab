package domain

import "testing"

func TestBounds_Area(t *testing.T) {
	tests := []struct {
		name string
		b    Bounds
		want float64
	}{
		{"unit", Bounds{0, 0, 1, 1}, 1},
		{"rect", Bounds{10, 20, 4, 3}, 12},
		{"zero width", Bounds{0, 0, 0, 5}, 0},
		{"negative dims clamp to zero", Bounds{0, 0, -4, 3}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.Area(); got != tt.want {
				t.Errorf("Area() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBounds_Center(t *testing.T) {
	b := Bounds{X: 10, Y: 20, W: 4, H: 6}
	cx, cy := b.Center()
	if cx != 12 || cy != 23 {
		t.Errorf("Center() = (%v,%v), want (12,23)", cx, cy)
	}
}

func TestBounds_ContainsPoint(t *testing.T) {
	b := Bounds{X: 0, Y: 0, W: 10, H: 10}
	tests := []struct {
		name string
		x, y float64
		want bool
	}{
		{"inside", 5, 5, true},
		{"top-left edge", 0, 0, true},
		{"right edge excluded", 10, 5, false},
		{"bottom edge excluded", 5, 10, false},
		{"outside", -1, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := b.ContainsPoint(tt.x, tt.y); got != tt.want {
				t.Errorf("ContainsPoint(%v,%v) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestBounds_Intersects(t *testing.T) {
	a := Bounds{X: 0, Y: 0, W: 10, H: 10}
	tests := []struct {
		name string
		b    Bounds
		want bool
	}{
		{"overlap", Bounds{5, 5, 10, 10}, true},
		{"contained", Bounds{2, 2, 2, 2}, true},
		{"touching edge only", Bounds{10, 0, 5, 5}, false},
		{"separate", Bounds{100, 100, 5, 5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.Intersects(tt.b); got != tt.want {
				t.Errorf("Intersects() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWindow_IsVisible(t *testing.T) {
	tests := []struct {
		name string
		w    Window
		want bool
	}{
		{"normal on-screen window", Window{OnScreen: true}, true},
		{"minimized", Window{OnScreen: false, Minimized: true}, false},
		{"hidden app", Window{OnScreen: false, Hidden: true}, false},
		{"off-screen but not minimized/hidden (other space)", Window{OnScreen: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.IsVisible(); got != tt.want {
				t.Errorf("IsVisible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBounds_Area_NegativeHeight(t *testing.T) {
	if got := (Bounds{0, 0, 4, -3}).Area(); got != 0 {
		t.Errorf("Area() with negative height = %v, want 0", got)
	}
}
