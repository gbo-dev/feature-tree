package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestRunGitHonorsCanceledCommandContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, exitCode, err := RunGit(ctx, "", "rev-parse", "--git-dir")
	if err == nil {
		t.Fatalf("RunGit expected canceled context error")
	}
	if exitCode != -1 {
		t.Fatalf("RunGit exitCode = %d, want -1 for context cancellation", exitCode)
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("RunGit cancellation error = %q, expected canceled marker", err.Error())
	}
}

func TestCommandErrorUsesFallbackWhenStderrMissing(t *testing.T) {
	err := CommandError("example action", "", 2, nil, "fallback message")
	if err == nil {
		t.Fatalf("CommandError expected non-nil error")
	}
	if !strings.Contains(err.Error(), "fallback message") {
		t.Fatalf("CommandError = %q, expected fallback message", err.Error())
	}
}

func TestRunGitCommonWorksWhenProcessCWDWasDeleted(t *testing.T) {
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	testutil.InitRepoWithMain(t, repo)

	repoCtx := &RepoContext{
		RepoRoot:     repo,
		GitCommonDir: filepath.Join(repo, ".git"),
	}

	deletedCWD := filepath.Join(base, "deleted-cwd")
	if err := os.MkdirAll(deletedCWD, 0o755); err != nil {
		t.Fatalf("create temporary cwd: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(deletedCWD); err != nil {
		t.Fatalf("chdir to temporary cwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if err := os.RemoveAll(deletedCWD); err != nil {
		t.Fatalf("remove temporary cwd failed: %v", err)
	}

	stdout, stderr, exitCode, runErr := RunGitCommon(context.Background(), repoCtx, "rev-parse", "--abbrev-ref", "HEAD")
	if runErr != nil {
		t.Fatalf("RunGitCommon returned unexpected error: %v", runErr)
	}
	if exitCode != 0 {
		t.Fatalf("RunGitCommon exitCode = %d, want 0 (stderr: %q)", exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != "main" {
		t.Fatalf("RunGitCommon stdout = %q, want %q", stdout, "main")
	}
}

func TestFetchOriginPrefixesErrors(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepoWithMain(t, repo)

	repoCtx := &RepoContext{
		RepoRoot:     repo,
		GitCommonDir: filepath.Join(repo, ".git"),
	}

	err := FetchOrigin(context.Background(), repoCtx)
	if err == nil {
		t.Fatalf("FetchOrigin expected error when origin remote is missing")
	}
	if !strings.Contains(err.Error(), "fetch failed:") {
		t.Fatalf("FetchOrigin error = %q, expected fetch failure context prefix", err.Error())
	}
}

func TestFetchOriginTreatsCanceledContextAsNoOp(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	testutil.InitRepoWithMain(t, repo)

	repoCtx := &RepoContext{
		RepoRoot:     repo,
		GitCommonDir: filepath.Join(repo, ".git"),
	}

	commandCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := FetchOrigin(commandCtx, repoCtx); err != nil {
		t.Fatalf("FetchOrigin on canceled context = %v, want nil", err)
	}
}
