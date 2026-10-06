package sequence

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func mustParse(src string) *Diagram {
	d, err := Parse(src)
	if err != nil {
		panic(err)
	}
	return d
}

func kinds(d *Diagram) []itemKind {
	var ks []itemKind
	for _, it := range d.items {
		ks = append(ks, it.kind)
	}
	return ks
}

func TestParse(t *testing.T) {
	Convey("Given participants with aliases and an actor", t, func() {
		d := mustParse("sequenceDiagram\nactor U as User\nparticipant W as Web app\nU->>W: Open\nW-->>U: Page")

		Convey("Then ids, labels and kinds are kept in declared order", func() {
			So(d.Participants[0].ID, ShouldEqual, "U")
			So(d.Participants[0].Label, ShouldEqual, "User")
			So(d.Participants[0].Kind, ShouldEqual, KindActor)
			So(d.Participants[1].Label, ShouldEqual, "Web app")
			So(d.Participants[1].Kind, ShouldEqual, KindParticipant)
		})
		Convey("Then messages refer to ids", func() {
			So(d.Messages[0].From, ShouldEqual, "U")
			So(d.Messages[0].To, ShouldEqual, "W")
			So(d.Messages[1].Arrow.Dashed, ShouldBeTrue)
		})
	})

	Convey("Given undeclared participants", t, func() {
		d := mustParse("sequenceDiagram\nA->>B: hi\nB->>C: yo")
		Convey("Then they are created in first-seen order", func() {
			So([]string{d.Participants[0].ID, d.Participants[1].ID, d.Participants[2].ID}, ShouldResemble, []string{"A", "B", "C"})
		})
	})

	Convey("Given each arrow operator", t, func() {
		cases := []struct {
			op     string
			dashed bool
			head   Head
			both   bool
		}{
			{"->", false, HeadNone, false},
			{"-->", true, HeadNone, false},
			{"->>", false, HeadArrow, false},
			{"-->>", true, HeadArrow, false},
			{"-x", false, HeadCross, false},
			{"--x", true, HeadCross, false},
			{"-)", false, HeadOpen, false},
			{"--)", true, HeadOpen, false},
			{"<<->>", false, HeadArrow, true},
			{"<<-->>", true, HeadArrow, true},
		}
		for _, c := range cases {
			d := mustParse("sequenceDiagram\nA" + c.op + "B: x")
			So(d.Messages[0].Arrow, ShouldResemble, Arrow{Dashed: c.dashed, Head: c.head, Both: c.both})
			So(d.Messages[0].From, ShouldEqual, "A")
			So(d.Messages[0].To, ShouldEqual, "B")
		}
	})

	Convey("Given critical/option and par/and blocks", t, func() {
		d := mustParse("sequenceDiagram\ncritical c\nA->>B: 1\noption o\nA->>B: 2\nend\npar a\nA->>B: 3\nand b\nA->>C: 4\nend")
		Convey("Then both frames and their sections are recorded", func() {
			So(len(d.Frames), ShouldEqual, 2)
			So(d.Frames[0].Kind, ShouldEqual, "critical")
			So(d.Frames[0].Label, ShouldEqual, "c")
			So(d.Frames[0].Sections[0].Label, ShouldEqual, "o")
			So(d.Frames[1].Kind, ShouldEqual, "par")
			So(d.Frames[1].Sections[0].Label, ShouldEqual, "b")
			So(len(d.Messages), ShouldEqual, 4)
		})
	})

	Convey("Given create and destroy", t, func() {
		d := mustParse("sequenceDiagram\nA->>B: x\ncreate participant C as Carl\nB->>C: y\ndestroy C\nC->>B: z")
		Convey("Then the creating and destroying messages are marked", func() {
			c := d.participant("C")
			So(c.Label, ShouldEqual, "Carl")
			So(c.Created, ShouldBeTrue)
			So(c.Destroyed, ShouldBeTrue)
			So(d.Messages[1].Creates, ShouldEqual, "C")
			So(d.Messages[2].Destroys, ShouldResemble, []string{"C"})
		})
	})

	Convey("Given a destroy with no message after it", t, func() {
		d := mustParse("sequenceDiagram\nA->>B: x\ndestroy B")
		Convey("Then the lifeline still ends", func() {
			So(d.participant("B").Destroyed, ShouldBeTrue)
			So(kinds(d)[len(d.items)-1], ShouldEqual, itDestroy)
		})
	})

	Convey("Given boxes", t, func() {
		d := mustParse("sequenceDiagram\nbox Aqua Group one\nparticipant A\nparticipant B\nend\nbox rgb(1, 2, 3) Two\nparticipant C\nend\nbox Plain\nparticipant D\nend\nbox transparent Clear\nparticipant E\nend\nA->>E: x")
		Convey("Then colours are split from labels and validated", func() {
			So(len(d.Boxes), ShouldEqual, 4)
			So(d.Boxes[0].Color, ShouldEqual, "aqua")
			So(d.Boxes[0].Label, ShouldEqual, "Group one")
			So(len(d.Boxes[0].Members), ShouldEqual, 2)
			So(d.Boxes[1].Color, ShouldEqual, "rgb(1, 2, 3)")
			So(d.Boxes[1].Label, ShouldEqual, "Two")
			So(d.Boxes[2].Color, ShouldEqual, "")
			So(d.Boxes[2].Label, ShouldEqual, "Plain")
			So(d.Boxes[3].Color, ShouldEqual, "")
			So(d.Boxes[3].Label, ShouldEqual, "Clear")
			So(d.participant("C").Box, ShouldEqual, d.Boxes[1])
		})
	})

	Convey("Given a box nested inside a loop", t, func() {
		d := mustParse("sequenceDiagram\nloop every day\nbox Group\nparticipant A\nend\nA->>A: x\nend")
		Convey("Then the box's end closes the box, not the loop", func() {
			So(len(d.Frames), ShouldEqual, 1)
			So(kinds(d), ShouldResemble, []itemKind{itFrameStart, itMessage, itFrameEnd})
		})
	})

	Convey("Given rect colours", t, func() {
		Convey("A valid colour is a fill", func() {
			d := mustParse("sequenceDiagram\nrect rgb(0, 0, 255)\nA->>B: hi\nend")
			So(d.Frames[0].Color, ShouldEqual, "rgb(0, 0, 255)")
			So(d.Frames[0].Label, ShouldEqual, "")
		})
		Convey("An invalid colour is dropped, not drawn as text", func() {
			d := mustParse("sequenceDiagram\nrect rgb(0,0,255\" onload=x)\nA->>B: hi\nend")
			So(d.Frames[0].Color, ShouldEqual, "")
			So(d.Frames[0].Label, ShouldEqual, "")
		})
		Convey("A named colour works", func() {
			d := mustParse("sequenceDiagram\nrect LightYellow\nA->>B: hi\nend")
			So(d.Frames[0].Color, ShouldEqual, "lightyellow")
		})
	})

	Convey("Given activations", t, func() {
		d := mustParse("sequenceDiagram\nA->>+B: call\nactivate A\nB-->>-A: return\ndeactivate A")
		Convey("Then the shorthand and keywords are recorded", func() {
			So(d.Messages[0].Activate, ShouldBeTrue)
			So(d.Messages[1].Deactivate, ShouldBeTrue)
			So(kinds(d), ShouldResemble, []itemKind{itMessage, itActivate, itMessage, itDeactivate})
		})
	})

	Convey("Given notes", t, func() {
		d := mustParse("sequenceDiagram\nNote left of A: l\nnote RIGHT OF A: r\nNote over A, B: both")
		So(d.Notes[0].Pos, ShouldEqual, NoteLeft)
		So(d.Notes[1].Pos, ShouldEqual, NoteRight)
		So(d.Notes[2].Pos, ShouldEqual, NoteOver)
		So(d.Notes[2].Of, ShouldResemble, []string{"A", "B"})
		So(d.Notes[2].Text, ShouldEqual, "both")
	})

	Convey("Given autonumber operands", t, func() {
		d := mustParse("sequenceDiagram\nautonumber 10 10\nA->>B: one\nA->>B: two\nautonumber off\nA->>B: three")
		So(d.Messages[0].Num, ShouldEqual, 10)
		So(d.Messages[1].Num, ShouldEqual, 20)
		So(d.Messages[2].Num, ShouldEqual, 0)
	})

	Convey("Given a title, links and entity codes", t, func() {
		d := mustParse("sequenceDiagram\ntitle Checkout\nparticipant A\nlink A: Dashboard @ https://example.com\nlinks A: {\"x\": \"https://example.com\"}\nA->>B: a#59; b #quot;c#quot;")
		Convey("Then the title is kept, links are ignored and codes decoded", func() {
			So(d.Title, ShouldEqual, "Checkout")
			So(len(d.Messages), ShouldEqual, 1)
			So(d.Messages[0].Text, ShouldEqual, `a; b "c"`)
		})
	})

	Convey("Given semicolons and colons in message text", t, func() {
		d := mustParse("sequenceDiagram\nS->>S: place = hub.Where(id); spot(place): ok")
		So(d.Messages[0].Text, ShouldEqual, "place = hub.Where(id); spot(place): ok")
	})

	Convey("Given source without the header", t, func() {
		_, err := Parse("A->>B: hi")
		So(err, ShouldNotBeNil)
	})

	Convey("Given an unknown statement", t, func() {
		_, err := Parse("sequenceDiagram\nthis is not a message")
		So(err, ShouldNotBeNil)
	})
}
