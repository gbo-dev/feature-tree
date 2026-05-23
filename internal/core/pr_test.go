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
	}

	prInfo, err := svc.getPRInfo(context.Background(), 42, false)
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 99, PRCheckoutOptions{})
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 101, PRCheckoutOptions{UsePRRef: true})
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 202, PRCheckoutOptions{})
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

func TestFetchAndCheckoutPRFailsOnBranchNameCollision(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-pr-collision"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)

	collisionFile := filepath.Join(source, "collision-file.txt")
	if err := os.WriteFile(collisionFile, []byte("collision content\n"), 0o644); err != nil {
		t.Fatalf("write collision file: %v", err)
	}
	testutil.RunGit(t, source, "add", "collision-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "collision commit")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	prHeadSHA := testutil.RunGit(t, source, "rev-parse", "--verify", featureBranch)
	mainSHA := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/heads/"+cloneResult.DefaultBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/heads/"+featureBranch, mainSHA)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/404/head", prHeadSHA)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{HeadRefName: featureBranch}, nil
		},
	}

	_, err = svc.FetchAndCheckoutPRWithOptions(context.Background(), 404, PRCheckoutOptions{})
	if err == nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), featureBranch) {
		t.Fatalf("error = %q, want mention of branch %q", err.Error(), featureBranch)
	}

	branchSHA := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/heads/"+featureBranch)
	if branchSHA != mainSHA {
		t.Fatalf("refs/heads/%s = %q, want unchanged %q", featureBranch, branchSHA, mainSHA)
	}
}

func TestFetchAndCheckoutPRAdvancesExistingBranchFromCachedPRHead(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-pr-advance"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)

	prFile := filepath.Join(source, "advance-file.txt")
	if err := os.WriteFile(prFile, []byte("v1\n"), 0o644); err != nil {
		t.Fatalf("write advance file: %v", err)
	}
	testutil.RunGit(t, source, "add", "advance-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "advance v1")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	prNumber := 606
	v1SHA := testutil.RunGit(t, source, "rev-parse", "--verify", featureBranch)
	testutil.RunGit(t, "", "--git-dir", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), v1SHA)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{HeadRefName: featureBranch}, nil
		},
	}

	first, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), prNumber, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("first FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "worktree", "remove", "--force", first.Path)

	testutil.RunGit(t, source, "checkout", featureBranch)
	if err := os.WriteFile(prFile, []byte("v2\n"), 0o644); err != nil {
		t.Fatalf("write updated advance file: %v", err)
	}
	testutil.RunGit(t, source, "add", "advance-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "advance v2")
	v2SHA := testutil.RunGit(t, source, "rev-parse", "--verify", featureBranch)
	testutil.RunGit(t, source, "push", remote, featureBranch)
	testutil.RunGit(t, "", "--git-dir", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), v2SHA)

	second, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), prNumber, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("second FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if !second.Created {
		t.Fatalf("second FetchAndCheckoutPRWithOptions Created = false, want true")
	}

	branchSHA := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "rev-parse", "--verify", "refs/heads/"+featureBranch)
	if branchSHA != v2SHA {
		t.Fatalf("refs/heads/%s = %q, want advanced PR head %q", featureBranch, branchSHA, v2SHA)
	}
}

func TestFetchAndCheckoutPRHintsWhenMetadataUnavailable(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	featureBranch := "feature-metadata-unavailable"
	testutil.RunGit(t, source, "checkout", "-b", featureBranch)
	metadataFile := filepath.Join(source, "metadata-file.txt")
	if err := os.WriteFile(metadataFile, []byte("metadata\n"), 0o644); err != nil {
		t.Fatalf("write metadata file: %v", err)
	}
	testutil.RunGit(t, source, "add", "metadata-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "metadata commit")
	testutil.RunGit(t, source, "checkout", "main")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	prNumber := 607
	featureSHA := testutil.RunGit(t, source, "rev-parse", "--verify", featureBranch)
	testutil.RunGit(t, "", "--git-dir", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), featureSHA)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{}, fmt.Errorf("gh unavailable")
		},
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), prNumber, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if !strings.Contains(strings.Join(result.Hints, " "), "PR metadata unavailable") {
		t.Fatalf("hints = %v, want metadata unavailable hint", result.Hints)
	}
}

func TestFetchAndCheckoutPRSkipsUpstreamHintForSyntheticPullBranch(t *testing.T) {
	base := t.TempDir()
	source := filepath.Join(base, "source")
	testutil.InitRepoWithMain(t, source)

	prFile := filepath.Join(source, "synthetic-file.txt")
	if err := os.WriteFile(prFile, []byte("synthetic\n"), 0o644); err != nil {
		t.Fatalf("write synthetic file: %v", err)
	}
	testutil.RunGit(t, source, "add", "synthetic-file.txt")
	testutil.RunGit(t, source, "commit", "-m", "synthetic commit")

	remote := filepath.Join(base, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", source, remote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	prNumber := 608
	prSHA := testutil.RunGit(t, source, "rev-parse", "--verify", "HEAD")
	testutil.RunGit(t, "", "--git-dir", remote, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), prSHA)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "remote", "remove", "origin")
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", fmt.Sprintf("refs/pull/%d/head", prNumber), prSHA)

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{}, nil
		},
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), prNumber, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	hints := strings.Join(result.Hints, " ")
	if !strings.Contains(hints, "PR head branch is unknown") {
		t.Fatalf("hints = %v, want unknown head branch hint", result.Hints)
	}
	if strings.Contains(hints, "git push -u origin pull/") {
		t.Fatalf("hints = %v, should not suggest pushing synthetic pull branch to origin", result.Hints)
	}
}

func TestFetchAndCheckoutPRWithOptionsUsePRRefDoesNotSetMismatchedUpstream(t *testing.T) {
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 303, PRCheckoutOptions{UsePRRef: true})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != "pull/303" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/303")
	}

	_, _, configErr := testutil.RunGitWithError(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch.pull/303.remote")
	if configErr == nil {
		t.Fatalf("branch.pull/303 should not have upstream remote configured")
	}

	if len(result.Hints) == 0 {
		t.Fatalf("FetchAndCheckoutPRWithOptions hints = %v, want push hint", result.Hints)
	}
	hint := strings.Join(result.Hints, " ")
	if !strings.Contains(hint, featureBranch) {
		t.Fatalf("hint = %q, want mention of head branch %q", hint, featureBranch)
	}
}

func TestFetchAndCheckoutPRForkSetsForkRemoteUpstream(t *testing.T) {
	base := t.TempDir()
	upstreamSource := filepath.Join(base, "upstream-source")
	testutil.InitRepoWithMain(t, upstreamSource)

	featureBranch := "feature-fork-pr"
	testutil.RunGit(t, upstreamSource, "checkout", "-b", featureBranch)
	forkFile := filepath.Join(upstreamSource, "fork-file.txt")
	if err := os.WriteFile(forkFile, []byte("fork content\n"), 0o644); err != nil {
		t.Fatalf("write fork file: %v", err)
	}
	testutil.RunGit(t, upstreamSource, "add", "fork-file.txt")
	testutil.RunGit(t, upstreamSource, "commit", "-m", "fork commit")
	testutil.RunGit(t, upstreamSource, "checkout", "main")

	upstreamRemote := filepath.Join(base, "upstream.git")
	testutil.RunGit(t, "", "clone", "--bare", upstreamSource, upstreamRemote)

	forkSource := filepath.Join(base, "fork-source")
	testutil.RunGit(t, "", "clone", upstreamRemote, forkSource)
	testutil.RunGit(t, forkSource, "checkout", featureBranch)
	testutil.RunGit(t, forkSource, "push", "origin", featureBranch)

	forkRemote := filepath.Join(base, "fork.git")
	testutil.RunGit(t, "", "clone", "--bare", forkSource, forkRemote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), upstreamRemote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	featureSHA := testutil.RunGit(t, forkSource, "rev-parse", "--verify", featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/505/head", featureSHA)

	forkURL := "file://" + filepath.ToSlash(forkRemote)
	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{
				HeadRefName:         featureBranch,
				HeadRepositoryURL:   forkURL,
				HeadRepositoryOwner: "forkuser",
				IsCrossRepository:   true,
			}, nil
		},
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 505, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != featureBranch {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, featureBranch)
	}

	forkRemoteName := forkRemoteName("forkuser")
	remoteURL := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "remote", "get-url", forkRemoteName)
	if remoteURL != forkURL {
		t.Fatalf("remote %s URL = %q, want %q", forkRemoteName, remoteURL, forkURL)
	}

	trackRemote := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch."+featureBranch+".remote")
	if trackRemote != forkRemoteName {
		t.Fatalf("branch.%s.remote = %q, want %q", featureBranch, trackRemote, forkRemoteName)
	}

	trackMerge := testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch."+featureBranch+".merge")
	if trackMerge != "refs/heads/"+featureBranch {
		t.Fatalf("branch.%s.merge = %q, want %q", featureBranch, trackMerge, "refs/heads/"+featureBranch)
	}
}

func TestFetchAndCheckoutPRForkRemoteNameCollisionReturnsHint(t *testing.T) {
	base := t.TempDir()
	upstreamSource := filepath.Join(base, "upstream-source")
	testutil.InitRepoWithMain(t, upstreamSource)

	featureBranch := "feature-fork-collision"
	testutil.RunGit(t, upstreamSource, "checkout", "-b", featureBranch)
	forkFile := filepath.Join(upstreamSource, "fork-collision-file.txt")
	if err := os.WriteFile(forkFile, []byte("fork collision\n"), 0o644); err != nil {
		t.Fatalf("write fork collision file: %v", err)
	}
	testutil.RunGit(t, upstreamSource, "add", "fork-collision-file.txt")
	testutil.RunGit(t, upstreamSource, "commit", "-m", "fork collision commit")
	testutil.RunGit(t, upstreamSource, "checkout", "main")

	upstreamRemote := filepath.Join(base, "upstream.git")
	testutil.RunGit(t, "", "clone", "--bare", upstreamSource, upstreamRemote)

	forkSource := filepath.Join(base, "fork-source")
	testutil.RunGit(t, "", "clone", upstreamRemote, forkSource)
	testutil.RunGit(t, forkSource, "checkout", featureBranch)
	testutil.RunGit(t, forkSource, "push", "origin", featureBranch)

	forkRemote := filepath.Join(base, "fork.git")
	testutil.RunGit(t, "", "clone", "--bare", forkSource, forkRemote)

	target := filepath.Join(base, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), upstreamRemote, target)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	featureSHA := testutil.RunGit(t, forkSource, "rev-parse", "--verify", featureBranch)
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "update-ref", "refs/pull/506/head", featureSHA)

	forkURL := "file://" + filepath.ToSlash(forkRemote)
	conflictingRemoteName := forkRemoteName("forkuser")
	testutil.RunGit(t, "", "--git-dir", cloneResult.GitCommonDir, "remote", "add", conflictingRemoteName, "file://"+filepath.ToSlash(upstreamRemote))

	svc := &Service{
		Ctx: &gitx.RepoContext{
			RepoRoot:      cloneResult.RepoRoot,
			GitCommonDir:  cloneResult.GitCommonDir,
			DefaultBranch: cloneResult.DefaultBranch,
			IncludeFile:   ".worktreeinclude",
		},
		prMetadataResolver: func(context.Context, int) (PRMetadata, error) {
			return PRMetadata{
				HeadRefName:         featureBranch,
				HeadRepositoryURL:   forkURL,
				HeadRepositoryOwner: "forkuser",
				IsCrossRepository:   true,
			}, nil
		},
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 506, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if !strings.Contains(strings.Join(result.Hints, " "), "could not add remote") {
		t.Fatalf("hints = %v, want remote collision hint", result.Hints)
	}

	_, _, configErr := testutil.RunGitWithError(t, "", "--git-dir", cloneResult.GitCommonDir, "config", "--get", "branch."+featureBranch+".remote")
	if configErr == nil {
		t.Fatalf("branch.%s should not have upstream remote configured", featureBranch)
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 77, PRCheckoutOptions{})
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
	}

	_, err = svc.getPRInfo(context.Background(), 999999, false)
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
	}

	prInfo, err := svc.getPRInfo(context.Background(), 55, false)
	if err != nil {
		t.Fatalf("getPRInfo returned error: %v", err)
	}

	updatedSHA, _, err := svc.ensureLocalRefUpdated(context.Background(), prInfo.Number, prInfo.HeadSHA)
	if err != nil {
		t.Fatalf("ensureLocalRefUpdated returned error: %v", err)
	}

	stdout, _, _, _ := gitx.RunGitCommon(context.Background(), svc.Ctx, "rev-parse", "--verify", fmt.Sprintf("refs/pull/%d/head", 55))
	expectedSHA := strings.TrimSpace(stdout)
	if expectedSHA != updatedSHA {
		t.Fatalf("ensureLocalRefUpdated: expected SHA %q, got %q", expectedSHA, updatedSHA)
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), prNumber, PRCheckoutOptions{})
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
	}

	result, err := svc.FetchAndCheckoutPRWithOptions(context.Background(), 42, PRCheckoutOptions{})
	if err != nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions returned error: %v", err)
	}
	if result.Branch != "pull/42" {
		t.Fatalf("FetchAndCheckoutPRWithOptions Branch = %q, want %q", result.Branch, "pull/42")
	}

	if len(result.Warnings) != 1 {
		t.Fatalf("FetchAndCheckoutPRWithOptions warnings = %v, want exactly one warning", result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "failed to update PR #42 from origin; using cached ref refs/pull/42/head") {
		t.Fatalf("warning = %q, want cached-ref warning", result.Warnings[0])
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
	}

	_, err = svc.FetchAndCheckoutPRWithOptions(context.Background(), 42, PRCheckoutOptions{})
	if err == nil {
		t.Fatalf("FetchAndCheckoutPRWithOptions expected error without origin, got nil")
	}
}
