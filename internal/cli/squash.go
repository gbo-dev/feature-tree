package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
)

func newSquashCmd() *cobra.Command {
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "squash [--base <branch>]",
		Short: "Squash current branch commits into one",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("unexpected arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			result, err := svc.SquashBranch(cmd.Context(), baseBranch)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Squashed %d commits on %s into one commit\n", result.Count, result.Branch); err != nil {
				return fmt.Errorf("write squash output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: detected default branch)")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}
