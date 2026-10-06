// Package goldentest holds the checks the diagram packages' tests share:
// golden-file comparison of rendered SVG, and a strict allow-list check that
// the output is plain static SVG (no style attribute or element, no
// foreignObject, no script, no external reference, every value plain).
//
// Set GOLDEN_UPDATE=1 to rewrite the golden files after an intentional
// output change, then review the diff.
package goldentest

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var elements = map[string]bool{
	"svg": true, "g": true, "defs": true, "marker": true, "title": true, "desc": true,
	"path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true,
	"text": true, "tspan": true,
}

var attributes = map[string]bool{
	"xmlns": true, "viewBox": true, "preserveAspectRatio": true, "width": true, "height": true, "role": true,
	"aria-label": true, "aria-hidden": true, "id": true,
	"x": true, "y": true, "x1": true, "x2": true, "y1": true, "y2": true, "dx": true, "dy": true,
	"cx": true, "cy": true, "r": true, "rx": true, "ry": true, "d": true, "points": true, "transform": true,
	"refX": true, "refY": true, "markerWidth": true, "markerHeight": true, "markerUnits": true, "orient": true,
	"marker-start": true, "marker-mid": true, "marker-end": true,
	"fill": true, "fill-opacity": true, "fill-rule": true, "stroke": true, "stroke-width": true, "stroke-opacity": true,
	"stroke-dasharray": true, "stroke-linecap": true, "stroke-linejoin": true, "opacity": true,
	"font-family": true, "font-size": true, "font-weight": true, "font-style": true,
	"text-anchor": true, "dominant-baseline": true,
}

var (
	localRef   = regexp.MustCompile(`^url\(#([A-Za-z0-9_-]+)\)$`)
	plainValue = regexp.MustCompile(`^[A-Za-z0-9 #%.,:;()_+\-'"/]*$`)
)

// CheckSVG reports the first thing in svg that is not plain static SVG of
// the allowed elements and attributes, or nil when all of it is.
func CheckSVG(svg []byte) error {
	d := xml.NewDecoder(strings.NewReader(string(svg)))
	d.Strict = true
	ids := map[string]bool{}
	var refs []string
	depth, roots := 0, 0
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if !elements[name] {
				return fmt.Errorf("element <%s> is not allowed", name)
			}
			if depth == 0 {
				roots++
				if name != "svg" || roots > 1 {
					return fmt.Errorf("root must be one <svg>, got <%s>", name)
				}
			}
			for _, a := range t.Attr {
				key := a.Name.Local
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && key == "xmlns") {
					if a.Value != "http://www.w3.org/2000/svg" {
						return fmt.Errorf("namespace %q is not allowed", a.Value)
					}
					continue
				}
				if a.Name.Space != "" || !attributes[key] {
					return fmt.Errorf("attribute %q on <%s> is not allowed", key, name)
				}
				v := a.Value
				l := strings.ToLower(v)
				if strings.Contains(l, "url(") {
					m := localRef.FindStringSubmatch(strings.TrimSpace(v))
					if m == nil {
						return fmt.Errorf("attribute %s=%q is not a local reference", key, v)
					}
					refs = append(refs, m[1])
					continue
				}
				if strings.ContainsAny(v, "<>&\\`") || strings.Contains(l, "javascript") || strings.Contains(l, "expression") || !plainValue.MatchString(v) {
					return fmt.Errorf("attribute %s=%q on <%s> is not a plain value", key, v, name)
				}
				if key == "id" {
					if ids[v] {
						return fmt.Errorf("id %q is used twice", v)
					}
					ids[v] = true
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return errors.New("text outside the root element")
			}
		case xml.Directive:
			return errors.New("directives are not allowed")
		case xml.ProcInst:
			if t.Target != "xml" {
				return fmt.Errorf("processing instruction %q is not allowed", t.Target)
			}
		}
	}
	if roots != 1 || depth != 0 {
		return errors.New("not one complete <svg> element")
	}
	for _, r := range refs {
		if !ids[r] {
			return fmt.Errorf("url(#%s) points at no id in the picture", r)
		}
	}
	return nil
}

// Render renders one diagram source.
type Render func(src string) ([]byte, error)

// Golden renders every dir/*.mmd, checks the output with CheckSVG and
// compares it byte for byte with the matching .svg golden file. With
// GOLDEN_UPDATE=1 set it rewrites the golden files instead of comparing.
func Golden(t *testing.T, dir string, render Render) {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join(dir, "*.mmd"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no golden inputs in %s", dir)
	}
	update := os.Getenv("GOLDEN_UPDATE") == "1"
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".mmd")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := render(string(src))
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if err := CheckSVG(got); err != nil {
				t.Fatalf("output is not plain static SVG: %v", err)
			}
			path := strings.TrimSuffix(in, ".mmd") + ".svg"
			if update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file (run with GOLDEN_UPDATE=1): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("output differs from %s; if the change is intended, rerun with GOLDEN_UPDATE=1 and review the diff", path)
			}
		})
	}
}

// Injection is a value that breaks out of an attribute or a text node if a
// renderer writes it without escaping.
const Injection = `"><a href="javascript:alert(1)">x</a><rect width="9999" `

// Hostile renders src and fails the test when the output is not plain
// static SVG or carries the raw Injection payload.
func Hostile(t *testing.T, render Render, src string) {
	t.Helper()
	got, err := render(src)
	if err != nil {
		return // refusing the source is a safe outcome
	}
	if err := CheckSVG(got); err != nil {
		t.Errorf("hostile source produced unsafe SVG: %v\nsource:\n%s", err, src)
	}
}
