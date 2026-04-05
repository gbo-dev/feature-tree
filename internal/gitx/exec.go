package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultCommandTimeout = 5 * time.Minute

func normalizeCommandContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func RunGit(ctx context.Context, dir string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	return runCommand(normalizeCommandContext(ctx), dir, "git", args...)
}

func RunGitCommon(commandCtx context.Context, repoCtx *RepoContext, args ...string) (stdout string, stderr string, exitCode int, err error) {
	fullArgs := append([]string{"--git-dir", repoCtx.GitCommonDir}, args...)
	workingDir := ""
	if repoCtx != nil {
		workingDir = strings.TrimSpace(repoCtx.RepoRoot)
	}
	return runCommand(normalizeCommandContext(commandCtx), workingDir, "git", fullArgs...)
}

func runCommand(commandCtx context.Context, dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(normalizeCommandContext(commandCtx), defaultCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	out := strings.TrimSpace(outBuf.String())
	errText := strings.TrimSpace(errBuf.String())

	if runErr == nil {
		return out, errText, 0, nil
	}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return out, errText, -1, fmt.Errorf("%s timed out after %s", name, defaultCommandTimeout)
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return out, errText, -1, fmt.Errorf("%s canceled", name)
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return out, errText, exitErr.ExitCode(), nil
	}

	return out, errText, -1, runErr
}

func CommandError(action string, stderr string, exitCode int, err error, fallback string) error {
	if err != nil {
		return fmt.Errorf("ft: %s: %w", action, err)
	}
	if exitCode == 0 {
		return nil
	}

	message := strings.TrimSpace(stderr)
	if message == "" {
		message = strings.TrimSpace(fallback)
	}
	if message == "" {
		message = fmt.Sprintf("command failed with exit code %d", exitCode)
	}

	return fmt.Errorf("ft: %s: %s", action, message)
}

func ExpectSuccess(action string, stdout string, stderr string, exitCode int, err error, fallback string) (string, error) {
	if cmdErr := CommandError(action, stderr, exitCode, err, fallback); cmdErr != nil {
		return "", cmdErr
	}
	return stdout, nil
}
