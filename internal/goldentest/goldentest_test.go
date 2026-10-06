package goldentest

import "testing"

func TestCheckSVG(t *testing.T) {
	ok := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs><marker id="a"><path d="M0 0"/></marker></defs><line x1="0" marker-end="url(#a)"/><text>a &lt;b&gt;</text></svg>`
	if err := CheckSVG([]byte(ok)); err != nil {
		t.Fatalf("plain SVG refused: %v", err)
	}
	bad := map[string]string{
		"style attribute": `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:red"/></svg>`,
		"style element":   `<svg xmlns="http://www.w3.org/2000/svg"><style>x</style></svg>`,
		"foreignObject":   `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject/></svg>`,
		"link":            `<svg xmlns="http://www.w3.org/2000/svg"><a href="x"/></svg>`,
		"foreign ref":     `<svg xmlns="http://www.w3.org/2000/svg"><line marker-end="url(#nope)"/></svg>`,
		"script value":    `<svg xmlns="http://www.w3.org/2000/svg"><rect fill="javascript:x"/></svg>`,
		"broken":          `<svg xmlns="http://www.w3.org/2000/svg"><rect fill=""x"/></svg>`,
		"two ids":         `<svg xmlns="http://www.w3.org/2000/svg"><g id="a"/><g id="a"/></svg>`,
		"class":           `<svg xmlns="http://www.w3.org/2000/svg"><g class="a"/></svg>`,
	}
	for name, s := range bad {
		if err := CheckSVG([]byte(s)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestTextBoxes(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50" font-family="sans-serif" font-size="10">` +
		`<g transform="translate(10,20)"><text x="0" y="0" text-anchor="middle">ab</text></g>` +
		`<text font-weight="bold"><tspan x="50" y="10">c</tspan><tspan x="50" dy="12">d</tspan></text>` +
		`<text x="0" y="40" transform="rotate(-90 0 40)">long text here</text></svg>`
	boxes, err := TextBoxes([]byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	if len(boxes) != 4 {
		t.Fatalf("got %d boxes: %v", len(boxes), boxes)
	}
	if b := boxes[0]; b.X0 >= 10 || b.X1 <= 10 || b.Y1 <= 20 || b.Y0 >= 20 {
		t.Errorf("translated, centred text: %v", b)
	}
	if boxes[2].Y0 <= boxes[1].Y1-1 {
		t.Errorf("dy did not move the second line down: %v %v", boxes[1], boxes[2])
	}
	if b := boxes[3]; b.Y0 >= 0 || b.X1-b.X0 > 12 {
		t.Errorf("rotated text should run up past the top: %v", b)
	}
	off, _ := OffCanvas([]byte(svg), 1)
	if len(off) != 1 || off[0].Text != "long text here" {
		t.Errorf("off canvas: %v", off)
	}
}
