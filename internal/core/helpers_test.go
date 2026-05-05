package core

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func setupServiceWithFeatureWorktree(t *testing.T) (*Service, string, string) {
	t.Helper()

	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	branch := "feature-remove"
	featurePath := filepath.Join(cloneResult.RepoRoot, branch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", branch, featurePath, cloneResult.DefaultBranch)

	svc := &Service{Ctx: &gitx.RepoContext{
		RepoRoot:      cloneResult.RepoRoot,
		GitCommonDir:  cloneResult.GitCommonDir,
		DefaultBranch: cloneResult.DefaultBranch,
		IncludeFile:   ".worktreeinclude",
	}}

	return svc, featurePath, branch
}
