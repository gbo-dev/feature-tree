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
		t.Fatalf("ft create without branch expected non-interactive error")
	}
	if !strings.Contains(err.Error(), "no branch specified and no interactive TTY available") {
		t.Fatalf("ft create error = %q, expected non-interactive message", err.Error())
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
