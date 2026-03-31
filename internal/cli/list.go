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
			svc, err := core.NewService()
			if err != nil {
				return err
			}

			entries, err := gitx.ListWorktrees(svc.Ctx)
			if err != nil {
				return err
			}

			current, _ := gitx.CurrentBranch("")
			return tui.PrintWorktreeList(entries, current, svc.Ctx, cmd.OutOrStdout())
		},
	}
}
