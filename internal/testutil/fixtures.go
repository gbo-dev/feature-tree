package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

type BareRepoFixture struct {
	RepoRoot      string
	GitCommonDir  string
	DefaultBranch string
	WorktreePath  string
}

type FeatureWorktreeFixture struct {
	*BareRepoFixture
	Branch      string
	FeaturePath string
}

func SetupBareRepo(t *testing.T) *BareRepoFixture {
	t.Helper()

	base := t.TempDir()
	source := filepath.Join(base, "source")
	InitRepoWithMain(t, source)

	remote := filepath.Join(base, "origin.git")
	RunGit(t, "", "clone", "--bare", source, remote)

	repoRoot := filepath.Join(base, "repo")
	gitDir := filepath.Join(repoRoot, ".git")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("create repo root: %v", err)
	}

	RunGit(t, "", "clone", "--bare", remote, gitDir)
	RunGit(t, "", "--git-dir", gitDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	RunGit(t, "", "--git-dir", gitDir, "fetch", "origin")
	RunGit(t, "", "--git-dir", gitDir, "remote", "set-head", "origin", "--auto")
	RunGit(t, "", "--git-dir", gitDir, "config", "branch.main.remote", "origin")
	RunGit(t, "", "--git-dir", gitDir, "config", "branch.main.merge", "refs/heads/main")

	worktreePath := filepath.Join(repoRoot, "main")
	RunGit(t, "", "--git-dir", gitDir, "worktree", "add", worktreePath, "main")

	return &BareRepoFixture{
		RepoRoot:      repoRoot,
		GitCommonDir:  gitDir,
		DefaultBranch: "main",
		WorktreePath:  worktreePath,
	}
}

func SetupFeatureWorktree(t *testing.T, branch string) *FeatureWorktreeFixture {
	t.Helper()

	bare := SetupBareRepo(t)
	featurePath := filepath.Join(bare.RepoRoot, branch)
	RunGit(t, "", "--git-dir", bare.GitCommonDir, "worktree", "add", "-b", branch, featurePath, bare.DefaultBranch)

	return &FeatureWorktreeFixture{
		BareRepoFixture: bare,
		Branch:          branch,
		FeaturePath:     featurePath,
	}
}

func CommitEquivalentChanges(t *testing.T, worktreePath string, filename string) {
	t.Helper()

	tempFile := filepath.Join(worktreePath, filename)
	if err := os.WriteFile(tempFile, []byte("temporary content\n"), 0o644); err != nil {
		t.Fatalf("write equivalent temp file: %v", err)
	}
	RunGit(t, worktreePath, "add", filename)
	RunGit(t, worktreePath, "commit", "-m", "add temporary file")
	RunGit(t, worktreePath, "rm", filename)
	RunGit(t, worktreePath, "commit", "-m", "remove temporary file")
}
