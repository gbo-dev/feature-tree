package gitx

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "https with .git", url: "https://github.com/org/repo.git", want: "repo"},
		{name: "https without .git", url: "https://github.com/org/repo", want: "repo"},
		{name: "ssh scp-like", url: "git@github.com:org/repo.git", want: "repo"},
		{name: "local path", url: "/tmp/example/repo.git", want: "repo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := repoNameFromURL(tc.url)
			if got != tc.want {
				t.Fatalf("repoNameFromURL(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestCloneRepoBootstrapsTrackingAndInitialWorktree(t *testing.T) {
	SetCommandContext(context.Background())

	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "cloned")
	result, err := CloneRepo(remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	if result.DefaultBranch != "main" {
		t.Fatalf("default branch = %q, want %q", result.DefaultBranch, "main")
	}
	if result.RepoRoot != target {
		t.Fatalf("RepoRoot = %q, want %q", result.RepoRoot, target)
	}
	if result.GitCommonDir != filepath.Join(target, ".git") {
		t.Fatalf("GitCommonDir = %q, want %q", result.GitCommonDir, filepath.Join(target, ".git"))
	}
	if result.WorktreePath != filepath.Join(target, "main") {
		t.Fatalf("WorktreePath = %q, want %q", result.WorktreePath, filepath.Join(target, "main"))
	}

	originHead := testutil.RunGit(t, "", "--git-dir", result.GitCommonDir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if originHead != "origin/main" {
		t.Fatalf("origin/HEAD = %q, want %q", originHead, "origin/main")
	}

	trackRemote := testutil.RunGit(t, "", "--git-dir", result.GitCommonDir, "config", "--get", "branch.main.remote")
	if trackRemote != "origin" {
		t.Fatalf("branch.main.remote = %q, want %q", trackRemote, "origin")
	}

	trackMerge := testutil.RunGit(t, "", "--git-dir", result.GitCommonDir, "config", "--get", "branch.main.merge")
	if trackMerge != "refs/heads/main" {
		t.Fatalf("branch.main.merge = %q, want %q", trackMerge, "refs/heads/main")
	}

	headBranch := testutil.RunGit(t, result.WorktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	if headBranch != "main" {
		t.Fatalf("worktree HEAD = %q, want %q", headBranch, "main")
	}

	// Verifies that tracking config is valid enough for pull to work from the worktree.
	testutil.RunGit(t, result.WorktreePath, "pull", "--ff-only")
}

func TestCloneRepoFailsWhenRemoteHeadIsUnset(t *testing.T) {
	SetCommandContext(context.Background())

	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)
	testutil.RunGit(t, "", "--git-dir", remote, "symbolic-ref", "HEAD", "refs/heads/does-not-exist")

	target := filepath.Join(base, "cloned")
	_, err := CloneRepo(remote, target)
	if err == nil {
		t.Fatalf("CloneRepo expected error when remote HEAD is unset")
	}
	if !strings.Contains(err.Error(), "resolve origin/HEAD") {
		t.Fatalf("CloneRepo error = %q, expected origin/HEAD resolution error", err.Error())
	}

	_, statErr := os.Stat(target)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("CloneRepo should clean up failed target directory, stat err = %v", statErr)
	}
}
