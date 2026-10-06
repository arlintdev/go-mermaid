package theme

import (
	"reflect"
	"testing"
)

// empty lists the paths of the empty colors in v.
func empty(v reflect.Value, path string) []string {
	var out []string
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			out = append(out, empty(v.Field(i), path+"."+v.Type().Field(i).Name)...)
		}
	case reflect.String:
		if v.String() == "" {
			out = append(out, path)
		}
	case reflect.Slice:
		if v.Len() == 0 {
			out = append(out, path)
		}
		for i := 0; i < v.Len(); i++ {
			out = append(out, empty(v.Index(i), path)...)
		}
	}
	return out
}

func TestEveryPaletteIsComplete(t *testing.T) {
	for _, name := range append(Names(), "unknown") {
		if e := empty(reflect.ValueOf(For(name)), name); len(e) > 0 {
			t.Errorf("theme %s leaves colors empty: %v", name, e)
		}
	}
}

func TestBuiltInsAreComplete(t *testing.T) {
	for _, name := range []string{"default", "dark"} {
		if e := empty(reflect.ValueOf(palettes[name]), name); len(e) > 0 {
			t.Errorf("built-in %s leaves colors empty: %v", name, e)
		}
	}
}

func TestCustomPaletteFillsFromItsBackground(t *testing.T) {
	Register("test-night", Palette{Background: "#101010", Text: "#fafafa"})
	Register("test-day", Palette{Background: "#fdfdfd", NodeFill: "#eee"})
	night, day := For("test-night"), For("test-day")
	if night.NoteFill != palettes["dark"].NoteFill || night.Gantt.TaskFill != palettes["dark"].Gantt.TaskFill {
		t.Errorf("a dark custom palette should take the dark theme's colors, got note %s", night.NoteFill)
	}
	if night.Text != "#fafafa" {
		t.Errorf("a custom color was replaced: %s", night.Text)
	}
	if day.NoteFill != palettes["default"].NoteFill || day.NodeFill != "#eee" {
		t.Errorf("a light custom palette should take the default theme's colors, got note %s", day.NoteFill)
	}
	if night.ClusterFill != "#101010" {
		t.Errorf("a custom palette's group fill follows its background, got %s", night.ClusterFill)
	}
}

func TestOver(t *testing.T) {
	p := For("default").Over(Palette{NodeFill: "#123456", Gantt: GanttColors{TaskFill: "#abcdef"}})
	if p.NodeFill != "#123456" || p.Gantt.TaskFill != "#abcdef" {
		t.Errorf("Over did not put the colors over: %s %s", p.NodeFill, p.Gantt.TaskFill)
	}
	if p.Text != palettes["default"].Text || p.Gantt.TaskStroke != palettes["default"].Gantt.TaskStroke {
		t.Error("Over replaced colors the overlay left empty")
	}
}

func TestEscaped(t *testing.T) {
	p := Palette{Text: `"><x`, Pie: PieColors{Slices: []string{"<a>"}}}
	e := p.Escaped()
	if e.Text != "&quot;&gt;&lt;x" || e.Pie.Slices[0] != "&lt;a&gt;" {
		t.Errorf("not escaped: %q %q", e.Text, e.Pie.Slices[0])
	}
	if p.Pie.Slices[0] != "<a>" {
		t.Error("Escaped changed the palette it was called on")
	}
}

func TestColorMaths(t *testing.T) {
	tests := []struct {
		name, got, want string
	}{
		{"mix halfway", Mix("#000000", "#ffffff", 0.5, "x"), "#808080"},
		{"mix a short hex", Mix("#fff", "#000", 0.25, "x"), "#c0c0c0"},
		{"mix a non-color", Mix("url(#a)", "#000", 0.5, "fallback"), "fallback"},
		{"darken", Darken("#ffffff"), "#aaaaaa"},
		{"darken a non-color", Darken("none"), "none"},
		{"text that reads stays", TextOn("#ECECFF", "#333333"), "#333333"},
		{"light text on a pale fill turns dark", TextOn("#ECECFF", "#e6e6e6"), "#333333"},
		{"dark text on a dark fill turns light", TextOn("navy", "#333333"), "#e6e6e6"},
		{"an unknown fill keeps the text", TextOn("transparent", "#e6e6e6"), "#e6e6e6"},
		{"a pale line darkens on a light page", LineOn("#ececff", "#ffffff"), "#9d9daa"},
		{"a pale line stays on a dark page", LineOn("#ececff", "#1e1e1e"), "#ececff"},
		{"a strong line stays", LineOn("#8493a6", "#ffffff"), "#8493a6"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, tc.got, tc.want)
		}
	}
	if !IsDark("#1e1e1e") || IsDark("#ffffff") || IsDark("not a color") {
		t.Error("IsDark is wrong")
	}
	if op := BackdropOpacity("rgb(200, 220, 255)", "#e6e6e6"); op >= 1 {
		t.Errorf("a pale backdrop under light text should be translucent, got %v", op)
	}
	if op := BackdropOpacity("rgb(200, 220, 255)", "#333333"); op != 1 {
		t.Errorf("a pale backdrop under dark text stays as it is, got %v", op)
	}
}

func TestNode(t *testing.T) {
	p := For("dark")
	f, s, tx := p.Node("", "", "")
	if f != p.NodeFill || s != p.NodeStroke || tx != p.Text {
		t.Error("an unstyled node takes the palette's colors")
	}
	if _, _, tx = p.Node("#ffe0e0", "", ""); tx != palettes["default"].Text {
		t.Errorf("a pale styled fill gets dark text, got %s", tx)
	}
	if _, _, tx = p.Node("#ffe0e0", "", "red"); tx != "red" {
		t.Errorf("a styled text color wins, got %s", tx)
	}
}
