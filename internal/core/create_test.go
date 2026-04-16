package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestCreateWorktreeCreatesNewBranchAndPath(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	result, err := svc.CreateWorktree(context.Background(), "feature-create", "")
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

	if exists, err := gitx.BranchExistsLocal(context.Background(), svc.Ctx, "feature-create"); err != nil {
		t.Fatalf("branch existence check failed: %v", err)
	} else if !exists {
		t.Fatalf("expected created branch to exist in bare repository")
	}
}

func TestCreateWorktreeReturnsExistingWorktreeWithoutCreating(t *testing.T) {
	svc, featurePath, existingBranch := setupServiceWithFeatureWorktree(t)

	result, err := svc.CreateWorktree(context.Background(), existingBranch, "")
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}
	if result.Created {
		t.Fatalf("CreateWorktree Created = true, want false for existing branch")
	}
	gotPath := testutil.CanonicalPath(t, result.Path)
	wantPath := testutil.CanonicalPath(t, featurePath)
	if gotPath != wantPath {
		t.Fatalf("CreateWorktree path = %q, want %q", result.Path, featurePath)
	}
}

func TestCreateWorktreeUsesExistingLocalBranchWithoutWorktree(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	branch := "feature-local-only"
	testutil.RunGit(t, "", "--git-dir", svc.Ctx.GitCommonDir, "branch", branch, svc.Ctx.DefaultBranch)

	result, err := svc.CreateWorktree(context.Background(), branch, "")
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}
	if !result.Created {
		t.Fatalf("CreateWorktree Created = false, want true")
	}
	if result.Branch != branch {
		t.Fatalf("CreateWorktree branch = %q, want %q", result.Branch, branch)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected created worktree path %q to exist: %v", result.Path, err)
	}
	if filepath.Base(result.Path) != SanitizeBranchName(branch) {
		t.Fatalf("CreateWorktree path base = %q, want %q", filepath.Base(result.Path), SanitizeBranchName(branch))
	}
}
