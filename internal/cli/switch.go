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

func newSwitchCmd() *cobra.Command {
	var createIfMissing bool
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "switch [branch]",
		Short: "Switch to an existing worktree branch",
		Long: `Switch to an existing worktree branch.

Interactive picker notes:
- STATE shows clean or combinations of +, !, ?
- + means staged changes
- ! means unstaged changes
- ? means untracked files
- Preview tabs: tab/s-tab`,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completeSwitchBranches(cmd, args, toComplete)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("ft: unexpected arguments")
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
				entries, err := gitx.ListWorktrees(cmd.Context(), svc.Ctx)
				if err != nil {
					return err
				}

				current, _ := gitx.CurrentBranch(cmd.Context(), "")
				if term.IsTerminal(int(os.Stdin.Fd())) {
					picked, pickErr := tui.PickSwitchBranch(cmd.Context(), entries, current, svc.Ctx)
					if pickErr != nil {
						if errors.Is(pickErr, tui.ErrSelectionCancelled) {
							return fmt.Errorf("ft: selection cancelled")
						}
						return pickErr
					}
					branch = picked
				} else {
					return fmt.Errorf("ft: no branch specified and no interactive TTY available")
				}
			}

			result, err := svc.Switch(branch, createIfMissing, baseBranch)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if result.Created {
				if _, err := fmt.Fprintf(out, "Created worktree: %s -> %s\n", result.Branch, result.Path); err != nil {
					return fmt.Errorf("ft: write switch output: %w", err)
				}
			}
			if _, err := fmt.Fprintf(out, "Switched to %s (%s)\n", result.Branch, result.Path); err != nil {
				return fmt.Errorf("ft: write switch output: %w", err)
			}
			shell.EmitCDOrWarning(result.Path, cmd.OutOrStdout(), cmd.ErrOrStderr())

			return nil
		},
	}

	cmd.Flags().BoolVarP(&createIfMissing, "create", "c", false, "Create worktree if branch is missing")
	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch used with --create")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}
