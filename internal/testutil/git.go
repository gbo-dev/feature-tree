package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=ft-test",
		"GIT_AUTHOR_EMAIL=ft-test@example.com",
		"GIT_COMMITTER_NAME=ft-test",
		"GIT_COMMITTER_EMAIL=ft-test@example.com",
	)
}

func RunGitWithError(t *testing.T, dir string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()

	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = gitEnv()

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	stdout = strings.TrimSpace(outBuf.String())
	stderr = strings.TrimSpace(errBuf.String())
	return stdout, stderr, err
}

func RunGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, err := RunGitWithError(t, dir, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\nstderr: %s", args, err, stderr)
	}
	return stdout
}

func Chdir(t *testing.T, dir string) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
}

func CanonicalPath(t *testing.T, path string) string {
	t.Helper()

	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolve absolute path %q: %v", path, err)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}

	return filepath.Clean(resolved)
}

func InitRepoWithMain(t *testing.T, dir string) {
	t.Helper()

	RunGit(t, "", "init", dir)

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("test repo\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", readmePath, err)
	}

	RunGit(t, dir, "add", "README.md")
	RunGit(t, dir, "commit", "-m", "initial commit")
	RunGit(t, dir, "branch", "-M", "main")
}
