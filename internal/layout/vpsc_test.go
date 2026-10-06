package layout

import (
	"math"
	"testing"
)

func TestVPSCSimple(t *testing.T) {
	s := &vpsc{}
	a := s.addVar(0, 1)
	b := s.addVar(0, 1)
	c := s.addVar(110, 4)
	s.constrain(a, b, 100)
	s.prepare()
	s.solve()
	if math.Abs(b.x-a.x-100) > 1e-6 || math.Abs(a.x+50) > 1e-6 || math.Abs(c.x-110) > 1e-6 {
		t.Errorf("a=%v b=%v c=%v", a.x, b.x, c.x)
	}
}
