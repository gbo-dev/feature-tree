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

			result, err := gitx.CloneRepo(cmd.Context(), url, dir)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if _, err := fmt.Fprintf(out, "Cloned into %s\n", result.RepoRoot); err != nil {
				return fmt.Errorf("ft: write clone output: %w", err)
			}
			if _, err := fmt.Fprintf(out, "Default branch: %s\n", result.DefaultBranch); err != nil {
				return fmt.Errorf("ft: write clone output: %w", err)
			}
			if _, err := fmt.Fprintf(out, "Worktree: %s\n", result.WorktreePath); err != nil {
				return fmt.Errorf("ft: write clone output: %w", err)
			}
			if _, err := fmt.Fprintf(out, "\ncd %s/%s\n", result.RepoRoot, result.DefaultBranch); err != nil {
				return fmt.Errorf("ft: write clone output: %w", err)
			}
			return nil
		},
	}
}
