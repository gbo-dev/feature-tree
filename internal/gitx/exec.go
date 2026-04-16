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

func requireCommandContext(commandCtx context.Context) error {
	if commandCtx == nil {
		return fmt.Errorf("missing command context")
	}
	return nil
}

func RunGit(ctx context.Context, dir string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	if err := requireCommandContext(ctx); err != nil {
		return "", "", -1, err
	}
	return runCommand(ctx, dir, "git", args...)
}

func RunGitCommon(commandCtx context.Context, repoCtx *RepoContext, args ...string) (stdout string, stderr string, exitCode int, err error) {
	if err := requireCommandContext(commandCtx); err != nil {
		return "", "", -1, err
	}
	if repoCtx == nil {
		return "", "", -1, fmt.Errorf("missing repository context")
	}

	fullArgs := append([]string{"--git-dir", repoCtx.GitCommonDir}, args...)
	workingDir := ""
	workingDir = strings.TrimSpace(repoCtx.RepoRoot)
	return runCommand(commandCtx, workingDir, "git", fullArgs...)
}

func runCommand(commandCtx context.Context, dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	if err := requireCommandContext(commandCtx); err != nil {
		return "", "", -1, err
	}

	ctx, cancel := context.WithTimeout(commandCtx, defaultCommandTimeout)
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
		return fmt.Errorf("%s: %w", action, err)
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

	return fmt.Errorf("%s: %s", action, message)
}

const fetchTimeout = 30 * time.Second

func FetchOrigin(commandCtx context.Context, ctx *RepoContext) error {
	if err := requireCommandContext(commandCtx); err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	if ctx == nil {
		return fmt.Errorf("fetch failed: missing repository context")
	}

	fetchCtx, cancel := context.WithTimeout(commandCtx, fetchTimeout)
	defer cancel()

	fullArgs := append([]string{"--git-dir", ctx.GitCommonDir}, "fetch", "origin")
	cmd := exec.CommandContext(fetchCtx, "git", fullArgs...)
	if dir := strings.TrimSpace(ctx.RepoRoot); dir != "" {
		cmd.Dir = dir
	}

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	if runErr == nil {
		return nil
	}

	if errors.Is(runErr, context.DeadlineExceeded) {
		return fmt.Errorf("fetch failed: timed out after %s", fetchTimeout)
	}
	if errors.Is(runErr, context.Canceled) {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		errText := strings.TrimSpace(errBuf.String())
		if errText != "" {
			return fmt.Errorf("fetch failed: %s", errText)
		}
		return fmt.Errorf("fetch failed: exit code %d", exitErr.ExitCode())
	}

	return fmt.Errorf("fetch failed: %w", runErr)
}

func ExpectSuccess(action string, stdout string, stderr string, exitCode int, err error, fallback string) (string, error) {
	if cmdErr := CommandError(action, stderr, exitCode, err, fallback); cmdErr != nil {
		return "", cmdErr
	}
	return stdout, nil
}
