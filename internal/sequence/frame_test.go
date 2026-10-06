package sequence

import (
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFrameHoldsLongLabel(t *testing.T) {
	Convey("Given frames whose labels are long unbreakable words", t, func() {
		src, err := os.ReadFile("testdata/frame_long_labels.mmd")
		So(err, ShouldBeNil)
		lay := layoutOf(string(src))
		d := lay.Diagram
		frame := func(kind string) *Frame {
			for _, f := range d.Frames {
				if f.Kind == kind {
					return f
				}
			}
			return nil
		}
		a, b, c := d.participant("A"), d.participant("B"), d.participant("C")

		Convey("Then every frame is wide enough for its tab and labels", func() {
			for _, f := range d.Frames {
				So(f.X1-f.X0, ShouldBeGreaterThanOrEqualTo, frameMinWidth(f, lay.m)-0.01)
				for _, l := range f.Lines {
					So(lay.m.face.Width(l, lay.m.fs), ShouldBeLessThanOrEqualTo, frameWordMax+0.01)
				}
			}
		})
		Convey("Then the lifelines move apart instead of the frame reaching over one it does not enclose", func() {
			So(frame("opt").X1, ShouldBeLessThan, b.X-lay.m.barW)
			So(frame("break").X0, ShouldBeGreaterThan, b.X)
			for _, k := range []string{"critical", "par"} {
				So(frame(k).X0, ShouldBeGreaterThan, a.X)
				So(frame(k).X1, ShouldBeGreaterThan, c.X)
			}
		})
	})

	Convey("Given a frame with a long label inside a participant box", t, func() {
		src, err := os.ReadFile("testdata/long_words.mmd")
		So(err, ShouldBeNil)
		lay := layoutOf(string(src))
		box, loop := lay.Diagram.Boxes[0], lay.Diagram.Frames[0]

		Convey("Then the frame stays inside the box", func() {
			So(loop.X0, ShouldBeGreaterThanOrEqualTo, box.X0)
			So(loop.X1, ShouldBeLessThanOrEqualTo, box.X1)
		})
	})

	Convey("Given a frame label far longer than any frame should be", t, func() {
		word := ""
		for i := 0; i < 12; i++ {
			word += "Unbreakable"
		}
		lay := layoutOf("sequenceDiagram\nA->>B: x\nloop " + word + "\nA->>B: y\nend")

		Convey("Then the word is cut into lines no wider than the cap, losing nothing", func() {
			f := lay.Diagram.Frames[0]
			So(len(f.Lines), ShouldBeGreaterThan, 1)
			joined := ""
			for _, l := range f.Lines {
				joined += l
				So(lay.m.face.Width(l, lay.m.fs), ShouldBeLessThanOrEqualTo, frameWordMax)
			}
			So(joined, ShouldEqual, "["+word+"]")
		})
	})
}
