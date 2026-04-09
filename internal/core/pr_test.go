package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestParsePRNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "valid positive", input: "123", want: 123, wantErr: false},
		{name: "valid single digit", input: "1", want: 1, wantErr: false},
		{name: "zero", input: "0", want: 0, wantErr: true},
		{name: "negative", input: "-1", want: 0, wantErr: true},
		{name: "non-numeric", input: "abc", want: 0, wantErr: true},
		{name: "empty", input: "", want: 0, wantErr: true},
		{name: "with spaces", input: " 123 ", want: 0, wantErr: true},
		{name: "with leading zeros", input: "007", want: 7, wantErr: false},
		{name: "large number", input: "2147483647", want: 2147483647, wantErr: false},
		{name: "float-like", input: "1.5", want: 0, wantErr: true},
		{name: "with letters", input: "123abc", want: 0, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePRNumber(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParsePRNumber(%q) expected error, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Fatalf("ParsePRNumber(%q) unexpected error: %v", tc.input, err)
				}
				if got != tc.want {
					t.Fatalf("ParsePRNumber(%q) = %d, want %d", tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestGetPRInfoFetchesFromOrigin(t *testing.T) {
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

	featureBranch := "feature-to-pr"
	featureBranchPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", featureBranch, featureBranchPath, cloneResult.DefaultBranch)

	testutil.RunGit(t, featureBranchPath, "config", "user.name", "Test User")
	testutil.RunGit(t, featureBranchPath, "config", "user.email", "test@example.com")

	prFile := filepath.Join(featureBranchPath, "pr-file.txt")
	if err := os.WriteFile(prFile, []byte("PR content\n"), 0o644); err != nil {
		t.Fatalf("write pr file: %v", err)
	}
	testutil.RunGit(t, featureBranchPath, "add", "pr-file.txt")
	testutil.RunGit(t, featureBranchPath, "commit", "-m", "PR commit")

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/42/head", featureBranch)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	prInfo, err := svc.GetPRInfo(42)
	if err != nil {
		t.Fatalf("GetPRInfo returned error: %v", err)
	}
	if prInfo.Number != 42 {
		t.Fatalf("GetPRInfo Number = %d, want 42", prInfo.Number)
	}
	if prInfo.HeadRef != featureBranch {
		t.Fatalf("GetPRInfo HeadRef = %q, want %q", prInfo.HeadRef, featureBranch)
	}
	if prInfo.BaseBranch != cloneResult.DefaultBranch {
		t.Fatalf("GetPRInfo BaseBranch = %q, want %q", prInfo.BaseBranch, cloneResult.DefaultBranch)
	}
}

func TestFetchAndCheckoutPRCreatesWorktree(t *testing.T) {
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

	featureBranch := "feature-pr-test"
	featureBranchPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", featureBranch, featureBranchPath, cloneResult.DefaultBranch)

	testutil.RunGit(t, featureBranchPath, "config", "user.name", "Test User")
	testutil.RunGit(t, featureBranchPath, "config", "user.email", "test@example.com")

	prFile := filepath.Join(featureBranchPath, "pr-test-file.txt")
	if err := os.WriteFile(prFile, []byte("PR test content\n"), 0o644); err != nil {
		t.Fatalf("write pr test file: %v", err)
	}
	testutil.RunGit(t, featureBranchPath, "add", "pr-test-file.txt")
	testutil.RunGit(t, featureBranchPath, "commit", "-m", "PR test commit")

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/99/head", featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "remove", "--force", featureBranchPath)
	worktreeList := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "list", "--porcelain")
	if strings.Contains(worktreeList, featureBranchPath) {
		t.Fatalf("test setup failed: feature branch worktree still present at %q", featureBranchPath)
	}

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPR(99)
	if err != nil {
		t.Fatalf("FetchAndCheckoutPR returned error: %v", err)
	}
	if result.Number != 99 {
		t.Fatalf("FetchAndCheckoutPR Number = %d, want 99", result.Number)
	}
	if result.Branch != featureBranch {
		t.Fatalf("FetchAndCheckoutPR Branch = %q, want %q", result.Branch, featureBranch)
	}
	if !result.Created {
		t.Fatalf("FetchAndCheckoutPR Created = false, want true")
	}

	expectedPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	if result.Path != expectedPath {
		t.Fatalf("FetchAndCheckoutPR Path = %q, want %q", result.Path, expectedPath)
	}
}

func TestFetchAndCheckoutPRReusesExistingWorktree(t *testing.T) {
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

	pullBranch := "pull/77"
	pullBranchPath := filepath.Join(cloneResult.RepoRoot, "pull-77")
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", pullBranch, pullBranchPath, cloneResult.DefaultBranch)

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/77/head", pullBranch)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPR(77)
	if err != nil {
		t.Fatalf("FetchAndCheckoutPR returned error: %v", err)
	}
	if result.Number != 77 {
		t.Fatalf("FetchAndCheckoutPR Number = %d, want 77", result.Number)
	}
	if result.Branch != "pull/77" {
		t.Fatalf("FetchAndCheckoutPR Branch = %q, want %q", result.Branch, "pull/77")
	}
	if result.Created {
		t.Fatalf("FetchAndCheckoutPR Created = true, want false for existing worktree")
	}

	canonicalPath := testutil.CanonicalPath(t, result.Path)
	canonicalWant := testutil.CanonicalPath(t, pullBranchPath)
	if canonicalPath != canonicalWant {
		t.Fatalf("FetchAndCheckoutPR Path = %q, want %q", canonicalPath, canonicalWant)
	}
}

func TestGetPRInfoHandlesNonexistentPR(t *testing.T) {
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

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	_, err = svc.GetPRInfo(999999)
	if err == nil {
		t.Fatalf("GetPRInfo expected error for nonexistent PR, got nil")
	}
}

func TestEnsureLocalRefUpdatedRefreshesStaleRef(t *testing.T) {
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

	featureBranch := "feature-stale"
	featureBranchPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", featureBranch, featureBranchPath, cloneResult.DefaultBranch)

	testutil.RunGit(t, featureBranchPath, "config", "user.name", "Test User")
	testutil.RunGit(t, featureBranchPath, "config", "user.email", "test@example.com")

	prFile := filepath.Join(featureBranchPath, "stale-file.txt")
	if err := os.WriteFile(prFile, []byte("stale content\n"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}
	testutil.RunGit(t, featureBranchPath, "add", "stale-file.txt")
	testutil.RunGit(t, featureBranchPath, "commit", "-m", "stale commit")

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/55/head", featureBranch)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	prInfo, err := svc.GetPRInfo(55)
	if err != nil {
		t.Fatalf("GetPRInfo returned error: %v", err)
	}

	err = svc.ensureLocalRefUpdated(prInfo)
	if err != nil {
		t.Fatalf("ensureLocalRefUpdated returned error: %v", err)
	}

	stdout, _, _, _ := gitx.RunGitCommon(context.Background(), svc.Ctx, "rev-parse", "--verify", fmt.Sprintf("refs/pull/%d/head", 55))
	updatedSHA := strings.TrimSpace(stdout)
	if updatedSHA != prInfo.HeadSHA {
		t.Fatalf("ensureLocalRefUpdated: expected SHA %q, got %q", prInfo.HeadSHA, updatedSHA)
	}
}

func TestFetchAndCheckoutPRNoOriginFails(t *testing.T) {
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

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "remote", "remove", "origin")

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	_, err = svc.FetchAndCheckoutPR(42)
	if err == nil {
		t.Fatalf("FetchAndCheckoutPR expected error without origin, got nil")
	}
}
