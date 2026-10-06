package parser

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStyles(t *testing.T) {
	Convey("Given classDef, class, style, and inline directives", t, func() {
		src := strings.Join([]string{
			"graph TD",
			"A --> B",
			"C:::warn --> B",
			"classDef warn fill:#fdd,stroke:#f00,color:#900",
			"class A warn",
			"style B fill:#dfd,stroke:#393",
		}, "\n")
		g, err := Flowchart(src)
		So(err, ShouldBeNil)

		Convey("Then class assignments resolve regardless of order", func() {
			So(g.NodeByID("A").Style.Fill, ShouldEqual, "#fdd")
			So(g.NodeByID("A").Style.Color, ShouldEqual, "#900")
			So(g.NodeByID("C").Style.Stroke, ShouldEqual, "#f00") // via inline :::
		})

		Convey("Then direct style overrides apply", func() {
			So(g.NodeByID("B").Style.Fill, ShouldEqual, "#dfd")
			So(g.NodeByID("B").Style.Stroke, ShouldEqual, "#393")
		})

		Convey("Then the styling lines make no nodes", func() {
			So(len(g.Nodes), ShouldEqual, 3)
		})
	})

	Convey("Given styling lines that end in a semicolon or share a line", t, func() {
		g, err := Flowchart("graph TD\nA --> B; classDef hot fill:#f99;\nclass A,B hot;")
		So(err, ShouldBeNil)
		So(g.NodeByID("A").Style.Fill, ShouldEqual, "#f99")
		So(g.NodeByID("B").Style.Fill, ShouldEqual, "#f99")
	})

	Convey("Given classDef default", t, func() {
		g, err := Flowchart("graph TD\nA --> B\nclassDef default fill:#eee\nstyle B fill:#abc")
		So(err, ShouldBeNil)
		So(g.NodeByID("A").Style.Fill, ShouldEqual, "#eee")
		So(g.NodeByID("B").Style.Fill, ShouldEqual, "#abc")
	})

	Convey("Given style values that are not plain values", t, func() {
		g, err := Flowchart(`graph TD
A --> B
style A fill:url(javascript:x),stroke:"><script>,stroke-width:9999,color:#fff" onload="x
classDef bad stroke-dasharray:1 2 <x>,font-weight:expression(1)
class B bad
linkStyle 0 stroke:red"/><a,stroke-width:3px`)
		So(err, ShouldBeNil)

		Convey("Then every value that fails validation is dropped", func() {
			a := g.NodeByID("A").Style
			So(a.Fill, ShouldEqual, "")
			So(a.Stroke, ShouldEqual, "")
			So(a.StrokeWidth, ShouldEqual, "")
			So(a.Color, ShouldEqual, "")
			b := g.NodeByID("B").Style
			So(b.StrokeDash, ShouldEqual, "")
			So(b.FontWeight, ShouldEqual, "")
			So(g.Edges[0].Style.Stroke, ShouldEqual, "")
			So(g.Edges[0].Style.StrokeWidth, ShouldEqual, "3")
		})
	})

	Convey("Given a style on a subgraph", t, func() {
		g, err := Flowchart("graph TD\nsubgraph S\nA\nend\nstyle S fill:#ffe,stroke:#cc0")
		So(err, ShouldBeNil)
		So(g.Subgraphs[0].Style.Fill, ShouldEqual, "#ffe")
	})
}
