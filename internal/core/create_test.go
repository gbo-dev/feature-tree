package core

import (
	"os"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func TestCreateWorktreeCreatesNewBranchAndPath(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	result, err := svc.CreateWorktree("feature-create", "")
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}
	if !result.Created {
		t.Fatalf("CreateWorktree Created = false, want true")
	}
	if result.Branch != "feature-create" {
		t.Fatalf("CreateWorktree branch = %q, want %q", result.Branch, "feature-create")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected created worktree path %q to exist: %v", result.Path, err)
	}

	if exists, err := gitx.BranchExistsLocal(svc.CommandCtx, svc.Ctx, "feature-create"); err != nil {
		t.Fatalf("branch existence check failed: %v", err)
	} else if !exists {
		t.Fatalf("expected created branch to exist in bare repository")
	}
}

func TestCreateWorktreeReturnsExistingWorktreeWithoutCreating(t *testing.T) {
	svc, featurePath, existingBranch := setupServiceWithFeatureWorktree(t)

	result, err := svc.CreateWorktree(existingBranch, "")
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}
	if result.Created {
		t.Fatalf("CreateWorktree Created = true, want false for existing branch")
	}
	if result.Path != featurePath {
		t.Fatalf("CreateWorktree path = %q, want %q", result.Path, featurePath)
	}
}
