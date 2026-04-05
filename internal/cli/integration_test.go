package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/shell"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestCompletionCommandBash(t *testing.T) {
	stdout, stderr, err := runRootCommand(t, "", "completion", "bash")
	if err != nil {
		t.Fatalf("ft completion bash returned error: %v", err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft completion bash stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "__start_ft") {
		t.Fatalf("ft completion bash output missing bash entrypoint, got: %q", stdout)
	}
}

func TestInitCommandNoArgsEmitsNoStderr(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")

	stdout, stderr, err := runRootCommand(t, "", "init")
	if err != nil {
		t.Fatalf("ft init returned error: %v", err)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft init stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "# ft shell integration for bash") {
		t.Fatalf("ft init stdout missing integration header, got: %q", stdout)
	}
}

func TestCommandFlowCreateSwitchRemove(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "create", "feature-cli")
	if err != nil {
		t.Fatalf("ft create returned error: %v", err)
	}
	if !strings.Contains(stdout, "Created worktree: feature-cli ->") {
		t.Fatalf("ft create output missing created message, got: %q", stdout)
	}
	if !strings.Contains(stdout, shell.CDMarkerPrefix) {
		t.Fatalf("ft create output missing cd marker, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft create stderr = %q, want empty", stderr)
	}

	stdout, stderr, err = runRootCommand(t, mainWorktreePath, "switch", "feature-cli")
	if err != nil {
		t.Fatalf("ft switch returned error: %v", err)
	}
	if !strings.Contains(stdout, "Switched to feature-cli") {
		t.Fatalf("ft switch output missing switched message, got: %q", stdout)
	}
	if !strings.Contains(stdout, shell.CDMarkerPrefix) {
		t.Fatalf("ft switch output missing cd marker, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft switch stderr = %q, want empty", stderr)
	}

	stdout, stderr, err = runRootCommand(t, mainWorktreePath, "remove", "feature-cli", "--force-worktree", "--no-delete-branch")
	if err != nil {
		t.Fatalf("ft remove returned error: %v", err)
	}
	if !strings.Contains(stdout, "Removed worktree:") {
		t.Fatalf("ft remove output missing removal message, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft remove stderr = %q, want empty", stderr)
	}

	_, _, gitErr := testutil.RunGitWithError(t, "", "--git-dir", filepath.Join(repoRoot, ".git"), "show-ref", "--verify", "--quiet", "refs/heads/feature-cli")
	if gitErr != nil {
		t.Fatalf("feature branch should remain when --no-delete-branch is used: %v", gitErr)
	}
}

func TestRemoveReportsMergedBranchDeletion(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	if _, _, err := runRootCommand(t, mainWorktreePath, "create", "feature-merged"); err != nil {
		t.Fatalf("ft create feature-merged returned error: %v", err)
	}

	featurePath := filepath.Join(repoRoot, "feature-merged")
	mergedFile := filepath.Join(featurePath, "merged.txt")
	if err := os.WriteFile(mergedFile, []byte("merged\n"), 0o644); err != nil {
		t.Fatalf("write merged file: %v", err)
	}
	testutil.RunGit(t, featurePath, "add", "merged.txt")
	testutil.RunGit(t, featurePath, "commit", "-m", "feature merged commit")
	testutil.RunGit(t, mainWorktreePath, "merge", "--no-ff", "feature-merged", "-m", "merge feature-merged")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "remove", "feature-merged")
	if err != nil {
		t.Fatalf("ft remove feature-merged returned error: %v", err)
	}
	if !strings.Contains(stdout, "Removed worktree and deleted merged branch: feature-merged") {
		t.Fatalf("ft remove output missing merged-branch deletion message, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft remove stderr = %q, want empty", stderr)
	}
}

func TestRemoveReportsCleanDeletionWithoutMergedClaim(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	if _, _, err := runRootCommand(t, mainWorktreePath, "create", "feature-clean"); err != nil {
		t.Fatalf("ft create feature-clean returned error: %v", err)
	}

	basePath := filepath.Dir(repoRoot)
	remotePath := filepath.Join(basePath, "origin.git")
	updaterPath := filepath.Join(basePath, "updater")
	testutil.RunGit(t, "", "clone", remotePath, updaterPath)
	testutil.RunGit(t, updaterPath, "config", "user.name", "Test User")
	testutil.RunGit(t, updaterPath, "config", "user.email", "test@example.com")

	remoteOnlyFile := filepath.Join(updaterPath, "remote-only.txt")
	if err := os.WriteFile(remoteOnlyFile, []byte("remote change\n"), 0o644); err != nil {
		t.Fatalf("write remote-only file: %v", err)
	}
	testutil.RunGit(t, updaterPath, "add", "remote-only.txt")
	testutil.RunGit(t, updaterPath, "commit", "-m", "advance origin main")
	testutil.RunGit(t, updaterPath, "push", "origin", "main")

	testutil.RunGit(t, mainWorktreePath, "fetch", "origin")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "remove", "feature-clean")
	if err != nil {
		t.Fatalf("ft remove feature-clean returned error: %v", err)
	}
	if !strings.Contains(stdout, "Removed worktree and deleted branch: feature-clean (fully contained in origin/main)") {
		t.Fatalf("ft remove output missing contained-in-target message, got: %q", stdout)
	}
	if strings.Contains(stdout, "deleted merged branch") {
		t.Fatalf("ft remove output should not claim merged deletion, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft remove stderr = %q, want empty", stderr)
	}
}

func TestRemoveReportsEquivalentDeletionMessage(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	if _, _, err := runRootCommand(t, mainWorktreePath, "create", "feature-equivalent"); err != nil {
		t.Fatalf("ft create feature-equivalent returned error: %v", err)
	}

	featurePath := filepath.Join(repoRoot, "feature-equivalent")
	tempFile := filepath.Join(featurePath, "EQUIVALENT.txt")
	if err := os.WriteFile(tempFile, []byte("temporary content\n"), 0o644); err != nil {
		t.Fatalf("write equivalent temp file: %v", err)
	}
	testutil.RunGit(t, featurePath, "add", "EQUIVALENT.txt")
	testutil.RunGit(t, featurePath, "commit", "-m", "add temporary file")
	testutil.RunGit(t, featurePath, "rm", "EQUIVALENT.txt")
	testutil.RunGit(t, featurePath, "commit", "-m", "remove temporary file")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "remove", "feature-equivalent")
	if err != nil {
		t.Fatalf("ft remove feature-equivalent returned error: %v", err)
	}
	if !strings.Contains(stdout, "Removed worktree and deleted branch: feature-equivalent (no effective changes vs main)") {
		t.Fatalf("ft remove output missing equivalent-deletion message, got: %q", stdout)
	}
	if strings.Contains(stdout, "force-deleted") {
		t.Fatalf("ft remove output should not claim force deletion, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft remove stderr = %q, want empty", stderr)
	}
}

func TestRemoveFromInsideTargetWorktreeEmitsFallbackCDMarker(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	if _, _, err := runRootCommand(t, mainWorktreePath, "create", "feature-self-remove"); err != nil {
		t.Fatalf("ft create feature-self-remove returned error: %v", err)
	}

	featurePath := filepath.Join(repoRoot, "feature-self-remove")
	stdout, stderr, err := runRootCommand(t, featurePath, "remove")
	if err != nil {
		t.Fatalf("ft remove from target worktree returned error: %v", err)
	}
	canonicalMainPath := testutil.CanonicalPath(t, mainWorktreePath)
	canonicalMarker := shell.CDMarkerPrefix + canonicalMainPath
	if !strings.Contains(stdout, canonicalMarker) {
		t.Fatalf("ft remove output missing fallback cd marker, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft remove stderr = %q, want empty", stderr)
	}

	if _, statErr := os.Stat(featurePath); !os.IsNotExist(statErr) {
		t.Fatalf("feature worktree path should be removed, stat error: %v", statErr)
	}
}

func TestSwitchWithoutBranchInNonInteractiveSessionFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "switch")
	if err == nil {
		t.Fatalf("ft switch without branch expected non-interactive error")
	}
	if !strings.Contains(err.Error(), "no branch specified and no interactive TTY available") {
		t.Fatalf("ft switch error = %q, expected non-interactive message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestCreateWithoutBranchInNonInteractiveSessionFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "create")
	if err == nil {
		t.Fatalf("ft create without branch expected branch-required error")
	}
	if !strings.Contains(err.Error(), "branch name is required") {
		t.Fatalf("ft create error = %q, expected branch-required message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestCreateAllBranchesWithoutBranchInNonInteractiveSessionFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "create", "--all-branches")
	if err == nil {
		t.Fatalf("ft create --all-branches without branch expected non-interactive error")
	}
	if !strings.Contains(err.Error(), "no branch specified and no interactive TTY available") {
		t.Fatalf("ft create --all-branches error = %q, expected non-interactive message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestListOutsideGitRepoFails(t *testing.T) {
	nonRepoPath := t.TempDir()

	stdout, stderr, err := runRootCommand(t, nonRepoPath, "list")
	if err == nil {
		t.Fatalf("ft list outside a git repository should fail")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("ft list outside repo error = %q, expected git repository error", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestListAtDetachedHeadFailsWithClearMessage(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)
	testutil.RunGit(t, mainWorktreePath, "checkout", "--detach")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "list")
	if err == nil {
		t.Fatalf("ft list at detached HEAD should fail")
	}
	if !strings.Contains(err.Error(), "cannot determine current branch while HEAD is detached") {
		t.Fatalf("ft list detached HEAD error = %q, expected detached HEAD message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestSwitchShortcutAtDetachedHeadFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)
	testutil.RunGit(t, mainWorktreePath, "checkout", "--detach")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "switch", "@")
	if err == nil {
		t.Fatalf("ft switch @ should fail on detached HEAD")
	}
	if !strings.Contains(err.Error(), "HEAD is detached; @ is unavailable") {
		t.Fatalf("ft switch @ detached HEAD error = %q, expected detached HEAD shortcut error", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestRemoveWithoutBranchAtDetachedHeadFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)
	testutil.RunGit(t, mainWorktreePath, "checkout", "--detach")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "remove")
	if err == nil {
		t.Fatalf("ft remove without explicit branch should fail on detached HEAD")
	}
	if !strings.Contains(err.Error(), "cannot infer branch from detached HEAD") {
		t.Fatalf("ft remove detached HEAD error = %q, expected detached HEAD inference error", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestCreateWithAtBaseAtDetachedHeadFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)
	testutil.RunGit(t, mainWorktreePath, "checkout", "--detach")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "create", "feature-detached", "--base", "@")
	if err == nil {
		t.Fatalf("ft create --base @ should fail on detached HEAD")
	}
	if !strings.Contains(err.Error(), "HEAD is detached; @ is unavailable") {
		t.Fatalf("ft create --base @ detached HEAD error = %q, expected detached HEAD shortcut error", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)

	_, _, gitErr := testutil.RunGitWithError(t, mainWorktreePath, "show-ref", "--verify", "refs/heads/feature-detached")
	if gitErr == nil {
		t.Fatalf("feature-detached branch should not be created on detached HEAD when --base @ fails")
	}
}

func TestRemoveAtShortcutAtDetachedHeadFails(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)
	testutil.RunGit(t, mainWorktreePath, "checkout", "--detach")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "remove", "@")
	if err == nil {
		t.Fatalf("ft remove @ should fail on detached HEAD")
	}
	if !strings.Contains(err.Error(), "HEAD is detached; @ is unavailable") {
		t.Fatalf("ft remove @ detached HEAD error = %q, expected detached HEAD shortcut error", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestPRCommandFetchesAndCreatesWorktree(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	testutil.RunGit(t, "", "--git-dir", filepath.Join(repoRoot, ".git"), "update-ref", "refs/pull/123/head", "main")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "pr", "123")
	if err != nil {
		t.Fatalf("ft pr returned error: %v", err)
	}
	if !strings.Contains(stdout, "Created worktree: pull/123 ->") {
		t.Fatalf("ft pr output missing created message, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Switched to pull/123") {
		t.Fatalf("ft pr output missing switched message, got: %q", stdout)
	}
	if !strings.Contains(stdout, shell.CDMarkerPrefix) {
		t.Fatalf("ft pr output missing cd marker, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft pr stderr = %q, want empty", stderr)
	}
}

func TestPRCommandReusesExistingWorktree(t *testing.T) {
	repoRoot, mainWorktreePath := setupCLIRepo(t)
	t.Setenv(shell.EmitCDEnv, shell.EmitCDValue)

	pullBranchPath := filepath.Join(repoRoot, "pull-456")
	testutil.RunGit(t, "", "--git-dir", filepath.Join(repoRoot, ".git"), "worktree", "add", "-b", "pull/456", pullBranchPath, "main")

	testutil.RunGit(t, "", "--git-dir", filepath.Join(repoRoot, ".git"), "update-ref", "refs/pull/456/head", "pull/456")

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "pr", "456")
	if err != nil {
		t.Fatalf("ft pr returned error: %v", err)
	}
	if !strings.Contains(stdout, "Already exists: pull/456") {
		t.Fatalf("ft pr output missing exists message, got: %q", stdout)
	}
	if !strings.Contains(stdout, "Switched to pull/456") {
		t.Fatalf("ft pr output missing switched message, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("ft pr stderr = %q, want empty", stderr)
	}
}

func TestPRCommandInvalidPRNumber(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "pr", "abc")
	if err == nil {
		t.Fatalf("ft pr with invalid PR number expected error")
	}
	if !strings.Contains(err.Error(), "not a valid PR number") {
		t.Fatalf("ft pr error = %q, expected invalid PR number message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func TestPRCommandNoPRArgument(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	stdout, stderr, err := runRootCommand(t, mainWorktreePath, "pr")
	if err == nil {
		t.Fatalf("ft pr without PR number expected error")
	}
	if !strings.Contains(err.Error(), "requires exactly one argument") {
		t.Fatalf("ft pr error = %q, expected argument required message", err.Error())
	}
	assertNoOutputOnError(t, stdout, stderr)
}

func setupCLIRepo(t *testing.T) (string, string) {
	t.Helper()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "source")
	testutil.InitRepoWithMain(t, sourcePath)

	remotePath := filepath.Join(basePath, "origin.git")
	testutil.RunGit(t, "", "clone", "--bare", sourcePath, remotePath)

	targetPath := filepath.Join(basePath, "repo")
	cloneResult, err := gitx.CloneRepo(context.Background(), remotePath, targetPath)
	if err != nil {
		t.Fatalf("CloneRepo failed: %v", err)
	}

	return cloneResult.RepoRoot, cloneResult.WorktreePath
}

func runRootCommand(t *testing.T, cwd string, args ...string) (string, string, error) {
	t.Helper()

	ctx := context.Background()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newRootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	if strings.TrimSpace(cwd) != "" {
		originalWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("getwd failed: %v", err)
		}
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("chdir to %s failed: %v", cwd, err)
		}
		defer func() {
			_ = os.Chdir(originalWD)
		}()
	}

	err := cmd.ExecuteContext(ctx)
	return stdout.String(), stderr.String(), err
}

func assertNoOutputOnError(t *testing.T, stdout string, stderr string) {
	t.Helper()

	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout should be empty on error, got: %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr should be empty on error, got: %q", stderr)
	}
}
