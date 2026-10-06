package quadrant

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParse(t *testing.T) {
	Convey("Given a quadrant chart", t, func() {
		src := "quadrantChart\ntitle Reach\nx-axis Low --> High\ny-axis Bottom --> Top\nquadrant-1 Expand\nA: [0.3, 0.6]\nB: [0.45, 0.23]"

		Convey("When parsing", func() {
			d, err := Parse(src)

			Convey("Then title, axes, quadrant and points parse", func() {
				So(err, ShouldBeNil)
				So(d.Title, ShouldEqual, "Reach")
				So(d.XLeft, ShouldEqual, "Low")
				So(d.XRight, ShouldEqual, "High")
				So(d.YBottom, ShouldEqual, "Bottom")
				So(d.YTop, ShouldEqual, "Top")
				So(d.Quadrant[0], ShouldEqual, "Expand")
				So(len(d.Points), ShouldEqual, 2)
				So(d.Points[0].X, ShouldEqual, 0.3)
				So(d.Points[0].Y, ShouldEqual, 0.6)
			})
		})
	})

	Convey("Given an invalid point", t, func() {
		Convey("When parsing", func() {
			_, err := Parse("quadrantChart\nA: [x, y]")

			Convey("Then it returns an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})

	Convey("Given no header", t, func() {
		Convey("When parsing", func() {
			_, err := Parse("title X")

			Convey("Then it returns an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestRender(t *testing.T) {
	Convey("Given a quadrant chart, when rendering", t, func() {
		out, err := Render("quadrantChart\ntitle T\nquadrant-1 Q1\nA: [0.7, 0.8]",
			RenderOptions{Theme: "default", FontSize: 14, Padding: 16})
		svg := string(out)

		Convey("Then it draws quadrants, axes grid, and the point", func() {
			So(err, ShouldBeNil)
			So(svg, ShouldStartWith, "<svg")
			So(svg, ShouldContainSubstring, "<circle")
			So(svg, ShouldContainSubstring, ">A<")
			So(svg, ShouldContainSubstring, ">Q1<")
		})
	})
}

func TestPointStyles(t *testing.T) {
	d, err := Parse("quadrantChart\nA: [0.1, 0.2] radius: 10, color: #ff0000, stroke-color: #000, stroke-width: 2px\n" +
		"B:::hot: [0.3, 0.4] color: blue\nC:::hot: [0.5, 0.6]\nD: [0.5, 0.5] color: url(#x), radius: 9999\n" +
		"classDef hot color: #00ff00, radius: 8, stroke-width: 3px")
	if err != nil {
		t.Fatal(err)
	}
	a := d.Points[0].Style
	if a.Radius != 10 || a.Color != "#ff0000" || a.StrokeColor != "#000" || a.StrokeWidth != 2 {
		t.Errorf("inline style: %+v", a)
	}
	b := d.Points[1].Style.merge(d.Classes[d.Points[1].Class])
	if d.Points[1].Label != "B" || b.Color != "blue" || b.Radius != 8 || b.StrokeWidth != 3 {
		t.Errorf("inline over class: %+v", b)
	}
	c := d.Points[2].Style.merge(d.Classes["hot"])
	if c.Color != "#00ff00" {
		t.Errorf("class style: %+v", c)
	}
	if s := d.Points[3].Style; s.Color != "" || s.Radius != 0 {
		t.Errorf("invalid values must be dropped: %+v", s)
	}
}
