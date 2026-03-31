package core

import (
	"context"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestSanitizeBranchName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "feature/login", want: "feature-login"},
		{in: `bugfix\windows\path`, want: "bugfix-windows-path"},
		{in: "plain-branch", want: "plain-branch"},
	}

	for _, tc := range tests {
		got := SanitizeBranchName(tc.in)
		if got != tc.want {
			t.Fatalf("SanitizeBranchName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFindWorktreePath(t *testing.T) {
	worktrees := []gitx.Worktree{
		{Branch: "main", Path: "/repo/main"},
		{Branch: "feature/a", Path: "/repo/feature-a"},
	}

	if got := FindWorktreePath(worktrees, "feature/a"); got != "/repo/feature-a" {
		t.Fatalf("FindWorktreePath found %q, want %q", got, "/repo/feature-a")
	}
	if got := FindWorktreePath(worktrees, "missing"); got != "" {
		t.Fatalf("FindWorktreePath for missing branch = %q, want empty string", got)
	}
}

func TestResolveBranchShortcutDefaultBranch(t *testing.T) {
	svc := &Service{Ctx: &gitx.RepoContext{DefaultBranch: "main"}}

	got, err := svc.ResolveBranchShortcut("^")
	if err != nil {
		t.Fatalf("ResolveBranchShortcut(^) returned error: %v", err)
	}
	if got != "main" {
		t.Fatalf("ResolveBranchShortcut(^) = %q, want %q", got, "main")
	}
}

func TestResolveBranchShortcutCurrentBranch(t *testing.T) {
	gitx.SetCommandContext(context.Background())

	repo := t.TempDir()
	testutil.InitRepoWithMain(t, repo)
	testutil.Chdir(t, repo)

	svc := &Service{Ctx: &gitx.RepoContext{DefaultBranch: "main"}}
	got, err := svc.ResolveBranchShortcut("@")
	if err != nil {
		t.Fatalf("ResolveBranchShortcut(@) returned error: %v", err)
	}
	if got != "main" {
		t.Fatalf("ResolveBranchShortcut(@) = %q, want %q", got, "main")
	}
}

func TestResolveBranchShortcutDetachedHead(t *testing.T) {
	gitx.SetCommandContext(context.Background())

	repo := t.TempDir()
	testutil.InitRepoWithMain(t, repo)
	testutil.Chdir(t, repo)

	head := testutil.RunGit(t, repo, "rev-parse", "HEAD")
	testutil.RunGit(t, repo, "checkout", "--detach", head)

	svc := &Service{Ctx: &gitx.RepoContext{DefaultBranch: "main"}}
	_, err := svc.ResolveBranchShortcut("@")
	if err == nil {
		t.Fatalf("ResolveBranchShortcut(@) expected error on detached HEAD")
	}
	if !strings.Contains(err.Error(), "HEAD is detached") {
		t.Fatalf("detached HEAD error = %q, expected mention of detached HEAD", err.Error())
	}
}
