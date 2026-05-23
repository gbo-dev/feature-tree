package tui

import (
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/textwidth"
)

func stripANSI(s string) string {
	for strings.Contains(s, "\x1b[") {
		start := strings.Index(s, "\x1b[")
		end := strings.Index(s[start:], "m")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return s
}

func displayPart(line string) string {
	if idx := strings.Index(line, "\t"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func TestBuildRowPrefix(t *testing.T) {
	tests := []struct {
		name  string
		row   pickerRow
		plain string
	}{
		{
			name:  "no markers",
			row:   pickerRow{},
			plain: "  ",
		},
		{
			name:  "current only",
			row:   pickerRow{current: true},
			plain: " @",
		},
		{
			name:  "default branch only",
			row:   pickerRow{marker: "^"},
			plain: " ^",
		},
		{
			name:  "mismatch only",
			row:   pickerRow{branchMismatch: true},
			plain: "~ ",
		},
		{
			name:  "current and mismatch",
			row:   pickerRow{current: true, branchMismatch: true},
			plain: "~@",
		},
		{
			name:  "default branch and mismatch",
			row:   pickerRow{marker: "^", branchMismatch: true},
			plain: "~^",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildRowPrefix(tc.row)
			if plain := stripANSI(got); plain != tc.plain {
				t.Fatalf("buildRowPrefix plain = %q, want %q (raw %q)", plain, tc.plain, got)
			}
			if width := textwidth.Width(stripANSI(got)); width != 2 {
				t.Fatalf("buildRowPrefix width = %d, want 2", width)
			}
		})
	}
}

func TestParseSelectedBranch(t *testing.T) {
	branch, err := parseSelectedBranch("display\tfeature/test")
	if err != nil {
		t.Fatalf("parseSelectedBranch returned error: %v", err)
	}
	if branch != "feature/test" {
		t.Fatalf("parseSelectedBranch = %q, want %q", branch, "feature/test")
	}
}

func TestParseSelectedBranchIgnoresExtendedHiddenPayload(t *testing.T) {
	branch, err := parseSelectedBranch("display\tfeature/test\t/tmp/head.txt\t/tmp/log.txt")
	if err != nil {
		t.Fatalf("parseSelectedBranch returned error: %v", err)
	}
	if branch != "feature/test" {
		t.Fatalf("parseSelectedBranch = %q, want %q", branch, "feature/test")
	}
}

func TestParseSelectedBranchRejectsMissingPayload(t *testing.T) {
	_, err := parseSelectedBranch("display only")
	if err == nil {
		t.Fatalf("parseSelectedBranch expected error for missing payload")
	}
}

func TestBuildFZFLinesEmitsHiddenBranchPayload(t *testing.T) {
	rows := []pickerRow{
		{
			branch:   "feature/demo",
			commit:   gitx.CommitInfo{Hash: "abcd", Subject: "demo subject"},
			path:     "../feature-demo",
			state:    "clean",
			relation: "A: 1 B: 0",
			current:  true,
		},
	}

	layout := computeLayout(rows)
	lines := buildFZFLines(rows, layout)
	if len(lines) != 1 {
		t.Fatalf("buildFZFLines len = %d, want 1", len(lines))
	}
	if !strings.HasSuffix(lines[0], "\tfeature/demo") {
		t.Fatalf("buildFZFLines output missing hidden payload: %q", lines[0])
	}
}

func TestBuildFZFLinesAppendsHiddenFields(t *testing.T) {
	rows := []pickerRow{
		{
			branch:   "feature/demo",
			commit:   gitx.CommitInfo{Hash: "abcd", Subject: "demo subject"},
			path:     "../feature-demo",
			state:    "clean",
			relation: "A: 1 B: 0",
			hidden:   []string{"/tmp/head.txt", "/tmp/log.txt"},
		},
	}

	layout := computeLayout(rows)
	lines := buildFZFLines(rows, layout)
	if len(lines) != 1 {
		t.Fatalf("buildFZFLines len = %d, want 1", len(lines))
	}
	if !strings.HasSuffix(lines[0], "\tfeature/demo\t/tmp/head.txt\t/tmp/log.txt") {
		t.Fatalf("buildFZFLines output missing appended hidden fields: %q", lines[0])
	}
}

func TestBuildFZFLinesShowsBranchPathMismatchMarker(t *testing.T) {
	rows := []pickerRow{
		{
			branch:         "other-branch",
			path:           "../feature-a",
			state:          "clean",
			relation:       "A: 0 B: 0",
			branchMismatch: true,
		},
	}

	layout := computeLayout(rows)
	lines := buildFZFLines(rows, layout)
	if len(lines) != 1 {
		t.Fatalf("buildFZFLines len = %d, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "~") {
		t.Fatalf("buildFZFLines output missing mismatch marker: %q", lines[0])
	}
}

func TestBuildFZFLinesShowsCurrentAndMismatchMarkersTogether(t *testing.T) {
	rows := []pickerRow{
		{
			branch:         "other-branch",
			path:           ".",
			state:          "clean",
			relation:       "A: 0 B: 0",
			current:        true,
			branchMismatch: true,
		},
	}

	layout := computeLayout(rows)
	lines := buildFZFLines(rows, layout)
	if len(lines) != 1 {
		t.Fatalf("buildFZFLines len = %d, want 1", len(lines))
	}
	display := displayPart(lines[0])
	plain := stripANSI(display)
	if !strings.Contains(plain, "@") || !strings.Contains(plain, "~") {
		t.Fatalf("buildFZFLines output missing current and mismatch markers: %q", lines[0])
	}
	idxTilde := strings.Index(plain, "~")
	idxAt := strings.Index(plain, "@")
	if idxTilde < 0 || idxAt < 0 || idxTilde > idxAt {
		t.Fatalf("buildFZFLines mismatch marker should precede current marker, got plain %q", plain)
	}
	if strings.Contains(plain, "@~") {
		t.Fatalf("buildFZFLines should not render @ before ~, got plain %q", plain)
	}
}

func TestBuildFZFLinesKeepsBranchColumnAlignedWithMismatch(t *testing.T) {
	rows := []pickerRow{
		{
			branch:   "main",
			path:     ".",
			state:    "clean",
			relation: "A: 0 B: 0",
		},
		{
			branch:         "other-branch",
			path:           "../feature-a",
			state:          "clean",
			relation:       "A: 0 B: 0",
			current:        true,
			branchMismatch: true,
		},
	}

	layout := computeLayout(rows)
	lines := buildFZFLines(rows, layout)
	if len(lines) != 2 {
		t.Fatalf("buildFZFLines len = %d, want 2", len(lines))
	}

	mainIdx := strings.Index(stripANSI(displayPart(lines[0])), "main")
	mismatchIdx := strings.Index(stripANSI(displayPart(lines[1])), "other-branch")
	if mainIdx < 0 || mismatchIdx < 0 {
		t.Fatalf("buildFZFLines missing branch names: %q %q", lines[0], lines[1])
	}
	if mainIdx != mismatchIdx {
		t.Fatalf("branch column misaligned: main at %d, mismatch at %d\n%q\n%q", mainIdx, mismatchIdx, lines[0], lines[1])
	}
}

func TestFitListLayoutKeepsLineWithinLimit(t *testing.T) {
	layout := rowLayout{
		branchWidth:   120,
		pathWidth:     90,
		stateWidth:    20,
		relationWidth: 24,
		commitWidth:   90,
	}

	fitted := fitListLayout(layout)
	if lineWidth(fitted) > listLineMaxWidth {
		t.Fatalf("line width after fit = %d, want <= %d", lineWidth(fitted), listLineMaxWidth)
	}
}
