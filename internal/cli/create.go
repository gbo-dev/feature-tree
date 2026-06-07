package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gbo-dev/feature-tree/internal/core"
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

				state, err := svc.WorktreeState(cmd.Context())
				if err != nil {
					return mapDetachedHead(err, "cannot infer branch from detached HEAD")
				}

				if term.IsTerminal(int(os.Stdin.Fd())) {
					picked, pickErr := tui.PickCreateBranch(cmd.Context(), state.Entries, state.CurrentBranch, svc.Ctx, includeAllBranches)
					if pickErr != nil {
						return handlePickerError(pickErr)
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
					return errWriteOutput("create", err)
				}
			} else {
				if _, err := fmt.Fprintf(out, "Already exists: %s (%s)\n", result.Branch, result.Path); err != nil {
					return errWriteOutput("create", err)
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
