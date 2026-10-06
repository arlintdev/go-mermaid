package sequence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func layoutOf(src string) *Layout {
	return Compute(mustParse(src), Options{FontSize: 14, FontFace: "sans-serif"})
}

func corpus(t *testing.T) map[string]*Layout {
	files, _ := filepath.Glob("testdata/*.mmd")
	out := map[string]*Layout{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[strings.TrimSuffix(filepath.Base(f), ".mmd")] = layoutOf(string(src))
	}
	return out
}

func TestLayoutCorpus(t *testing.T) {
	Convey("Given every sequence diagram in testdata", t, func() {
		for name, lay := range corpus(t) {
			d, m := lay.Diagram, lay.m
			inCanvas := func(lo, hi float64) bool {
				return lo+lay.OffsetX >= -0.01 && hi+lay.OffsetX <= lay.Width+0.01
			}
			Convey(name+": message labels fit between their lifelines", func() {
				for _, msg := range d.Messages {
					w := widest(msg.Lines, m.face, m.fs)
					if msg.From == msg.To {
						i := d.index[msg.From]
						if i+1 < len(d.Participants) {
							next := d.Participants[i+1]
							So(d.Participants[i].X+selfLabelOffset(m, msg)+w, ShouldBeLessThan, next.X)
						}
						continue
					}
					span := msg.X2 - msg.X1
					if span < 0 {
						span = -span
					}
					So(w, ShouldBeLessThanOrEqualTo, span)
				}
			})
			Convey(name+": rows never overlap", func() {
				prevBottom := -1.0
				for _, msg := range d.Messages {
					top := msg.Y - 6*m.k - float64(len(msg.Lines))*m.lineH
					So(top, ShouldBeGreaterThan, prevBottom)
					prevBottom = msg.Y
					if msg.From == msg.To {
						prevBottom = msg.Y + m.loopH
					}
				}
			})
			Convey(name+": everything is on the canvas", func() {
				for _, p := range d.Participants {
					So(inCanvas(p.X-p.Width/2, p.X+p.Width/2), ShouldBeTrue)
				}
				for _, n := range d.Notes {
					So(inCanvas(n.X, n.X+n.W), ShouldBeTrue)
				}
				for _, f := range d.Frames {
					So(inCanvas(f.X0, f.X1), ShouldBeTrue)
					So(f.Y1, ShouldBeLessThan, lay.BottomY)
				}
			})
			Convey(name+": a nested frame sits inside its parent", func() {
				var stack []*Frame
				for _, it := range d.items {
					switch it.kind {
					case itFrameStart:
						if n := len(stack); n > 0 {
							p := stack[n-1]
							So(it.frame.X0, ShouldBeGreaterThan, p.X0)
							So(it.frame.X1, ShouldBeLessThan, p.X1)
							So(it.frame.Y0, ShouldBeGreaterThan, p.Y0)
						}
						stack = append(stack, it.frame)
					case itFrameEnd:
						stack = stack[:len(stack)-1]
					}
				}
			})
		}
	})
}

func TestLayout(t *testing.T) {
	Convey("Given a long message between neighbours", t, func() {
		lay := layoutOf("sequenceDiagram\nA->>B: " + strings.Repeat("word ", 60))
		msg := lay.Diagram.Messages[0]
		Convey("Then it wraps and the lifelines move apart to hold it", func() {
			So(len(msg.Lines), ShouldBeGreaterThan, 1)
			So(widest(msg.Lines, lay.m.face, lay.m.fs), ShouldBeLessThanOrEqualTo, lay.m.wrapW)
			So(msg.X2-msg.X1, ShouldBeGreaterThanOrEqualTo, widest(msg.Lines, lay.m.face, lay.m.fs))
		})
	})

	Convey("Given an activated receiver", t, func() {
		lay := layoutOf("sequenceDiagram\nA->>+B: call\nB-->>-A: back")
		d := lay.Diagram
		Convey("Then arrows meet the bar's edge, not the lifeline", func() {
			b := d.participant("B")
			So(d.Messages[0].X2, ShouldEqual, b.X-lay.m.barW/2)
			So(d.Messages[1].X1, ShouldEqual, b.X-lay.m.barW/2)
			So(len(d.Bars), ShouldEqual, 1)
			So(d.Bars[0].Y1, ShouldEqual, d.Messages[0].Y)
			So(d.Bars[0].Y2, ShouldEqual, d.Messages[1].Y)
		})
	})

	Convey("Given nested activations", t, func() {
		lay := layoutOf("sequenceDiagram\nA->>+B: one\nA->>+B: two\nB-->>-A: x\nB-->>-A: y")
		Convey("Then the inner bar is offset right", func() {
			bars := lay.Diagram.Bars
			So(len(bars), ShouldEqual, 2)
			So(bars[0].Depth, ShouldEqual, 1)
			So(bars[1].Depth, ShouldEqual, 0)
		})
	})

	Convey("Given a created participant", t, func() {
		lay := layoutOf("sequenceDiagram\nA->>B: x\ncreate participant C\nB->>C: y\ndestroy C\nC->>B: z")
		d := lay.Diagram
		c := d.participant("C")
		Convey("Then its header is drawn at the creating message", func() {
			So(c.TopY, ShouldBeGreaterThan, lay.HeadTop)
			So(c.TopY+lay.HeadH/2, ShouldEqual, d.Messages[1].Y)
			So(d.Messages[1].X2, ShouldEqual, c.X-c.Width/2)
		})
		Convey("Then its lifeline stops at the destroying message", func() {
			So(c.LifeEnd, ShouldEqual, d.Messages[2].Y)
		})
	})

	Convey("Given a note left of the first participant", t, func() {
		lay := layoutOf("sequenceDiagram\nparticipant A\nNote left of A: a long note on the far left side")
		Convey("Then the drawing shifts right so it stays on the canvas", func() {
			So(lay.OffsetX, ShouldBeGreaterThan, 0)
			n := lay.Diagram.Notes[0]
			So(n.X+lay.OffsetX, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})

	Convey("Given boxes", t, func() {
		lay := layoutOf("sequenceDiagram\nbox Aqua Left\nparticipant A\nend\nbox Right\nparticipant B\nend\nA->>B: x")
		d := lay.Diagram
		Convey("Then boxes enclose their members and do not overlap", func() {
			l, r := d.Boxes[0], d.Boxes[1]
			So(l.X0, ShouldBeLessThan, d.participant("A").X-d.participant("A").Width/2)
			So(l.X1, ShouldBeLessThanOrEqualTo, r.X0)
			So(lay.HeadTop, ShouldBeGreaterThan, 0)
		})
	})
}

func TestRender(t *testing.T) {
	Convey("Given a diagram with every kind of statement", t, func() {
		src, err := os.ReadFile("testdata/s01_sequence_full.mmd")
		So(err, ShouldBeNil)
		out, err := renderDefault(string(src))
		svg := string(out)
		So(err, ShouldBeNil)
		Convey("Then frames carry their tab and condition", func() {
			for _, s := range []string{">alt<", ">[signed in]<", ">[not signed in]<", ">critical<", ">[conflict]<", ">break<", ">par<"} {
				So(svg, ShouldContainSubstring, s)
			}
		})
		Convey("Then the rect colour is a fill", func() {
			So(svg, ShouldContainSubstring, `fill="rgb(200, 220, 255)"`)
		})
		Convey("Then markers use the per-diagram prefix", func() {
			So(svg, ShouldNotContainSubstring, `id="arrowhead"`)
			So(svg, ShouldContainSubstring, `-seq-arrow)"`)
		})
		Convey("Then headers are mirrored at the bottom", func() {
			So(strings.Count(svg, ">Web app<"), ShouldEqual, 2)
		})
		Convey("Then autonumbers are drawn as badges", func() {
			So(svg, ShouldContainSubstring, ">13</text>")
		})
	})
}
