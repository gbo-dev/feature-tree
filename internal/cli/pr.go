package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/shell"
)

func newPRCmd() *cobra.Command {
	var usePRRef bool

	cmd := &cobra.Command{
		Use:   "pr <num>",
		Short: "Fetch and checkout a PR into a new worktree",
		Long: `Fetch a pull request from origin and create a worktree for it.

This command:
1. Fetches the PR ref from origin (or uses cached ref)
2. Ensures the local ref is up-to-date with the remote
3. Creates a new worktree for the PR branch
4. Switches to the new worktree

	By default, ft tries to use the PR head branch name for the local branch/worktree.
	Use --use-pr-ref to always use "pull/<num>" instead.

Examples:
  ft pr 123         # Checkout PR #123 into a new worktree
  ft pr 42          # Checkout PR #42`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("requires exactly one argument: PR number")
			}
			_, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a valid PR number", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			prNum, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%q is not a valid PR number", args[0])
			}

			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			result, err := svc.FetchAndCheckoutPRWithOptions(cmd.Context(), prNum, core.PRCheckoutOptions{
				UsePRRef: usePRRef,
			})
			if err != nil {
				return err
			}

			for _, warning := range result.Warnings {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning); err != nil {
					return fmt.Errorf("write pr output: %w", err)
				}
			}

			out := cmd.OutOrStdout()
			if result.Created {
				if _, err := fmt.Fprintf(out, "Created worktree: %s -> %s\n", result.Branch, result.Path); err != nil {
					return fmt.Errorf("write pr output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(out, "Already exists: %s (%s)\n", result.Branch, result.Path); err != nil {
					return fmt.Errorf("write pr output: %w", err)
				}
			}

			if _, err := fmt.Fprintf(out, "Switched to %s (%s)\n", result.Branch, result.Path); err != nil {
				return fmt.Errorf("write pr output: %w", err)
			}
			shell.EmitCDOrWarning(result.Path, cmd.OutOrStdout(), cmd.ErrOrStderr())

			return nil
		},
	}

	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cmd.Flags().BoolVar(&usePRRef, "use-pr-ref", false, `Use "pull/<num>" for the local branch/worktree name`)

	return cmd
}
