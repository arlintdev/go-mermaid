package cssval

import "testing"

func TestColor(t *testing.T) {
	good := map[string]string{"#FFF": "#fff", "#ff000080": "#ff000080", "Red": "red", " rgb(1, 2, 3) ": "rgb(1, 2, 3)", "hsl(120deg 50% 50%)": "hsl(120deg 50% 50%)"}
	for in, want := range good {
		if got, ok := Color(in); !ok || got != want {
			t.Errorf("Color(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "#ggg", "red;x", `red" onload="x`, "url(#a)", "expression(1)", "#12345", "rgb(1,2,3);fill:red"} {
		if got, ok := Color(in); ok {
			t.Errorf("Color(%q) accepted as %q", in, got)
		}
	}
}

func TestLengthsAndDashes(t *testing.T) {
	if v, ok := Length("2px"); !ok || v != "2px" {
		t.Errorf("Length(2px) = %q, %v", v, ok)
	}
	if _, ok := Length("2px;x"); ok {
		t.Error("Length accepted a trailing declaration")
	}
	if v, ok := Dash("5, 5"); !ok || v != "5 5" {
		t.Errorf("Dash(5, 5) = %q, %v", v, ok)
	}
	if _, ok := Dash(`5 "x`); ok {
		t.Error("Dash accepted a quote")
	}
	if v, ok := Pixels("10px", 100); !ok || v != 10 {
		t.Errorf("Pixels(10px) = %v, %v", v, ok)
	}
	if _, ok := Pixels("1e9", 100); ok {
		t.Error("Pixels accepted a value over the limit")
	}
	if v, ok := Opacity("50%"); !ok || v != "0.5" {
		t.Errorf("Opacity(50%%) = %q, %v", v, ok)
	}
}
