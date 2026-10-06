package gantt

import (
	"strings"
	"testing"
	"time"
)

// Many long tasks over excluded days must fail fast, not walk for ever.
func TestExclusionWalkIsBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString("gantt\nexcludes weekends\n")
	for i := 0; i < 500; i++ {
		b.WriteString("T : 2000-01-01, 70000d\n")
	}
	start := time.Now()
	_, _ = Parse(b.String())
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}
