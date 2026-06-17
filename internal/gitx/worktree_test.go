package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestWorktreeHelpersListAndBranchQueries(t *testing.T) {
	repoCtx, mainWorktreePath, featureWorktreePath := setupWorktreeTestRepo(t)

	entries, err := ListWorktrees(context.Background(), repoCtx)
	if err != nil {
		t.Fatalf("ListWorktrees returned error: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("ListWorktrees returned %d entries, want at least 2", len(entries))
	}

	mainFound := false
	featureFound := false
	wantMainPath := testutil.CanonicalPath(t, mainWorktreePath)
	wantFeaturePath := testutil.CanonicalPath(t, featureWorktreePath)
	for _, entry := range entries {
		entryPath := testutil.CanonicalPath(t, entry.Path)
		if entryPath == wantMainPath && entry.Branch == "main" {
			mainFound = true
		}
		if entryPath == wantFeaturePath && entry.Branch == "feature-worktree" {
			featureFound = true
		}
	}
	if !mainFound || !featureFound {
		t.Fatalf("expected both main and feature worktrees to be present; entries=%v", entries)
	}

	current, err := CurrentBranch(context.Background(), mainWorktreePath)
	if err != nil {
		t.Fatalf("CurrentBranch returned error: %v", err)
	}
	if current != "main" {
		t.Fatalf("CurrentBranch = %q, want %q", current, "main")
	}

	exists, err := BranchExistsLocal(context.Background(), repoCtx, "feature-worktree")
	if err != nil {
		t.Fatalf("BranchExistsLocal returned error: %v", err)
	}
	if !exists {
		t.Fatalf("BranchExistsLocal should report feature-worktree as existing")
	}
}

func TestWorktreeHelpersDirtySymbolsAndRelation(t *testing.T) {
	repoCtx, _, featureWorktreePath := setupWorktreeTestRepo(t)

	newFile := filepath.Join(featureWorktreePath, "UNTRACKED.txt")
	if err := os.WriteFile(newFile, []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	symbols, err := DirtySymbols(context.Background(), featureWorktreePath)
	if err != nil {
		t.Fatalf("DirtySymbols returned error: %v", err)
	}
	if !strings.Contains(symbols, "?") {
		t.Fatalf("DirtySymbols = %q, expected untracked symbol '?')", symbols)
	}

	testutil.RunGit(t, featureWorktreePath, "add", "UNTRACKED.txt")
	testutil.RunGit(t, featureWorktreePath, "commit", "-m", "feature commit")

	relation, err := BranchRelation(context.Background(), repoCtx, "feature-worktree")
	if err != nil {
		t.Fatalf("BranchRelation returned error: %v", err)
	}
	if !strings.Contains(relation, "A:") || !strings.Contains(relation, "B:") {
		t.Fatalf("BranchRelation = %q, expected ahead/behind format", relation)
	}
}

func setupWorktreeTestRepo(t *testing.T) (*RepoContext, string, string) {
	t.Helper()

	const branch = "feature-worktree"
	clone, featurePath := setupClonedRepoWithWorktree(t, branch)
	return repoContextFromClone(clone), clone.WorktreePath, featurePath
}
