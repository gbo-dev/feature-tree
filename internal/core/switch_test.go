package core

import (
	"context"
	"os"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
)

func TestSwitchReturnsExistingWorktreePath(t *testing.T) {
	svc, featurePath, featureBranch := setupServiceWithFeatureWorktree(t)

	result, err := svc.Switch(context.Background(), featureBranch, false, "")
	if err != nil {
		t.Fatalf("Switch returned error: %v", err)
	}
	if !result.DidSwitch {
		t.Fatalf("Switch DidSwitch = false, want true")
	}
	gotPath := testutil.CanonicalPath(t, result.Path)
	wantPath := testutil.CanonicalPath(t, featurePath)
	if gotPath != wantPath {
		t.Fatalf("Switch path = %q, want %q", result.Path, featurePath)
	}
	if result.Created {
		t.Fatalf("Switch Created = true, want false for existing worktree")
	}
}

func TestSwitchWithCreateCreatesMissingWorktree(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	result, err := svc.Switch(context.Background(), "feature-switch-create", true, "")
	if err != nil {
		t.Fatalf("Switch returned error: %v", err)
	}
	if !result.Created {
		t.Fatalf("Switch Created = false, want true")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected created worktree path %q to exist: %v", result.Path, err)
	}
}

func TestSwitchWithoutCreateFailsForMissingWorktree(t *testing.T) {
	svc, _, _ := setupServiceWithFeatureWorktree(t)

	_, err := svc.Switch(context.Background(), "feature-missing", false, "")
	if err == nil {
		t.Fatalf("Switch expected error when worktree is missing and --create is disabled")
	}
}
