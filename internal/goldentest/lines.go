package goldentest

import (
	"encoding/xml"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/arlintdev/go-mermaid/internal/curve"
	"github.com/arlintdev/go-mermaid/internal/domain"
)

var pathTok = regexp.MustCompile(`[A-Za-z]|[-0-9.]+`)

// Drawing is what a relationship-diagram picture holds, in the picture's own
// coordinates: its open lines (each <path fill="none"> outside <defs>,
// curves sampled) and its label backgrounds (each rect with rx="2" and
// fill-opacity="0.85", the renderers' label plate).
type Drawing struct {
	Lines  [][]domain.Point
	Labels []curve.Box
}

// ReadDrawing reads the lines and label plates of svg.
func ReadDrawing(svg []byte) (Drawing, error) {
	var out Drawing
	d := xml.NewDecoder(strings.NewReader(string(svg)))
	stack := []affine{identity}
	defs := 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			m := stack[len(stack)-1]
			attr := map[string]string{}
			for _, a := range t.Attr {
				attr[a.Name.Local] = a.Value
			}
			if v, ok := attr["transform"]; ok {
				tm, err := parseTransform(v)
				if err != nil {
					return out, err
				}
				m = m.mul(tm)
			}
			stack = append(stack, m)
			switch {
			case t.Name.Local == "defs":
				defs++
			case defs > 0:
			case t.Name.Local == "path" && attr["fill"] == "none":
				if pts := pathPoints(attr["d"], m); len(pts) > 1 {
					out.Lines = append(out.Lines, pts)
				}
			case t.Name.Local == "rect" && attr["rx"] == "2" && attr["fill-opacity"] == "0.85":
				f := func(k string) float64 { v, _ := strconv.ParseFloat(attr[k], 64); return v }
				x0, y0 := m.apply(f("x"), f("y"))
				x1, y1 := m.apply(f("x")+f("width"), f("y")+f("height"))
				out.Labels = append(out.Labels, curve.Box{X: math.Min(x0, x1), Y: math.Min(y0, y1), W: math.Abs(x1 - x0), H: math.Abs(y1 - y0)})
			}
		case xml.EndElement:
			if t.Name.Local == "defs" {
				defs--
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// pathPoints samples a path of one M followed by L, Q and C commands; any
// other path (a second M, an arc) yields nothing.
func pathPoints(d string, m affine) []domain.Point {
	toks := pathTok.FindAllString(d, -1)
	var pts []domain.Point
	cmd := ""
	var nums []float64
	pt := func(i int) domain.Point { x, y := m.apply(nums[i], nums[i+1]); return domain.Point{X: x, Y: y} }
	flush := func() bool {
		switch cmd {
		case "M":
			if len(pts) > 0 || len(nums) < 2 {
				return false
			}
			for i := 0; i+1 < len(nums); i += 2 {
				pts = append(pts, pt(i))
			}
		case "L":
			for i := 0; i+1 < len(nums); i += 2 {
				pts = append(pts, pt(i))
			}
		case "C":
			for i := 0; i+5 < len(nums); i += 6 {
				pts = append(pts, curve.Bezier(pts[len(pts)-1], pt(i), pt(i+2), pt(i+4))[1:]...)
			}
		case "Q":
			for i := 0; i+3 < len(nums); i += 4 {
				a, c, z := pts[len(pts)-1], pt(i), pt(i+2)
				// The cubic through the same curve as the quadratic a-c-z.
				c1 := domain.Point{X: a.X + 2*(c.X-a.X)/3, Y: a.Y + 2*(c.Y-a.Y)/3}
				c2 := domain.Point{X: z.X + 2*(c.X-z.X)/3, Y: z.Y + 2*(c.Y-z.Y)/3}
				pts = append(pts, curve.Bezier(a, c1, c2, z)[1:]...)
			}
		}
		return true
	}
	for _, tk := range toks {
		if unicode.IsLetter(rune(tk[0])) {
			if tk != "M" && tk != "L" && tk != "C" && tk != "Q" {
				return nil
			}
			if cmd != "" && !flush() {
				return nil
			}
			cmd, nums = tk, nil
			continue
		}
		v, err := strconv.ParseFloat(tk, 64)
		if err != nil {
			return nil
		}
		nums = append(nums, v)
	}
	if cmd == "" || !flush() {
		return nil
	}
	return pts
}

// HiddenLabels counts the label plates drawn over a line other than their
// own (a plate sits on its own line, so one crossed by two lines hides
// another), and the pairs of plates that overlap.
func HiddenLabels(svg []byte) (onLines, overlapping int, err error) {
	dr, err := ReadDrawing(svg)
	if err != nil {
		return 0, 0, err
	}
	for i, b := range dr.Labels {
		hits := 0
		for _, l := range dr.Lines {
			if curve.Crosses(l, b) {
				hits++
			}
		}
		if hits > 1 {
			onLines++
		}
		for _, o := range dr.Labels[i+1:] {
			if math.Min(b.X+b.W, o.X+o.W)-math.Max(b.X, o.X) > 1 && math.Min(b.Y+b.H, o.Y+o.H)-math.Max(b.Y, o.Y) > 1 {
				overlapping++
			}
		}
	}
	return onLines, overlapping, nil
}

// LabelsClear renders every dir/*.mmd and fails when two label plates
// overlap, or when more plates than most[name] (zero when absent) lie on a
// line other than their own.
func LabelsClear(t *testing.T, dir string, render Render, most map[string]int) {
	t.Helper()
	inputs, _ := filepath.Glob(filepath.Join(dir, "*.mmd"))
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".mmd")
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		out, err := render(string(src))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		on, over, err := HiddenLabels(out)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if on > most[name] {
			t.Errorf("%s: %d labels lie on another line, want at most %d", name, on, most[name])
		}
		if over > 0 {
			t.Errorf("%s: %d pairs of labels overlap", name, over)
		}
	}
}
