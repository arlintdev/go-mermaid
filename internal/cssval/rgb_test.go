package cssval

import "testing"

func TestRGB(t *testing.T) {
	tests := []struct {
		in      string
		r, g, b uint8
		ok      bool
	}{
		{"#fff", 255, 255, 255, true},
		{"#336699cc", 0x33, 0x66, 0x99, true},
		{"rgb(10, 20, 30)", 10, 20, 30, true},
		{"rgba(100%, 0%, 50%, 0.5)", 255, 0, 128, true},
		{"hsl(120, 100%, 25%)", 0, 128, 0, true},
		{"RebeccaPurple", 0x66, 0x33, 0x99, true},
		{"transparent", 0, 0, 0, false},
		{"none", 0, 0, 0, false},
		{"#12", 0, 0, 0, false},
		{"url(#x)", 0, 0, 0, false},
	}
	for _, tc := range tests {
		r, g, b, ok := RGB(tc.in)
		if ok != tc.ok || r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("RGB(%q) = %d,%d,%d,%v; want %d,%d,%d,%v", tc.in, r, g, b, ok, tc.r, tc.g, tc.b, tc.ok)
		}
	}
}
