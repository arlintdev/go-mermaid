package curve

import (
	"strings"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

func TestEdge(t *testing.T) {
	a, z := domain.Point{X: 0, Y: 0}, domain.Point{X: 100, Y: 100}
	tests := []struct {
		name       string
		pts        []domain.Point
		obstacles  []Box
		wantCurved bool
		wantPrefix string
	}{
		{"clear two-point line curves", []domain.Point{a, z}, nil, true, "M0,0 C0,50 100,50 100,100"},
		{"a box in the way keeps the polyline", []domain.Point{a, {X: 0, Y: 50}, {X: 100, Y: 50}, z},
			[]Box{{X: 30, Y: 30, W: 40, H: 40}}, false, "M0,0 L0,50 L100,50 L100,100"},
		{"a weaving route is kept", []domain.Point{a, {X: 0, Y: 20}, {X: 50, Y: 20}, {X: 50, Y: 80}, {X: 100, Y: 80}, z},
			nil, false, "M0,0 L0,20"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Edge(tc.pts, true, tc.obstacles, 0, 0)
			if s.Curved != tc.wantCurved {
				t.Errorf("Curved = %v, want %v", s.Curved, tc.wantCurved)
			}
			if !strings.HasPrefix(s.D, tc.wantPrefix) {
				t.Errorf("D = %q, want prefix %q", s.D, tc.wantPrefix)
			}
			if s.Start != a || s.End != z {
				t.Errorf("tips = %v %v, want %v %v", s.Start, s.End, a, z)
			}
		})
	}
}

func TestEdgeTrimsTheEnds(t *testing.T) {
	s := Edge([]domain.Point{{X: 0, Y: 0}, {X: 0, Y: 100}}, true, nil, 10, 20)
	if !strings.HasPrefix(s.D, "M0,10 ") || !strings.HasSuffix(s.D, " 0,80") {
		t.Errorf("D = %q, want the line pulled back 10 and 20 from its tips", s.D)
	}
}

func TestUnitTo(t *testing.T) {
	p := domain.Point{X: 1, Y: 1}
	if got := UnitTo(p, p, domain.Point{X: 1, Y: 5}); got != [2]float64{0, 1} {
		t.Errorf("UnitTo with q at p = %v, want toward r", got)
	}
	if got := UnitTo(p, p, p); got != [2]float64{0, 0} {
		t.Errorf("UnitTo with no direction = %v, want zero", got)
	}
}
