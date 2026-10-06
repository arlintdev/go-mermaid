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
