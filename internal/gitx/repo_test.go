package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestDetectDefaultBranchFallsBackToMainWhenOriginHeadMissing(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	bare := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, bare)

	got, err := detectDefaultBranch(bare)
	if err != nil {
		t.Fatalf("detectDefaultBranch returned error: %v", err)
	}
	if got != "main" {
		t.Fatalf("detectDefaultBranch = %q, want %q", got, "main")
	}
}

func TestDetectDefaultBranchFailsWhenOriginHeadMissingAndNoFallbackBranches(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.RunGit(t, "", "init", source)

	filePath := filepath.Join(source, "README.md")
	if err := os.WriteFile(filePath, []byte("test repo\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
	testutil.RunGit(t, source, "add", "README.md")
	testutil.RunGit(t, source, "commit", "-m", "initial commit")
	testutil.RunGit(t, source, "branch", "-M", "develop")

	bare := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, bare)

	_, err := detectDefaultBranch(bare)
	if err == nil {
		t.Fatalf("detectDefaultBranch expected error when no origin/HEAD and no fallback branches")
	}
	if !strings.Contains(err.Error(), "could not determine default branch") {
		t.Fatalf("detectDefaultBranch error = %q, expected fail-fast message", err.Error())
	}
}
