package tui

import (
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func TestParseSelectedBranch(t *testing.T) {
	branch, err := parseSelectedBranch("display\tfeature/test")
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
