package cli

import (
	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/tui"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees with status indicators",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			state, err := svc.WorktreeState(cmd.Context())
			if err != nil {
				return mapCurrentBranchForList(err)
			}

			return tui.PrintWorktreeList(cmd.Context(), state.Entries, state.CurrentBranch, svc.Ctx, cmd.OutOrStdout())
		},
	}
}
