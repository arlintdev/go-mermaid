package layout

import (
	"testing"

	"github.com/arlintdev/go-mermaid/internal/domain"
	. "github.com/smartystreets/goconvey/convey"
)

func TestFlowMeasured(t *testing.T) {
	Convey("Given a graph whose caller sized its boxes, label and title", t, func() {
		g := &domain.Graph{Direction: domain.TopBottom,
			Nodes: []*domain.Node{
				{ID: "a", Label: "a", Size: domain.Size{W: 230, H: 140}},
				{ID: "b", Label: "b", Size: domain.Size{W: 90, H: 40}},
			},
			Edges:     []*domain.Edge{{From: "a", To: "b", Label: "x", LabelSize: domain.Size{W: 150, H: 60}}},
			Subgraphs: []*domain.Subgraph{{ID: "s", Title: "s", NodeIDs: []string{"b"}, TitleSize: domain.Size{W: 300, H: 50}}},
		}
		Convey("When Flow lays it out as measured", func() {
			_, err := Flow(g, Options{Measured: true})
			So(err, ShouldBeNil)
			Convey("Then every size is kept and the label and title get their room", func() {
				So(g.Nodes[0].Size, ShouldResemble, domain.Size{W: 230, H: 140})
				So(g.Nodes[1].Size, ShouldResemble, domain.Size{W: 90, H: 40})
				So(g.Edges[0].LabelSize, ShouldResemble, domain.Size{W: 150, H: 60})
				a, b := g.Nodes[0], g.Nodes[1]
				box := g.Subgraphs[0].Box
				So(box.Size.W, ShouldBeGreaterThanOrEqualTo, 300)
				So(b.Pos.Y-box.Min.Y, ShouldBeGreaterThanOrEqualTo, 50)
				So(box.Min.Y-(a.Pos.Y+a.Size.H), ShouldBeGreaterThanOrEqualTo, 60)
			})
		})
	})
}
