package core

import (
	"testing"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func TestWorktreeBranchPathMismatch(t *testing.T) {
	repoRoot := "/repo"

	tests := []struct {
		name string
		wt   gitx.Worktree
		want bool
	}{
		{
			name: "aligned main worktree",
			wt:   gitx.Worktree{Path: "/repo/main", Branch: "main"},
			want: false,
		},
		{
			name: "aligned sanitized branch name",
			wt:   gitx.Worktree{Path: "/repo/feature-a", Branch: "feature/a"},
			want: false,
		},
		{
			name: "checked out branch differs from dedicated directory",
			wt:   gitx.Worktree{Path: "/repo/feature-a", Branch: "other-branch"},
			want: true,
		},
		{
			name: "ignore worktree outside repo root layout",
			wt:   gitx.Worktree{Path: "/elsewhere/custom", Branch: "other-branch"},
			want: false,
		},
		{
			name: "ignore nested path under repo root",
			wt:   gitx.Worktree{Path: "/repo/nested/feature-a", Branch: "other-branch"},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WorktreeBranchPathMismatch(repoRoot, tc.wt)
			if got != tc.want {
				t.Fatalf("WorktreeBranchPathMismatch() = %v, want %v", got, tc.want)
			}
		})
	}
}
