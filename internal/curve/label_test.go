package curve

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/domain"
)

func line(pts ...float64) Shape {
	var s Shape
	for i := 0; i+1 < len(pts); i += 2 {
		s.Pts = append(s.Pts, domain.Point{X: pts[i], Y: pts[i+1]})
	}
	return s
}

func TestPlaceLabels(t *testing.T) {
	// A vertical line from 0,0 to 0,200, crossed at y=100 by a horizontal
	// one; the label starts on the crossing.
	lines := []Shape{line(0, 0, 0, 200), line(-100, 100, 100, 100)}
	labels := []Label{{Line: 0, W: 60, H: 20, X: 0, Y: 100}}
	PlaceLabels(lines, labels, nil)
	l := labels[0]
	if l.X != 0 {
		t.Errorf("the label left its line: %+v", l)
	}
	if l.Y-10 < 100 && l.Y+10 > 100 {
		t.Errorf("the label still covers the crossing line: %+v", l)
	}
	if l.Y-10 < endRoom || l.Y+10 > 200-endRoom {
		t.Errorf("the label covers an end of its line: %+v", l)
	}

	clear := []Label{{Line: 0, W: 60, H: 20, X: 0, Y: 40}}
	PlaceLabels(lines, clear, nil)
	if clear[0].X != 0 || clear[0].Y != 40 {
		t.Errorf("a clear label moved: %+v", clear[0])
	}

	// Two labels may not be put on top of each other.
	two := []Label{{Line: 0, W: 60, H: 20, X: 0, Y: 100}, {Line: 1, W: 60, H: 20, X: 0, Y: 100}}
	PlaceLabels(lines, two, nil)
	a, b := two[0], two[1]
	if a.X-30 < b.X+30 && b.X-30 < a.X+30 && a.Y-10 < b.Y+10 && b.Y-10 < a.Y+10 {
		t.Errorf("labels overlap: %+v %+v", a, b)
	}
}

func TestPlaceLabelsBeside(t *testing.T) {
	// A short line crossed everywhere along it by a parallel neighbour two
	// pixels away: no spot on it is clear, so the label goes beside it, on
	// the side away from the neighbour.
	lines := []Shape{line(0, 0, 0, 120), line(4, 0, 4, 120)}
	labels := []Label{{Line: 0, W: 40, H: 16, X: 0, Y: 60}}
	PlaceLabels(lines, labels, nil)
	if l := labels[0]; l.X+20 > 1 {
		t.Errorf("the label should sit left of its line, clear of the neighbour: %+v", l)
	}
}

func TestCrosses(t *testing.T) {
	b := Box{X: 0, Y: 0, W: 10, H: 10}
	for _, c := range []struct {
		pts  []domain.Point
		want bool
	}{
		{[]domain.Point{{X: -5, Y: 5}, {X: 15, Y: 5}}, true},
		{[]domain.Point{{X: -5, Y: -5}, {X: 15, Y: 15}}, true},
		{[]domain.Point{{X: -5, Y: 20}, {X: 15, Y: 20}}, false},
		{[]domain.Point{{X: 0.5, Y: -5}, {X: 0.5, Y: 15}}, false}, // on the edge
	} {
		if got := Crosses(c.pts, b); got != c.want {
			t.Errorf("Crosses(%v) = %v, want %v", c.pts, got, c.want)
		}
	}
}
