package cssval

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		want Style
	}{
		{"fill:#F9F,stroke:#333,stroke-width:4px", Style{Fill: "#f9f", Stroke: "#333", StrokeWidth: "4"}},
		{"background:red; border-color: blue ;color:white !important", Style{Fill: "red", Stroke: "blue", Color: "white"}},
		{"stroke-dasharray: 5 5,font-weight:bold", Style{Dash: "5 5", FontWeight: "bold"}},
		{`fill:"><script>,stroke:url(#x),stroke-width:99,width:3`, Style{}},
		{"", Style{}},
	}
	for _, tc := range tests {
		if got := Parse(tc.in); got != tc.want {
			t.Errorf("Parse(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestStyleOver(t *testing.T) {
	got := Style{Fill: "red"}.Over(Style{Fill: "blue", Stroke: "green", FontWeight: "bold"})
	if want := (Style{Fill: "red", Stroke: "green", FontWeight: "bold"}); got != want {
		t.Errorf("Over = %+v, want %+v", got, want)
	}
}
