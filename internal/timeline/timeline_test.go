package timeline

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParse(t *testing.T) {
	Convey("Given a timeline with sections and multi-event periods", t, func() {
		src := "timeline\ntitle Social\nsection Early\n2002 : LinkedIn\n2004 : Facebook : Google"

		Convey("When parsing", func() {
			d, err := Parse(src)

			Convey("Then title, sections, periods and events parse", func() {
				So(err, ShouldBeNil)
				So(d.Title, ShouldEqual, "Social")
				So(len(d.Sections), ShouldEqual, 1)
				So(len(d.Sections[0].Periods), ShouldEqual, 2)
				So(d.Sections[0].Periods[1].Time, ShouldEqual, "2004")
				So(d.Sections[0].Periods[1].Events, ShouldResemble, []string{"Facebook", "Google"})
			})
		})
	})

	Convey("Given a period with no events and an event before any period", t, func() {
		d, err := Parse("timeline\nsection S\n2002 no colon")
		So(err, ShouldBeNil)
		So(d.Sections[0].Periods[0].Time, ShouldEqual, "2002 no colon")
		So(d.Sections[0].Periods[0].Events, ShouldBeEmpty)
		_, err = Parse("timeline\n: orphan")
		So(err, ShouldNotBeNil)
	})

	Convey("Given continuation lines under a period", t, func() {
		d, err := Parse("timeline\n2024 : Started\n     : First users\n2025 : PostgreSQL\n  : More : Still more")
		So(err, ShouldBeNil)
		ps := d.Periods()
		So(len(ps), ShouldEqual, 2)
		So(ps[0].Events, ShouldResemble, []string{"Started", "First users"})
		So(ps[1].Events, ShouldResemble, []string{"PostgreSQL", "More", "Still more"})
	})

	Convey("Given no header", t, func() {
		Convey("When parsing", func() {
			_, err := Parse("2002 : x")

			Convey("Then it returns an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func TestRender(t *testing.T) {
	Convey("Given a timeline, when rendering", t, func() {
		out, err := Render("timeline\ntitle T\n2002 : LinkedIn\n2004 : Facebook",
			RenderOptions{Theme: "default", FontSize: 14, Padding: 16})
		svg := string(out)

		Convey("Then it draws an axis, periods, and event boxes", func() {
			So(err, ShouldBeNil)
			So(svg, ShouldStartWith, "<svg")
			So(svg, ShouldContainSubstring, ">2002<")
			So(svg, ShouldContainSubstring, ">LinkedIn<")
			So(svg, ShouldContainSubstring, "marker-end")
		})
	})
}
