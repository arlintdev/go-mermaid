package svgutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWrap(t *testing.T) {
	long := "A very long label that goes on and on to see how the renderer wraps or does not wrap long text"
	lines := FaceSans.Wrap(long, 16, 200)
	if len(lines) < 3 {
		t.Fatalf("want several lines, got %q", lines)
	}
	for _, l := range lines {
		if w := FaceSans.Width(l, 16); w > 200 {
			t.Errorf("line %q is %.0f wide, over 200", l, w)
		}
	}
	if got := strings.Join(lines, " "); got != long {
		t.Errorf("wrapping lost words: %q", got)
	}
	for _, c := range []struct {
		in   string
		want int
	}{
		{"one<br>two", 2}, {"one<BR/>two", 2}, {"one<br />two", 2}, {`one\ntwo`, 2}, {"one\ntwo\nthree", 3}, {"", 1},
	} {
		if got := FaceSans.Wrap(c.in, 16, 200); len(got) != c.want {
			t.Errorf("Wrap(%q) = %q, want %d lines", c.in, got, c.want)
		}
	}
	if got := FaceSans.Wrap(strings.Repeat("x", 200), 16, 100); len(got) != 1 {
		t.Errorf("a long word is never cut between letters, got %q", got)
	}
	url := "https://identity.example.com/realms/organisation/protocol/openid-connect/auth"
	got := FaceSans.Wrap(url, 16, 150)
	if len(got) < 2 || strings.Join(got, "") != url {
		t.Errorf("a far too wide path breaks after its slashes, got %q", got)
	}
	for _, l := range got[:len(got)-1] {
		if !strings.HasSuffix(l, "/") && !strings.HasSuffix(l, "-") {
			t.Errorf("line %q does not end at a slash or hyphen", l)
		}
	}
	if got := FaceSans.Wrap("DIGIN_DATA_DIR/files", 16, 120); len(got) != 1 {
		t.Errorf("a word a little too wide stays whole, got %q", got)
	}
}

func TestWrapBreaksAfterHyphens(t *testing.T) {
	got := FaceSans.Wrap("OnFailure=: digin-backup-failed.service, make backup-failed", 16, 120)
	for _, l := range got {
		if FaceSans.Width(l, 16) > 120*1.5 {
			t.Errorf("line %q too wide", l)
		}
	}
	joined := strings.Join(got, "")
	if strings.ReplaceAll(joined, " ", "") != strings.ReplaceAll("OnFailure=: digin-backup-failed.service, make backup-failed", " ", "") {
		t.Errorf("lost text: %q", got)
	}
	found := false
	for _, l := range got {
		if strings.HasSuffix(l, "-") {
			found = true
		}
	}
	if !found {
		t.Errorf("no break after a hyphen: %q", got)
	}
}

func TestWrapHard(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  float64
		want []string
	}{
		{"words fill each line", "The internal Microsoft Exchange e-mail system.", 220,
			[]string{"The internal Microsoft Exchange", "e-mail system."}},
		{"explicit breaks", "a<br>b", 200, []string{"a", "b"}},
		{"empty text", "", 200, []string{""}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FaceSans.WrapHard(tc.in, 14, tc.max)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("WrapHard(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for _, l := range got {
				if w := FaceSans.Width(l, 14); w > tc.max {
					t.Errorf("line %q is %.1f wide, over %.0f", l, w, tc.max)
				}
			}
		})
	}
}

func TestCutIsLinear(t *testing.T) {
	word := strings.Repeat("́", 50000) + strings.Repeat("x", 50000)
	got := FaceSans.WrapHard(word, 14, 100)
	if strings.Join(got, "") != word {
		t.Error("cutting lost text")
	}
}

func TestWrapHardLongTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  float64
		want []string
	}{
		{"a word wider than the box stays whole", "xxxxxxxxxxxxxxxxxxxx", 60, []string{"xxxxxxxxxxxxxxxxxxxx"}},
		{"a path breaks after its slashes", "/var/lib/app/data", 60, []string{"/var/lib/", "app/data"}},
		{"an identifier breaks after underscores", "svc_auth_gateway_primary", 90, []string{"svc_auth_", "gateway_", "primary"}},
		{"a short word fits beside it", "see /.well-known/oauth-protected-resource", 120,
			[]string{"see /.well-known/", "oauth-protected-", "resource"}},
		{"wide characters break anywhere", "東京都渋谷区神南一丁目", 60, []string{"東京都渋", "谷区神南", "一丁目"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FaceSans.WrapHard(tc.in, 14, tc.max)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("WrapHard(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestWrapNeverSplitsLetters checks every line break of both wrappers falls
// at a space, after '/', '-' or '_', or beside a wide character.
func TestWrapNeverSplitsLetters(t *testing.T) {
	texts := []string{
		"401, WWW-Authenticate points to /.well-known/oauth-protected-resource",
		"authorization_servers: https://auth.example.com/realms/main",
		strings.Repeat("abcdefghij", 40) + " tail",
		"DIGIN_DATA_DIR/files/attachments/2024/very-long-file-name_with_parts.tar.gz",
		"日本語のテキストは空白なしで続きます",
	}
	for _, text := range texts {
		for _, w := range []float64{30, 60, 120, 200} {
			for name, lines := range map[string][]string{
				"Wrap":     FaceSans.Wrap(text, 14, w),
				"WrapHard": FaceSans.WrapHard(text, 14, w),
			} {
				if strings.Join(strings.Fields(strings.Join(lines, " ")), "") != strings.Join(strings.Fields(text), "") {
					t.Errorf("%s(%q, %v) lost text: %q", name, text, w, lines)
				}
				rest := text
				for _, l := range lines[:len(lines)-1] {
					rest = strings.TrimLeft(rest[len(l):], " ")
					if strings.HasPrefix(text[len(text)-len(rest)-1:], " ") {
						continue
					}
					last, _ := utf8.DecodeLastRuneInString(l)
					next, _ := utf8.DecodeRuneInString(rest)
					if !isSoftBreak(last) && !inRanges(wideRanges, last) && !inRanges(wideRanges, next) {
						t.Errorf("%s(%q, %v) split %q from %q", name, text, w, l, rest)
					}
				}
			}
		}
	}
}
