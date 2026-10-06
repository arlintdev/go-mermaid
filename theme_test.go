package mermaid_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	mermaid "github.com/arlintdev/go-mermaid"
	"github.com/arlintdev/go-mermaid/internal/goldentest"
)

// themeInputs are the diagrams the packages test with: the corpus, the
// single-feature probes and each package's own cases.
func themeInputs(t *testing.T) []string {
	t.Helper()
	inputs, err := filepath.Glob(filepath.Join("internal", "*", "testdata", "*.mmd"))
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no diagram inputs: %v", err)
	}
	return inputs
}

// TestGoldenDark renders every package's test diagrams in the dark theme on
// a transparent background, as a page in dark mode shows them, checks the
// output against the allow-list, and compares it with testdata/dark. Run
// with -update (or GOLDEN_UPDATE=1) to rewrite the golden files.
func TestGoldenDark(t *testing.T) {
	regen := *update || os.Getenv("GOLDEN_UPDATE") == "1"
	for _, in := range themeInputs(t) {
		pkg := filepath.Base(filepath.Dir(filepath.Dir(in)))
		name := pkg + "_" + strings.TrimSuffix(filepath.Base(in), ".mmd")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			got, err := mermaid.Render(string(src), mermaid.WithTheme(mermaid.Dark), mermaid.WithTransparentBackground())
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			if err := goldentest.CheckSVG(got); err != nil {
				t.Fatalf("output is not plain static SVG: %v", err)
			}
			path := filepath.Join("testdata", "dark", name+".svg")
			if regen {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file (run with -update): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("output differs from %s; if the change is intended, rerun with -update and review it", path)
			}
		})
	}
}

// TestBothThemesArePlainSVG renders every test diagram in the light and the
// dark theme, with and without a background, and checks each against the
// allow-list.
func TestBothThemesArePlainSVG(t *testing.T) {
	for _, in := range themeInputs(t) {
		src, err := os.ReadFile(in)
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range [][]mermaid.Option{
			nil,
			{mermaid.WithTransparentBackground()},
			{mermaid.WithTheme(mermaid.Dark)},
			{mermaid.WithTheme(mermaid.Dark), mermaid.WithTransparentBackground()},
		} {
			got, err := mermaid.Render(string(src), opts...)
			if err != nil {
				t.Errorf("%s: render: %v", in, err)
				continue
			}
			if err := goldentest.CheckSVG(got); err != nil {
				t.Errorf("%s: not plain static SVG: %v", in, err)
			}
		}
	}
}
