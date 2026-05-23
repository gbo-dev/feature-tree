package gittest

import (
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestSetupClonedRepoFromBareBootstrapsMainWorktree(t *testing.T) {
	clone := SetupClonedRepoFromBare(t)

	if clone.DefaultBranch != "main" {
		t.Fatalf("default branch = %q, want main", clone.DefaultBranch)
	}
	if clone.GitCommonDir != filepath.Join(clone.RepoRoot, ".git") {
		t.Fatalf("GitCommonDir = %q, want %q", clone.GitCommonDir, filepath.Join(clone.RepoRoot, ".git"))
	}
	if clone.WorktreePath != filepath.Join(clone.RepoRoot, "main") {
		t.Fatalf("WorktreePath = %q, want %q", clone.WorktreePath, filepath.Join(clone.RepoRoot, "main"))
	}

	headBranch := testutil.RunGit(t, clone.WorktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if headBranch != "main" {
		t.Fatalf("worktree HEAD = %q, want main", headBranch)
	}
}

func TestSetupClonedRepoWithWorktreeAddsBranch(t *testing.T) {
	const branch = "feature-fixture-test"

	clone, worktreePath := SetupClonedRepoWithWorktree(t, branch)

	wantPath := filepath.Join(clone.RepoRoot, branch)
	if worktreePath != wantPath {
		t.Fatalf("worktree path = %q, want %q", worktreePath, wantPath)
	}

	current := testutil.RunGit(t, worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if current != branch {
		t.Fatalf("worktree HEAD = %q, want %q", current, branch)
	}
}

func TestRepoContextFromCloneSetsIncludeFile(t *testing.T) {
	clone := SetupClonedRepoFromBare(t)
	ctx := RepoContextFromClone(clone)

	if ctx.IncludeFile != ".worktreeinclude" {
		t.Fatalf("IncludeFile = %q, want .worktreeinclude", ctx.IncludeFile)
	}
	if ctx.RepoRoot != clone.RepoRoot || ctx.GitCommonDir != clone.GitCommonDir {
		t.Fatalf("RepoContext paths mismatch clone result")
	}
}
