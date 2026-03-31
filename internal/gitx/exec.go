package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func RunGit(dir string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	return runCommand(dir, "git", args...)
}

func RunGitCommon(ctx *RepoContext, args ...string) (stdout string, stderr string, exitCode int, err error) {
	fullArgs := append([]string{"--git-dir", ctx.GitCommonDir}, args...)
	return runCommand("", "git", fullArgs...)
}

func runCommand(dir string, name string, args ...string) (stdout string, stderr string, exitCode int, err error) {
	cmd := exec.Command(name, args...)
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

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return out, errText, exitErr.ExitCode(), nil
	}

	return out, errText, -1, runErr
}

func expectSuccess(action string, stdout string, stderr string, exitCode int, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("%s: %w", action, err)
	}
	if exitCode != 0 {
		if stderr != "" {
			return "", fmt.Errorf("%s: %s", action, stderr)
		}
		return "", fmt.Errorf("%s: exit code %d", action, exitCode)
	}
	return stdout, nil
}
