package cli

import (
	"fmt"
	"strings"

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

			current, currentErr := gitx.CurrentBranch(cmd.Context(), "")
			if currentErr != nil {
				if strings.Contains(strings.ToLower(currentErr.Error()), "detached") {
					return fmt.Errorf("ft: cannot determine current branch while HEAD is detached; check out a branch and retry")
				}
				return fmt.Errorf("ft: resolve current branch for list: %w", currentErr)
			}

			return tui.PrintWorktreeList(cmd.Context(), entries, current, svc.Ctx, cmd.OutOrStdout())
		},
	}
}
