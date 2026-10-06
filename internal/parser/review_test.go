package parser

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestClickIsDropped(t *testing.T) {
	Convey("Given a click directive", t, func() {
		g, err := Flowchart("graph TD\nA\nclick A \"javascript:alert(1)\"\nclick A href \"https://example.com\"")

		Convey("Then it is read and left out, since a static picture cannot link", func() {
			So(err, ShouldBeNil)
			So(len(g.Nodes), ShouldEqual, 1)
			So(g.NodeByID("A").Link, ShouldEqual, "")
		})
	})
}

func TestInlineClassInsideLabel(t *testing.T) {
	Convey("Given a node whose label text contains ':::'", t, func() {
		g, err := Flowchart("graph LR\nA[\"a:::b\"]")

		Convey("Then the label is left intact and no class is read from it", func() {
			So(err, ShouldBeNil)
			So(g.NodeByID("A").Label, ShouldEqual, "a:::b")
			So(g.NodeByID("A").Style, ShouldBeNil)
		})
	})
}

func TestSubgraphPredeclaredMembership(t *testing.T) {
	Convey("Given a node declared before a subgraph then used inside it", t, func() {
		g, err := parse("graph TD\nA --> B\nsubgraph S\nA --> C\nend")

		Convey("When parsing", func() {
			Convey("Then the pre-declared node belongs to the subgraph", func() {
				So(err, ShouldBeNil)
				So(len(g.Subgraphs), ShouldEqual, 1)
				So(g.Subgraphs[0].NodeIDs, ShouldContain, "A")
				So(g.Subgraphs[0].NodeIDs, ShouldContain, "C")
			})
		})
	})
}
