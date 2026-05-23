package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func TestForkRemoteName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		owner string
		want  string
	}{
		{owner: "octocat", want: "ft-fork-octocat"},
		{owner: "My_Org", want: "ft-fork-my-org"},
		{owner: "", want: "ft-fork-head"},
	}
	for _, tc := range cases {
		if got := forkRemoteName(tc.owner); got != tc.want {
			t.Fatalf("forkRemoteName(%q) = %q, want %q", tc.owner, got, tc.want)
		}
	}
}

func TestResolvePRMetadataFromGHIncludesStderr(t *testing.T) {
	binDir := t.TempDir()
	ghPath := filepath.Join(binDir, "gh")
	if err := os.WriteFile(ghPath, []byte("#!/bin/sh\necho gh auth failed >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	t.Setenv("PATH", binDir)

	_, err := resolvePRMetadataFromGH(context.Background(), &gitx.RepoContext{RepoRoot: t.TempDir()}, 123)
	if err == nil {
		t.Fatalf("resolvePRMetadataFromGH expected error, got nil")
	}
	if !strings.Contains(err.Error(), "gh auth failed") {
		t.Fatalf("error = %q, want gh stderr", err.Error())
	}
}
