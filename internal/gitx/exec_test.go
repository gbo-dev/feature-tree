package gitx

import (
	"context"
	"strings"
	"testing"
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
