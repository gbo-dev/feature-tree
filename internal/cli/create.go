package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/shell"
)

func newCreateCmd() *cobra.Command {
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "create <branch>",
		Short: "Create a branch worktree",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeSwitchBranches(cmd, args, toComplete)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("ft: branch name is required")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			result, err := svc.CreateWorktree(args[0], baseBranch)
			if err != nil {
				return err
			}

			if result.Created {
				fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s -> %s\n", result.Branch, result.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Already exists: %s (%s)\n", result.Branch, result.Path)
			}

			shell.EmitCDOrWarning(result.Path, cmd.OutOrStdout(), cmd.ErrOrStderr())
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: detected default branch)")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}
