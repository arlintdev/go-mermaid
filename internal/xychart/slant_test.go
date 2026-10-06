package xychart

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/arlintdev/go-mermaid/internal/svgutil"
	. "github.com/smartystreets/goconvey/convey"
)

var slanted = regexp.MustCompile(`<text x="([0-9.]+)" y="[0-9.]+" [^>]*rotate\(-45[^>]*>([^<]*)</text>`)

func TestSlantedLabelStaysOnCanvas(t *testing.T) {
	Convey("Given a column chart whose first category is one very long word", t, func() {
		src := "xychart-beta\nx-axis [Pneumonoultramicroscopicsilicovolcanoconiosisdiagnosis, b, c]\nbar [1, 2, 3]"
		out, err := Render(src, opts())
		So(err, ShouldBeNil)

		Convey("Then the labels are slanted and none starts left of the canvas", func() {
			m := slanted.FindAllStringSubmatch(string(out), -1)
			So(len(m), ShouldEqual, 3)
			face := svgutil.FaceFor("sans-serif")
			for _, l := range m {
				x, _ := strconv.ParseFloat(l[1], 64)
				reach := face.Width(l[2], 14) * math.Sqrt2 / 2
				So(x-reach, ShouldBeGreaterThanOrEqualTo, 0)
			}
		})
	})
}

var upright = regexp.MustCompile(`<text x="([0-9.]+)" y="[0-9.]+" [^>]*rotate\(-90[^>]*>(m[0-9]+)</text>`)

func TestCrowdedLabelsStandUpright(t *testing.T) {
	Convey("Given a column chart with forty categories", t, func() {
		src, err := os.ReadFile("testdata/stress_many.mmd")
		So(err, ShouldBeNil)
		out, err := Render(string(src), opts())
		So(err, ShouldBeNil)

		Convey("Then the labels stand upright, at least a line apart, none slanted", func() {
			So(string(out), ShouldNotContainSubstring, "rotate(-45")
			m := upright.FindAllStringSubmatch(string(out), -1)
			So(len(m), ShouldBeGreaterThan, 10)
			for i := 1; i < len(m); i++ {
				a, _ := strconv.ParseFloat(m[i-1][1], 64)
				b, _ := strconv.ParseFloat(m[i][1], 64)
				So(b-a, ShouldBeGreaterThanOrEqualTo, 14)
			}
		})
	})
}
