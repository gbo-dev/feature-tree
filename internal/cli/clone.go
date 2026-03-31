package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <url> [dir]",
		Short: "Clone a repo into a bare-in-.git layout with an initial worktree",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return fmt.Errorf("ft: usage: ft clone <url> [dir]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			dir := ""
			if len(args) == 2 {
				dir = args[1]
			}

			result, err := gitx.CloneRepo(url, dir)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cloned into %s\n", result.RepoRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "Default branch: %s\n", result.DefaultBranch)
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree: %s\n", result.WorktreePath)
			fmt.Fprintf(cmd.OutOrStdout(), "\ncd %s/%s\n", result.RepoRoot, result.DefaultBranch)
			return nil
		},
	}
}
