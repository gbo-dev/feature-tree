package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

const defaultCommandTimeout = 5 * time.Minute

var commandContext atomic.Value

type commandContextHolder struct {
	ctx context.Context
}

func init() {
	commandContext.Store(commandContextHolder{ctx: context.Background()})
}

func SetCommandContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	commandContext.Store(commandContextHolder{ctx: ctx})
}

func RunGit(dir string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	return runCommand(dir, "git", args...)
}

func RunGitCommon(ctx *RepoContext, args ...string) (stdout string, stderr string, exitCode int, err error) {
	fullArgs := append([]string{"--git-dir", ctx.GitCommonDir}, args...)
	return runCommand("", "git", fullArgs...)
}

func runCommand(dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	baseCtx := context.Background()
	if loaded := commandContext.Load(); loaded != nil {
		if holder, ok := loaded.(commandContextHolder); ok && holder.ctx != nil {
			baseCtx = holder.ctx
		}
	}

	ctx, cancel := context.WithTimeout(baseCtx, defaultCommandTimeout)
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
