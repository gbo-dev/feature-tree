package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestListLocalBranchSnapshotsIncludesCommitAndRelation(t *testing.T) {
	repoCtx, _, featureWorktreePath := setupWorktreeTestRepo(t)

	filePath := filepath.Join(featureWorktreePath, "snapshot-test.txt")
	if err := os.WriteFile(filePath, []byte("snapshot\n"), 0o644); err != nil {
		t.Fatalf("write snapshot file: %v", err)
	}
	testutil.RunGit(t, featureWorktreePath, "add", "snapshot-test.txt")
	testutil.RunGit(t, featureWorktreePath, "commit", "-m", "snapshot branch commit")

	snapshots, err := ListLocalBranchSnapshots(context.Background(), repoCtx)
	if err != nil {
		t.Fatalf("ListLocalBranchSnapshots returned error: %v", err)
	}

	byBranch := make(map[string]LocalBranchSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		byBranch[snapshot.Branch] = snapshot
	}

	mainSnapshot, ok := byBranch[repoCtx.DefaultBranch]
	if !ok {
		t.Fatalf("default branch %q missing from snapshots", repoCtx.DefaultBranch)
	}
	if mainSnapshot.Relation != "-" {
		t.Fatalf("default branch relation = %q, want \"-\"", mainSnapshot.Relation)
	}
	if strings.TrimSpace(mainSnapshot.Commit.Hash) == "" || strings.TrimSpace(mainSnapshot.Commit.Subject) == "" {
		t.Fatalf("default branch commit info should be populated, got %+v", mainSnapshot.Commit)
	}

	featureSnapshot, ok := byBranch["feature-worktree"]
	if !ok {
		t.Fatalf("feature-worktree missing from snapshots")
	}
	if !strings.Contains(featureSnapshot.Relation, "A:") || !strings.Contains(featureSnapshot.Relation, "B:") {
		t.Fatalf("feature-worktree relation = %q, expected ahead/behind format", featureSnapshot.Relation)
	}
	if strings.TrimSpace(featureSnapshot.Commit.Hash) == "" || strings.TrimSpace(featureSnapshot.Commit.Subject) == "" {
		t.Fatalf("feature-worktree commit info should be populated, got %+v", featureSnapshot.Commit)
	}
}

func TestFetchBranchRelationsParallelReturnsUnknownForMissingBranch(t *testing.T) {
	repoCtx, _, _ := setupWorktreeTestRepo(t)

	relations := FetchBranchRelationsParallel(context.Background(), repoCtx, []string{
		repoCtx.DefaultBranch,
		"feature-worktree",
		"missing-branch",
	})
	if len(relations) != 3 {
		t.Fatalf("FetchBranchRelationsParallel len = %d, want 3", len(relations))
	}
	if relations[0] != "-" {
		t.Fatalf("default branch relation = %q, want \"-\"", relations[0])
	}
	if !strings.Contains(relations[1], "A:") || !strings.Contains(relations[1], "B:") {
		t.Fatalf("feature-worktree relation = %q, expected ahead/behind format", relations[1])
	}
	if relations[2] != "?" {
		t.Fatalf("missing-branch relation = %q, want \"?\"", relations[2])
	}
}
