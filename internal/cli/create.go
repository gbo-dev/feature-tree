package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/shell"
	"github.com/gbo-dev/feature-tree/internal/tui"
)

func newCreateCmd() *cobra.Command {
	var baseBranch string
	var includeAllBranches bool

	cmd := &cobra.Command{
		Use:   "create [branch]",
		Short: "Create a branch worktree",
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeCreateBranches(cmd, args, toComplete)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("unexpected arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			branch := ""
			if len(args) == 1 {
				branch = args[0]
			} else {
				if !includeAllBranches {
					return fmt.Errorf("branch name is required")
				}

				entries, err := gitx.ListWorktrees(cmd.Context(), svc.Ctx)
				if err != nil {
					return err
				}
				current, _ := gitx.CurrentBranch(cmd.Context(), "")

				if term.IsTerminal(int(os.Stdin.Fd())) {
					picked, pickErr := tui.PickCreateBranch(cmd.Context(), entries, current, svc.Ctx, includeAllBranches)
					if pickErr != nil {
						if errors.Is(pickErr, tui.ErrSelectionCancelled) {
							return fmt.Errorf("selection cancelled")
						}
						return pickErr
					}
					branch = picked
				} else {
					return fmt.Errorf("no branch specified and no interactive TTY available")
				}
			}

			result, err := svc.CreateWorktree(cmd.Context(), branch, baseBranch)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if result.Created {
				if _, err := fmt.Fprintf(out, "Created worktree: %s -> %s\n", result.Branch, result.Path); err != nil {
					return fmt.Errorf("write create output: %w", err)
				}
			} else {
				if _, err := fmt.Fprintf(out, "Already exists: %s (%s)\n", result.Branch, result.Path); err != nil {
					return fmt.Errorf("write create output: %w", err)
				}
			}

			shell.EmitCDOrWarning(result.Path, cmd.OutOrStdout(), cmd.ErrOrStderr())
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: detected default branch)")
	cmd.Flags().BoolVarP(&includeAllBranches, "all-branches", "a", false, "Include local branches without worktrees in picker")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}
