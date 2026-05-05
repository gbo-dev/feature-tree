package gitx

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestRunGitRejectsNilCommandContext(t *testing.T) {
	//nolint:staticcheck // intentional nil context for guard test
	_, _, exitCode, err := RunGit(nil, "", "rev-parse", "--git-dir")
	if err == nil {
		t.Fatalf("RunGit expected error for nil context")
	}
	if exitCode != -1 {
		t.Fatalf("RunGit exitCode = %d, want -1 for nil context", exitCode)
	}
	if !strings.Contains(err.Error(), "missing command context") {
		t.Fatalf("RunGit nil-context error = %q, expected missing context message", err.Error())
	}
}

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

func TestRunGitCommonRejectsNilCommandContext(t *testing.T) {
	repoCtx := &RepoContext{
		RepoRoot:     t.TempDir(),
		GitCommonDir: filepath.Join(t.TempDir(), ".git"),
	}

	//nolint:staticcheck // intentional nil context for guard test
	_, _, exitCode, err := RunGitCommon(nil, repoCtx, "rev-parse", "--abbrev-ref", "HEAD")
	if err == nil {
		t.Fatalf("RunGitCommon expected error for nil context")
	}
	if exitCode != -1 {
		t.Fatalf("RunGitCommon exitCode = %d, want -1 for nil context", exitCode)
	}
	if !strings.Contains(err.Error(), "missing command context") {
		t.Fatalf("RunGitCommon nil-context error = %q, expected missing context message", err.Error())
	}
}

func TestRunGitCommonWorksWhenProcessCWDWasDeleted(t *testing.T) {
	if os.Getenv("BE_DELETED_CWD_TEST") == "1" {
		repo := os.Getenv("TEST_REPO")
		deletedCWD := os.Getenv("DELETED_CWD")

		if err := os.Chdir(deletedCWD); err != nil {
			fmt.Fprintf(os.Stderr, "chdir failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.RemoveAll(deletedCWD); err != nil {
			fmt.Fprintf(os.Stderr, "remove failed: %v\n", err)
			os.Exit(1)
		}

		repoCtx := &RepoContext{
			RepoRoot:     repo,
			GitCommonDir: filepath.Join(repo, ".git"),
		}

		stdout, stderr, exitCode, runErr := RunGitCommon(context.Background(), repoCtx, "rev-parse", "--abbrev-ref", "HEAD")
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "RunGitCommon error: %v\n", runErr)
			os.Exit(1)
		}
		if exitCode != 0 {
			fmt.Fprintf(os.Stderr, "RunGitCommon exitCode = %d, stderr: %q\n", exitCode, stderr)
			os.Exit(1)
		}
		if strings.TrimSpace(stdout) != "main" {
			fmt.Fprintf(os.Stderr, "RunGitCommon stdout = %q, want main\n", stdout)
			os.Exit(1)
		}
		os.Exit(0)
	}

	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	testutil.InitRepoWithMain(t, repo)

	deletedCWD := filepath.Join(base, "deleted-cwd")
	if err := os.MkdirAll(deletedCWD, 0o755); err != nil {
		t.Fatalf("create temporary cwd: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunGitCommonWorksWhenProcessCWDWasDeleted$")
	cmd.Env = append(os.Environ(),
		"BE_DELETED_CWD_TEST=1",
		"TEST_REPO="+repo,
		"DELETED_CWD="+deletedCWD,
	)
	cmd.Dir = base

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("subprocess failed: %v\noutput: %s", err, out)
	}
}

func TestFetchOriginRejectsNilCommandContext(t *testing.T) {
	repoCtx := &RepoContext{
		RepoRoot:     t.TempDir(),
		GitCommonDir: filepath.Join(t.TempDir(), ".git"),
	}

	//nolint:staticcheck // intentional nil context for guard test
	err := FetchOrigin(nil, repoCtx)
	if err == nil {
		t.Fatalf("FetchOrigin expected error for nil context")
	}
	if !strings.Contains(err.Error(), "missing command context") {
		t.Fatalf("FetchOrigin nil-context error = %q, expected missing context message", err.Error())
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
