package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickerPreviewCommandReadsCacheFile(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := filepath.Join(tmpDir, "preview.txt")
	if err := os.WriteFile(cacheFile, []byte("hello preview\n"), 0o600); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	stdout, stderr, err := runRootCommand(t, "", "__picker-preview", cacheFile)
	if err != nil {
		t.Fatalf("__picker-preview returned error: %v", err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("__picker-preview stderr = %q, want empty", stderr)
	}
	if stdout != "hello preview\n" {
		t.Fatalf("__picker-preview stdout = %q, want %q", stdout, "hello preview\n")
	}
}

func TestPickerPreviewStateCommandCyclesTabs(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "tab-state")
	if err := os.WriteFile(stateFile, []byte("1"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	_, _, err := runRootCommand(t, "", "__picker-preview-state", "--state-file", stateFile, "--step", "1")
	if err != nil {
		t.Fatalf("__picker-preview-state returned error: %v", err)
	}

	out, readErr := os.ReadFile(stateFile)
	if readErr != nil {
		t.Fatalf("read state file: %v", readErr)
	}
	if strings.TrimSpace(string(out)) != "2" {
		t.Fatalf("state file = %q, want %q", strings.TrimSpace(string(out)), "2")
	}
}

func TestPickerPreviewTabCommandReadsCurrentTabFile(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "tab-state")
	if err := os.WriteFile(stateFile, []byte("3"), 0o600); err != nil {
		t.Fatalf("write state file: %v", err)
	}

	tab1 := filepath.Join(tmpDir, "tab1.txt")
	tab2 := filepath.Join(tmpDir, "tab2.txt")
	tab3 := filepath.Join(tmpDir, "tab3.txt")
	tab4 := filepath.Join(tmpDir, "tab4.txt")

	if err := os.WriteFile(tab1, []byte("one"), 0o600); err != nil {
		t.Fatalf("write tab1: %v", err)
	}
	if err := os.WriteFile(tab2, []byte("two"), 0o600); err != nil {
		t.Fatalf("write tab2: %v", err)
	}
	if err := os.WriteFile(tab3, []byte("three"), 0o600); err != nil {
		t.Fatalf("write tab3: %v", err)
	}
	if err := os.WriteFile(tab4, []byte("four"), 0o600); err != nil {
		t.Fatalf("write tab4: %v", err)
	}

	stdout, stderr, err := runRootCommand(t, "", "__picker-preview-tab", "--state-file", stateFile, tab1, tab2, tab3, tab4)
	if err != nil {
		t.Fatalf("__picker-preview-tab returned error: %v", err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("__picker-preview-tab stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "vs. default") {
		t.Fatalf("__picker-preview-tab stdout missing tab header labels: %q", stdout)
	}
	if !strings.Contains(stdout, "[tab/s-tab]") {
		t.Fatalf("__picker-preview-tab stdout missing key hint: %q", stdout)
	}
	if !strings.Contains(stdout, "\x1b[48;2;26;46;44m") {
		t.Fatalf("__picker-preview-tab stdout missing active-tab background color: %q", stdout)
	}
	if !strings.HasSuffix(stdout, "three") {
		t.Fatalf("__picker-preview-tab stdout should end with selected tab content, got: %q", stdout)
	}
	if strings.Contains(stdout, "3/4") {
		t.Fatalf("__picker-preview-tab should not show preview row counter, got: %q", stdout)
	}
}
