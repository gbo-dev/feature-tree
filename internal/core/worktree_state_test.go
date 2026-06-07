package core

import (
	"context"
	"errors"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestWorktreeStateReturnsEntriesAndCurrentBranch(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	entries, err := gitx.ListWorktrees(context.Background(), svc.Ctx)
	if err != nil {
		t.Fatalf("ListWorktrees returned error: %v", err)
	}
	for _, entry := range entries {
		if entry.Branch == svc.Ctx.DefaultBranch {
			testutil.Chdir(t, entry.Path)
			break
		}
	}

	state, err := svc.WorktreeState(context.Background())
	if err != nil {
		t.Fatalf("WorktreeState returned error: %v", err)
	}
	if len(state.Entries) < 2 {
		t.Fatalf("WorktreeState returned %d entries, want at least 2", len(state.Entries))
	}
	if state.CurrentBranch != svc.Ctx.DefaultBranch {
		t.Fatalf("WorktreeState current branch = %q, want %q", state.CurrentBranch, svc.Ctx.DefaultBranch)
	}
}

func TestWorktreeStateDetachedHead(t *testing.T) {
	svc, featurePath, _ := setupServiceWithFeatureWorktree(t)
	testutil.Chdir(t, featurePath)

	head := testutil.RunGit(t, featurePath, "rev-parse", "HEAD")
	testutil.RunGit(t, featurePath, "checkout", "--detach", head)

	_, err := svc.WorktreeState(context.Background())
	if err == nil {
		t.Fatalf("WorktreeState expected error on detached HEAD")
	}
	if !errors.Is(err, gitx.ErrDetachedHead) {
		t.Fatalf("WorktreeState detached HEAD error = %v, want ErrDetachedHead", err)
	}
}

func TestCurrentBranchRejectsNilContext(t *testing.T) {
	svc := &Service{Ctx: &gitx.RepoContext{DefaultBranch: "main"}}
	_, err := svc.CurrentBranch(nil)
	if err == nil {
		t.Fatalf("CurrentBranch expected error for nil context")
	}
}
