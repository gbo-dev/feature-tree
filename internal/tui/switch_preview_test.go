package tui

import (
	"strings"
	"testing"
)

func TestParseSwitchLogEntriesParsesNumstatTotals(t *testing.T) {
	stdout := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\taaaa\t2 hours ago\tadd feature\n3\t1\ta.txt\n2\t0\tb.txt\n"
	entries := parseSwitchLogEntries(stdout)
	if len(entries) != 1 {
		t.Fatalf("parseSwitchLogEntries len = %d, want 1", len(entries))
	}
	if entries[0].shortHash != "aaaa" {
		t.Fatalf("shortHash = %q, want %q", entries[0].shortHash, "aaaa")
	}
	if entries[0].added != 5 || entries[0].deleted != 1 {
		t.Fatalf("numstat totals = +%d -%d, want +5 -1", entries[0].added, entries[0].deleted)
	}
}

func TestParseSwitchNumstatLineBinaryFile(t *testing.T) {
	added, deleted, ok := parseSwitchNumstatLine("-\t-\timage.png")
	if !ok {
		t.Fatalf("parseSwitchNumstatLine should parse binary-file markers")
	}
	if added != 0 || deleted != 0 {
		t.Fatalf("binary numstat should be zeroed, got +%d -%d", added, deleted)
	}
}

func TestLooksLikeFullCommitHash(t *testing.T) {
	if !looksLikeFullCommitHash("0123456789abcdef0123456789abcdef01234567") {
		t.Fatalf("expected valid full hash")
	}
	if looksLikeFullCommitHash("not-a-hash") {
		t.Fatalf("expected invalid hash to be rejected")
	}
}

func TestRenderSwitchLogTabUsesDiffHeader(t *testing.T) {
	text := renderSwitchLogTable([]switchLogEntry{{shortHash: "aaaa", added: 845, deleted: 11, age: "2h", subject: "test"}})
	if !strings.Contains(text, "DIFF") {
		t.Fatalf("expected DIFF header, got: %q", text)
	}
	if !strings.Contains(text, "+845") || !strings.Contains(text, "-11") {
		t.Fatalf("expected +845 and -11 in table, got: %q", text)
	}
}

func TestColorizeDiffStatLineKeepsAlignment(t *testing.T) {
	line := "README.md              |  2 +-"
	out := colorizeDiffStatLine(line)
	if !strings.Contains(out, "|  2") {
		t.Fatalf("expected aligned count column, got: %q", out)
	}
	if !strings.Contains(out, "\x1b[38;5;114m+\x1b[0m") {
		t.Fatalf("expected plus sign coloring, got: %q", out)
	}
	if !strings.Contains(out, "\x1b[38;5;203m-\x1b[0m") {
		t.Fatalf("expected minus sign coloring, got: %q", out)
	}
}
