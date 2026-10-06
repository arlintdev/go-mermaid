package gantt

import "testing"

func FuzzParse(f *testing.F) {
	f.Add("gantt\ntitle P\ndateFormat YYYY-MM-DD\nsection S\nA : a1, 2024-01-01, 10d\nB : after a1, 2w")
	f.Add("gantt\nA : 5d")
	f.Add("gantt\nsection\n: : :")
	f.Add("gantt")
	f.Add("gantt\nexcludes weekends\naxisFormat %b %-d\ntickInterval 1week\nA : a, 2024-01-01, 1.5d\nB : crit, active, after a, until a\nC : milestone, vert, 2024-01-03, 0d")
	f.Fuzz(func(_ *testing.T, src string) {
		if d, err := Parse(src); err == nil {
			d.Bounds() // must not panic on resolved data
		}
		_, _ = Render(src, RenderOptions{FontSize: 14, Padding: 16})
	})
}
