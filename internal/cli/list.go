package cli

import (
	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/gitx"
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

			entries, err := gitx.ListWorktrees(cmd.Context(), svc.Ctx)
			if err != nil {
				return err
			}

			current, _ := gitx.CurrentBranch(cmd.Context(), "")
			return tui.PrintWorktreeList(cmd.Context(), entries, current, svc.Ctx, cmd.OutOrStdout())
		},
	}
}
