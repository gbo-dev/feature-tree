package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteCreateBranchesExcludesShortcutTokens(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(mainWorktreePath); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	got, directive := completeCreateBranches(cmd, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}

	for _, candidate := range got {
		if candidate == "^" || candidate == "@" {
			t.Fatalf("create completion should not include shortcut %q; candidates=%v", candidate, got)
		}
	}

	mainSeen := false
	for _, candidate := range got {
		if candidate == filepath.Base(mainWorktreePath) {
			mainSeen = true
			break
		}
	}
	if !mainSeen {
		t.Fatalf("expected create completion to include existing branch %q; candidates=%v", filepath.Base(mainWorktreePath), got)
	}
}
