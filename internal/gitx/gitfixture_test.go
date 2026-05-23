package gitx

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func setupClonedRepo(t *testing.T, bare testutil.BareFixture, targetDirName string) *CloneResult {
	t.Helper()

	target := filepath.Join(bare.BaseDir, targetDirName)
	result, err := CloneRepo(context.Background(), bare.BareDir, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}
	return result
}

func setupClonedRepoFromBare(t *testing.T) *CloneResult {
	t.Helper()
	bare := testutil.SetupBareRemote(t)
	return setupClonedRepo(t, bare, "repo")
}

func setupClonedRepoWithWorktree(t *testing.T, branch string) (*CloneResult, string) {
	t.Helper()

	clone := setupClonedRepoFromBare(t)
	worktreePath := filepath.Join(clone.RepoRoot, branch)
	testutil.RunGit(t, "", "--git-dir", clone.GitCommonDir, "worktree", "add", "-b", branch, worktreePath, clone.DefaultBranch)
	return clone, worktreePath
}

func repoContextFromClone(clone *CloneResult) *RepoContext {
	return &RepoContext{
		RepoRoot:      clone.RepoRoot,
		GitCommonDir:  clone.GitCommonDir,
		DefaultBranch: clone.DefaultBranch,
		IncludeFile:   ".worktreeinclude",
	}
}
