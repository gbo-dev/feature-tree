package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestRemoveWorktreeRejectsConflictingDeleteFlags(t *testing.T) {
	svc := &Service{Ctx: &gitx.RepoContext{DefaultBranch: "main"}}

	_, err := svc.RemoveWorktree(context.Background(), "feature", false, true, true)
	if err == nil {
		t.Fatalf("RemoveWorktree expected an error for conflicting flags")
	}
	if !strings.Contains(err.Error(), "cannot use --force-branch with --no-delete-branch") {
		t.Fatalf("unexpected error for conflicting flags: %v", err)
	}
}

func TestEnsureWorktreeSafeToRemoveRejectsDirtyWorktree(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	dirtyPath := filepath.Join(featurePath, "DIRTY.txt")
	if err := os.WriteFile(dirtyPath, []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	err := svc.ensureWorktreeSafeToRemove(context.Background(), featurePath, branch, svc.Ctx.DefaultBranch)
	if err == nil {
		t.Fatalf("ensureWorktreeSafeToRemove expected dirty-worktree error")
	}
	if !strings.Contains(err.Error(), "worktree is dirty") {
		t.Fatalf("dirty-worktree error = %q, expected dirty marker", err.Error())
	}
}

func TestRemoveWorktreeFromInsideTargetWorktree(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	testutil.Chdir(t, featurePath)

	result, err := svc.RemoveWorktree(context.Background(), branch, false, false, false)
	if err != nil {
		t.Fatalf("RemoveWorktree returned unexpected error: %v", err)
	}
	if !result.DeletedIdentical {
		t.Fatalf("RemoveWorktree result = %+v, expected identical-branch deletion", result)
	}

	_, _, gitErr := testutil.RunGitWithError(t, svc.Ctx.RepoRoot, "--git-dir", svc.Ctx.GitCommonDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if gitErr == nil {
		t.Fatalf("branch %q should be deleted after successful remove", branch)
	}
}

func TestRemoveWorktreeReportsEquivalentWhenNetChangesAreZero(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	testutil.CommitEquivalentChanges(t, featurePath, "EQUIVALENT.txt")

	result, err := svc.RemoveWorktree(context.Background(), branch, false, false, false)
	if err != nil {
		t.Fatalf("RemoveWorktree returned unexpected error: %v", err)
	}
	if !result.DeletedEquivalent {
		t.Fatalf("RemoveWorktree result = %+v, expected equivalent-branch deletion", result)
	}

	_, _, gitErr := testutil.RunGitWithError(t, svc.Ctx.RepoRoot, "--git-dir", svc.Ctx.GitCommonDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if gitErr == nil {
		t.Fatalf("branch %q should be deleted after equivalent remove", branch)
	}
}

func TestBranchDeletionRelationEquivalentWithoutDefaultBranchMerge(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	testutil.CommitEquivalentChanges(t, featurePath, "EQUIVALENT-RELATION.txt")

	relation, err := svc.branchDeletionRelation(context.Background(), branch, svc.Ctx.DefaultBranch)
	if err != nil {
		t.Fatalf("branchDeletionRelation returned unexpected error: %v", err)
	}
	if relation != branchDeletionEquivalent {
		t.Fatalf("branchDeletionRelation = %v, want %v", relation, branchDeletionEquivalent)
	}
}

func TestEnsureWorktreeSafeToRemoveAllowsCleanBranchWithoutUpstreamWhenDeletable(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	err := svc.ensureWorktreeSafeToRemove(context.Background(), featurePath, branch, svc.Ctx.DefaultBranch)
	if err != nil {
		t.Fatalf("ensureWorktreeSafeToRemove returned unexpected error: %v", err)
	}
}

func TestEnsureWorktreeSafeToRemoveRejectsBranchAheadOfUpstreamWhenNotDeletable(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	testutil.RunGit(t, featurePath, "branch", "--set-upstream-to", "origin/main", branch)

	aheadFile := filepath.Join(featurePath, "AHEAD.txt")
	if err := os.WriteFile(aheadFile, []byte("ahead\n"), 0o644); err != nil {
		t.Fatalf("write ahead file: %v", err)
	}
	testutil.RunGit(t, featurePath, "add", "AHEAD.txt")
	testutil.RunGit(t, featurePath, "commit", "-m", "ahead commit")

	err := svc.ensureWorktreeSafeToRemove(context.Background(), featurePath, branch, svc.Ctx.DefaultBranch)
	if err == nil {
		t.Fatalf("ensureWorktreeSafeToRemove expected non-deletable ahead-branch error")
	}
	if !strings.Contains(err.Error(), "has commits not pushed to origin/main") {
		t.Fatalf("ahead-branch error = %q, expected upstream push warning", err.Error())
	}
}

func TestEnsureWorktreeSafeToRemoveAllowsBranchAheadOfUpstreamWhenDeletable(t *testing.T) {
	svc, featurePath, branch := setupServiceWithFeatureWorktree(t)

	testutil.RunGit(t, featurePath, "branch", "--set-upstream-to", "origin/main", branch)
	testutil.RunGit(t, featurePath, "commit", "--allow-empty", "-m", "empty ahead commit")

	err := svc.ensureWorktreeSafeToRemove(context.Background(), featurePath, branch, svc.Ctx.DefaultBranch)
	if err != nil {
		t.Fatalf("ensureWorktreeSafeToRemove returned unexpected error: %v", err)
	}
}

func TestWorktreeUpstreamRefNoUpstreamReturnsEmpty(t *testing.T) {
	repo := t.TempDir()
	testutil.InitRepoWithMain(t, repo)

	upstream, err := worktreeUpstreamRef(context.Background(), repo)
	if err != nil {
		t.Fatalf("worktreeUpstreamRef returned error: %v", err)
	}
	if upstream != "" {
		t.Fatalf("worktreeUpstreamRef = %q, want empty upstream", upstream)
	}
}

func TestWorktreeUpstreamRefOutsideRepoReturnsError(t *testing.T) {
	nonRepo := t.TempDir()

	_, err := worktreeUpstreamRef(context.Background(), nonRepo)
	if err == nil {
		t.Fatalf("worktreeUpstreamRef expected error outside repository")
	}
	if !strings.Contains(err.Error(), "resolve upstream for worktree") {
		t.Fatalf("worktreeUpstreamRef error = %q, expected upstream resolution context", err.Error())
	}
}
