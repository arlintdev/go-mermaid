package render

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/arlintdev/go-mermaid/internal/goldentest"
	"github.com/arlintdev/go-mermaid/internal/layout"
	"github.com/arlintdev/go-mermaid/internal/parser"
)

func laidOut(src string) *layout.Result {
	g, err := parser.Flowchart(src)
	if err != nil {
		panic(err)
	}
	res, err := layout.Flow(g, layout.Options{FontSize: 16})
	if err != nil {
		panic(err)
	}
	return res
}

var opts = Options{Theme: "default", FontFace: "sans-serif", FontSize: 16, Padding: 16, IDPrefix: "t1"}

func draw(src string) string {
	out, err := SVG(laidOut(src), opts)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func TestSVG(t *testing.T) {
	Convey("Given a laid-out flowchart", t, func() {
		svg := draw("graph TD\nA[Start] --> B((End))")

		Convey("Then it produces a well-formed, plain SVG document", func() {
			So(svg, ShouldStartWith, "<svg")
			So(goldentest.CheckSVG([]byte(svg)), ShouldBeNil)
			So(svg, ShouldContainSubstring, `marker id="t1-arrow-0"`)
			So(svg, ShouldContainSubstring, `marker-end="url(#t1-arrow-0)"`)
			So(svg, ShouldContainSubstring, "<circle")
		})
	})

	Convey("Given each node shape", t, func() {
		cases := []struct{ name, src, want string }{
			{"round", "graph TD\nA(R)", `rx="5"`},
			{"stadium", "graph TD\nA([S])", "<rect"},
			{"diamond", "graph TD\nA{D}", "<polygon"},
			{"asymmetric", "graph TD\nA>F]", "<polygon"},
			{"double circle", "graph TD\nA(((D)))", "<circle"},
			{"cylinder", "graph TD\nA[(C)]", " A"},
			{"subroutine", "graph TD\nA[[S]]", " V"},
		}
		for _, c := range cases {
			c := c
			Convey("When rendering the "+c.name+" shape", func() {
				svg := draw(c.src)
				So(svg, ShouldContainSubstring, c.want)
				So(goldentest.CheckSVG([]byte(svg)), ShouldBeNil)
			})
		}
	})

	Convey("Given different link styles", t, func() {
		So(draw("graph TD\nA -.-> B"), ShouldContainSubstring, "stroke-dasharray")
		So(draw("graph TD\nA ==> B"), ShouldContainSubstring, `stroke-width="3.5"`)
		So(draw("graph TD\nA --- B"), ShouldNotContainSubstring, "marker-end")
		So(draw("graph TD\nA ~~~ B"), ShouldNotContainSubstring, "<path")
		both := draw("graph TD\nA <--> B")
		So(both, ShouldContainSubstring, "marker-start")
		So(both, ShouldContainSubstring, "marker-end")
		So(draw("graph TD\nA --o B"), ShouldContainSubstring, "-circle-")
		So(draw("graph TD\nA --x B"), ShouldContainSubstring, "-cross-")
	})

	Convey("Given an edge label", t, func() {
		svg := draw("graph TD\nA -->|go| B")
		So(svg, ShouldContainSubstring, ">go<")
		So(svg, ShouldContainSubstring, `fill="#e8e8e8"`)
	})

	Convey("Given the dark theme and an unknown theme", t, func() {
		dark, _ := SVG(laidOut("graph TD\nA --> B"), Options{Theme: "dark", FontSize: 16, Padding: 16})
		So(string(dark), ShouldContainSubstring, "#1e1e1e")
		unknown, _ := SVG(laidOut("graph TD\nA --> B"), Options{Theme: "nope", FontSize: 16})
		So(string(unknown), ShouldContainSubstring, "#ffffff")
	})

	Convey("Given a label with XML-special characters", t, func() {
		res := laidOut("graph TD\nA --> B")
		res.Graph.Nodes[0].Lines = []string{`a<b & "c"`}
		out, _ := SVG(res, opts)
		So(string(out), ShouldContainSubstring, "a&lt;b &amp; &quot;c&quot;")
	})

	Convey("Given an id prefix that is not a plain name", t, func() {
		out, _ := SVG(laidOut("graph TD\nA --> B"), Options{FontSize: 16, IDPrefix: `x"><a`})
		So(goldentest.CheckSVG(out), ShouldBeNil)
		So(string(out), ShouldContainSubstring, `id="m-arrow-0"`)
	})
}

func TestEscPlain(t *testing.T) {
	if strings.Contains(esc("plain"), "&") {
		t.Error("plain text was changed")
	}
}

func TestParallelEdgeLabelsSVG(t *testing.T) {
	Convey("Given opposite labelled edges between the same node pair", t, func() {
		svg := draw("flowchart TD\nClient -->|request| API\nAPI -->|response| Client")

		Convey("Then both labels are drawn, at different places", func() {
			So(svg, ShouldContainSubstring, ">request<")
			So(svg, ShouldContainSubstring, ">response<")
			re := regexp.MustCompile(`<rect x="([-0-9.]+)" y="([-0-9.]+)"[^>]*fill="#e8e8e8"`)
			m := re.FindAllStringSubmatch(svg, -1)
			So(len(m), ShouldEqual, 2)
			So(m[0][1]+","+m[0][2], ShouldNotEqual, m[1][1]+","+m[1][2])
			So(svg, ShouldNotContainSubstring, `<rect x="-`)
		})
	})
}

func TestSubgraphTitleFitsBox(t *testing.T) {
	Convey("Given a subgraph whose title is wider than its member nodes", t, func() {
		title := "A Very Long Subgraph Title That Is Wide"
		svg := draw("flowchart TD\nsubgraph s [" + title + "]\na --> b\nend")
		box := regexp.MustCompile(`<rect x="([-0-9.]+)" y="[-0-9.]+" width="([0-9.]+)"[^>]*fill="#ffffde"`).FindStringSubmatch(svg)
		So(box, ShouldNotBeNil)
		w, _ := strconv.ParseFloat(box[2], 64)
		So(w, ShouldBeGreaterThan, 280)
		So(svg, ShouldContainSubstring, title)
	})
}
