package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gbo-dev/feature-tree/internal/core"
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
				current, currentErr := svc.CurrentBranch(cmd.Context())
				if currentErr != nil {
					return mapDetachedHead(currentErr, "cannot infer branch from detached HEAD")
				}

				if current != svc.Ctx.DefaultBranch {
					branch = current
				} else if term.IsTerminal(int(os.Stdin.Fd())) {
					state, err := svc.WorktreeState(cmd.Context())
					if err != nil {
						return err
					}
					picked, pickErr := tui.PickRemoveBranch(cmd.Context(), state.Entries, current, svc.Ctx)
					if pickErr != nil {
						return handlePickerError(pickErr)
					}
					branch = picked
				} else {
					branch = current
				}
			}

			result, err := svc.RemoveWorktree(cmd.Context(), branch, forceWorktree, forceBranch, noDeleteBranch)
			if err != nil {
				return err
			}

			if strings.TrimSpace(result.FetchWarning) != "" {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "could not fetch from origin (%s); using cached refs\n", result.FetchWarning)
			}

			if err := renderRemoveResult(cmd.OutOrStdout(), result, svc.Ctx.DefaultBranch); err != nil {
				return err
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

func renderRemoveResult(w io.Writer, result *core.RemoveResult, defaultBranch string) error {
	writeLine := func(format string, args ...any) error {
		if _, err := fmt.Fprintf(w, format, args...); err != nil {
			return errWriteOutput("remove", err)
		}
		return nil
	}

	if result.NoDeleteBranch {
		return writeLine("Removed worktree: %s\n", result.Path)
	}
	if result.DeletedMerged {
		if result.TargetRef == defaultBranch {
			return writeLine("Removed worktree and deleted merged branch: %s\n", result.Branch)
		}
		return writeLine("Removed worktree and deleted branch: %s (fully contained in %s)\n", result.Branch, result.TargetRef)
	}
	if result.DeletedIdentical {
		return writeLine("Removed worktree and deleted branch: %s (identical to %s)\n", result.Branch, result.TargetRef)
	}
	if result.DeletedEquivalent {
		return writeLine("Removed worktree and deleted branch: %s (no effective changes vs %s)\n", result.Branch, result.TargetRef)
	}
	if result.DeletedForced {
		return writeLine("Removed worktree and force-deleted branch: %s\n", result.Branch)
	}
	if err := writeLine("Removed worktree: %s\n", result.Path); err != nil {
		return err
	}
	return writeLine("Kept branch: %s (not merged to %s; use -D to force delete)\n", result.Branch, result.TargetRef)
}
