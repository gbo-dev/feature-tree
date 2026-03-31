package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/gitx"
	"github.com/gbo-dev/feature-tree/internal/shell"
	"github.com/gbo-dev/feature-tree/internal/tui"
)

func newRemoveCmd() *cobra.Command {
	var forceWorktree bool
	var forceBranch bool
	var noDeleteBranch bool

	cmd := &cobra.Command{
		Use:   "remove [branch] [-f|--force-worktree] [-D|--force-branch] [--no-delete-branch]",
		Short: "Remove a branch worktree",
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
				current, currentErr := gitx.CurrentBranch(cmd.Context(), "")
				if currentErr != nil {
					return fmt.Errorf("ft: cannot infer branch from detached HEAD")
				}

				if current != svc.Ctx.DefaultBranch {
					branch = current
				} else if term.IsTerminal(int(os.Stdin.Fd())) {
					entries, err := gitx.ListWorktrees(cmd.Context(), svc.Ctx)
					if err != nil {
						return err
					}
					picked, pickErr := tui.PickRemoveBranch(cmd.Context(), entries, current, svc.Ctx)
					if pickErr != nil {
						if errors.Is(pickErr, tui.ErrSelectionCancelled) {
							return fmt.Errorf("ft: selection cancelled")
						}
						return pickErr
					}
					branch = picked
				} else {
					branch = current
				}
			}

			result, err := svc.RemoveWorktree(branch, forceWorktree, forceBranch, noDeleteBranch)
			if err != nil {
				return err
			}

			if result.NoDeleteBranch {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.Path)
			} else if result.DeletedMerged {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree and deleted merged branch: %s\n", result.Branch)
			} else if result.DeletedForced {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree and force-deleted branch: %s\n", result.Branch)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed worktree: %s\n", result.Path)
				fmt.Fprintf(cmd.OutOrStdout(), "Kept branch: %s (not merged to %s; use -D to force delete)\n", result.Branch, result.TargetRef)
			}

			if strings.TrimSpace(result.FallbackPath) != "" {
				shell.EmitCDOrWarning(result.FallbackPath, cmd.OutOrStdout(), cmd.ErrOrStderr())
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&forceWorktree, "force-worktree", "f", false, "Force remove worktree even if dirty")
	cmd.Flags().BoolVarP(&forceWorktree, "force", "", false, "Alias for --force-worktree")
	cmd.Flags().BoolVarP(&forceBranch, "force-branch", "D", false, "Force delete branch")
	cmd.Flags().BoolVar(&noDeleteBranch, "no-delete-branch", false, "Keep branch after worktree removal")
	cmd.ValidArgsFunction = completeRemovableWorktreeBranches
	return cmd
}
