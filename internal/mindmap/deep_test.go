package mindmap

import (
	"strings"
	"testing"
	"time"
)

// A very deep map lays out in linear time.
func TestDeepMapIsFast(t *testing.T) {
	var b strings.Builder
	b.WriteString("mindmap\n")
	for i := 0; i < 3000; i++ {
		b.WriteString(strings.Repeat(" ", i+1) + "n\n")
	}
	start := time.Now()
	if _, err := Render(b.String(), RenderOptions{FontSize: 14}); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}
