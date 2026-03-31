package textwidth

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestWidth(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{name: "ascii", in: "abc", want: 3},
		{name: "combining", in: "a\u0301", want: 1},
		{name: "cjk", in: "你", want: 2},
		{name: "ellipsis", in: Ellipsis, want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Width(tc.in)
			if got != tc.want {
				t.Fatalf("Width(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "ascii", in: "feature-branch", max: 8, want: "feature…"},
		{name: "combining", in: "a\u0301bc", max: 2, want: "a\u0301…"},
		{name: "cjk", in: "你好世界", max: 5, want: "你好…"},
		{name: "zero max", in: "abc", max: 0, want: ""},
		{name: "ellipsis only", in: "abc", max: 1, want: "…"},
		{name: "unchanged", in: "plain", max: 5, want: "plain"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.in, tc.max)
			if got != tc.want {
				t.Fatalf("Truncate(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestTruncateKeepsValidUTF8AndGraphemeBoundaries(t *testing.T) {
	in := "a👨‍👩‍👧‍👦b"
	out := Truncate(in, 3)
	if !utf8.ValidString(out) {
		t.Fatalf("Truncate produced invalid UTF-8: %q", out)
	}
	if Width(out) > 3 {
		t.Fatalf("Truncate width = %d, want <= 3", Width(out))
	}
	if !strings.HasSuffix(out, Ellipsis) {
		t.Fatalf("Truncate output = %q, expected ellipsis suffix", out)
	}
}
