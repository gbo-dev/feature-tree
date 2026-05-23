package cli

import (
	"context"
	"testing"

	"github.com/gbo-dev/feature-tree/internal/testutil"
	"github.com/spf13/cobra"
)

func TestCompleteCreateBranchesExcludesShortcutTokens(t *testing.T) {
	_, mainWorktreePath := setupCLIRepo(t)

	testutil.Chdir(t, mainWorktreePath)

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
		if candidate == "main" {
			mainSeen = true
			break
		}
	}
	if !mainSeen {
		t.Fatalf("expected create completion to include existing branch %q; candidates=%v", "main", got)
	}
}
