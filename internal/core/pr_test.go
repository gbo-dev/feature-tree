package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

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

	prInfo, err := svc.getPRInfo(42, false)
	if err != nil {
		t.Fatalf("getPRInfo returned error: %v", err)
	}
	if prInfo.Number != 42 {
		t.Fatalf("getPRInfo Number = %d, want 42", prInfo.Number)
	}
	if prInfo.HeadRef != featureBranch {
		t.Fatalf("getPRInfo HeadRef = %q, want %q", prInfo.HeadRef, featureBranch)
	}
	if prInfo.BaseBranch != cloneResult.DefaultBranch {
		t.Fatalf("getPRInfo BaseBranch = %q, want %q", prInfo.BaseBranch, cloneResult.DefaultBranch)
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

	result, err := svc.FetchAndCheckoutPRWithOptions(99, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Number != 99 {
		t.Fatalf("FetchAndCheckoutPRWithOptions Number = %d, want 99", result.Number)
	}
	if result.Branch != featureBranch {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, featureBranch)
	}
	if !result.Created {
		t.Fatalf("FetchAndCheckoutPRWithOptions Created = false, want true")
	}

	expectedPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	if result.Path != expectedPath {
		t.Fatalf("FetchAndCheckoutPRWithOptions Path = %q, want %q", result.Path, expectedPath)
	}
}

func TestFetchAndCheckoutPRWithOptionsUsesPRRef(t *testing.T) {
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

	featureBranch := "feature-pr-option"
	featureBranchPath := filepath.Join(cloneResult.RepoRoot, featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "add", "-b", featureBranch, featureBranchPath, cloneResult.DefaultBranch)
	testutil.RunGit(t, featureBranchPath, "config", "user.name", "Test User")
	testutil.RunGit(t, featureBranchPath, "config", "user.email", "test@example.com")

	prFile := filepath.Join(featureBranchPath, "pr-option-file.txt")
	if err := os.WriteFile(prFile, []byte("PR option content\n"), 0o644); err != nil {
		t.Fatalf("write pr option file: %v", err)
	}
	testutil.RunGit(t, featureBranchPath, "add", "pr-option-file.txt")
	testutil.RunGit(t, featureBranchPath, "commit", "-m", "PR option commit")

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/101/head", featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "remove", "--force", featureBranchPath)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(101, PRCheckoutOptions{UsePRRef: true})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != "pull/101" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/101")
	}
	expectedPath := filepath.Join(cloneResult.RepoRoot, "pull-101")
	if result.Path != expectedPath {
		t.Fatalf("FetchAndCheckoutPRWithOptions Path = %q, want %q", result.Path, expectedPath)
	}

	prHead := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/pull/101/head")
	branchHead := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/heads/pull/101")
	if branchHead != prHead {
		t.Fatalf("pull/101 HEAD = %q, want PR head %q", branchHead, prHead)
	}
}

func TestFetchAndCheckoutPRSetsTrackingToRemoteHeadBranch(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-pr-upstream"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)

	prFile := filepath.Join(source, "tracked-pr-file.txt")
	if err := os.WriteFile(prFile, []byte("tracked PR content\n"), 0o644); err != nil {
		t.Fatalf("write tracked PR file: %v", err)
	}
	testutil.RunGit(t, source, "add", "tracked-pr-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "tracked PR commit")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/202/head", "refs/remotes/origin/"+featureBranch)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(202, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != featureBranch {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, featureBranch)
	}

	trackRemote := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch."+featureBranch+".remote")
	if trackRemote != "origin" {
		t.Fatalf("branch.%s.remote = %q, want %q", featureBranch, trackRemote, "origin")
	}

	trackMerge := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch."+featureBranch+".merge")
	if trackMerge != "refs/heads/"+featureBranch {
		t.Fatalf("branch.%s.merge = %q, want %q", featureBranch, trackMerge, "refs/heads/"+featureBranch)
	}
}

func TestFetchAndCheckoutPRWithOptionsUsePRRefSetsTrackingToRemoteHeadBranch(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-pr-upstream-ref"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)

	prFile := filepath.Join(source, "tracked-pr-ref-file.txt")
	if err := os.WriteFile(prFile, []byte("tracked PR ref content\n"), 0o644); err != nil {
		t.Fatalf("write tracked PR ref file: %v", err)
	}
	testutil.RunGit(t, source, "add", "tracked-pr-ref-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "tracked PR ref commit")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/303/head", "refs/remotes/origin/"+featureBranch)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(303, PRCheckoutOptions{UsePRRef: true})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != "pull/303" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/303")
	}

	trackRemote := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch.pull/303.remote")
	if trackRemote != "origin" {
		t.Fatalf("branch.pull/303.remote = %q, want %q", trackRemote, "origin")
	}

	trackMerge := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch.pull/303.merge")
	if trackMerge != "refs/heads/"+featureBranch {
		t.Fatalf("branch.pull/303.merge = %q, want %q", trackMerge, "refs/heads/"+featureBranch)
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

	result, err := svc.FetchAndCheckoutPRWithOptions(77, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Number != 77 {
		t.Fatalf("FetchAndCheckoutPRWithOptions Number = %d, want 77", result.Number)
	}
	if result.Branch != "pull/77" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/77")
	}
	if result.Created {
		t.Fatalf("FetchAndCheckoutPRWithOptions Created = true, want false for existing worktree")
	}

	canonicalPath := testutil.CanonicalPath(t, result.Path)
	canonicalWant := testutil.CanonicalPath(t, pullBranchPath)
	if canonicalPath != canonicalWant {
		t.Fatalf("FetchAndCheckoutPRWithOptions Path = %q, want %q", canonicalPath, canonicalWant)
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

	_, err = svc.getPRInfo(999999, false)
	if err == nil {
		t.Fatalf("getPRInfo expected error for nonexistent PR, got nil")
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

	prInfo, err := svc.getPRInfo(55, false)
	if err != nil {
		t.Fatalf("getPRInfo returned error: %v", err)
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

func TestFetchAndCheckoutPRRefreshesStaleLocalPRRefBeforeResolvingBranchName(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-pr-refresh"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)

	prFile := filepath.Join(source, "refresh-pr-file.txt")
	if err := os.WriteFile(prFile, []byte("refresh PR content\n"), 0o644); err != nil {
		t.Fatalf("write refresh PR file: %v", err)
	}
	testutil.RunGit(t, source, "add", "refresh-pr-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "refresh PR commit")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	prNumber := 808
	featureSHA := testutil.RunGit(t, source, "rev-parse", "--verify", featureBranch)
	staleSHA := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/heads/"+cloneResult.DefaultBranch)

	testutil.RunGit(t, "", "--git-dir", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), featureSHA)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), staleSHA)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		CommandCtx: context.Background(),
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(prNumber, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != featureBranch {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, featureBranch)
	}

	refSHA := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", fmt.Sprintf("refs/pull/%d/head", prNumber))
	if refSHA != featureSHA {
		t.Fatalf("refs/pull/%d/head = %q, want %q", prNumber, refSHA, featureSHA)
	}
}

func TestFetchAndCheckoutPRWithCachedRefAndNoOriginWarnsAndUsesCache(t *testing.T) {
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

	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/42/head", "refs/heads/"+cloneResult.DefaultBranch)
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

	originalStderr := os.Stderr
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("os.Pipe failed: %v", pipeErr)
	}
	os.Stderr = w
	defer func() {
		os.Stderr = originalStderr
	}()

	result, err := svc.FetchAndCheckoutPRWithOptions(42, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != "pull/42" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/42")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close write pipe failed: %v", err)
	}
	warningBytes, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("read warning output failed: %v", readErr)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close read pipe failed: %v", err)
	}

	warningOutput := string(warningBytes)
	if !strings.Contains(warningOutput, "failed to update PR #42 from origin; using cached ref refs/pull/42/head") {
		t.Fatalf("warning output = %q, want cached-ref warning", warningOutput)
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

	_, err = svc.FetchAndCheckoutPRWithOptions(42, PRCheckoutOptions{})
	if err == nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions expected error without origin, got nil")
	}
}
