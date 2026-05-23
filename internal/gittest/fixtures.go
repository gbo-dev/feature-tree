package gittest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

// SetupClonedRepo clones a bare remote into a bare-in-.git layout under base/targetDirName.
func SetupClonedRepo(t *testing.T, bare testutil.BareFixture, targetDirName string) *gitx.CloneResult {
	t.Helper()

	target := filepath.Join(bare.BaseDir, targetDirName)
	result, err := gitx.CloneRepo(context.Background(), bare.BareDir, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}
	return result
}

// SetupClonedRepoFromBare creates a bare remote and clones it into base/repo.
func SetupClonedRepoFromBare(t *testing.T) *gitx.CloneResult {
	t.Helper()
	bare := testutil.SetupBareRemote(t)
	return SetupClonedRepo(t, bare, "repo")
}

// AddWorktree adds a branch worktree alongside the default-branch worktree.
func AddWorktree(t *testing.T, clone *gitx.CloneResult, branch string) string {
	t.Helper()

	worktreePath := filepath.Join(clone.RepoRoot, branch)
	testutil.RunGit(t, "", "--git-dir", clone.GitCommonDir, "worktree", "add", "-b", branch, worktreePath, clone.DefaultBranch)
	return worktreePath
}

// SetupClonedRepoWithWorktree clones a bare remote and adds a feature worktree.
func SetupClonedRepoWithWorktree(t *testing.T, branch string) (*gitx.CloneResult, string) {
	t.Helper()

	clone := SetupClonedRepoFromBare(t)
	path := AddWorktree(t, clone, branch)
	return clone, path
}

// RepoContextFromClone builds a standard RepoContext for tests.
func RepoContextFromClone(clone *gitx.CloneResult) *gitx.RepoContext {
	return &gitx.RepoContext{
		RepoRoot:      clone.RepoRoot,
		GitCommonDir:  clone.GitCommonDir,
		DefaultBranch: clone.DefaultBranch,
		IncludeFile:   ".worktreeinclude",
	}
}
