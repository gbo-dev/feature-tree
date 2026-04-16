package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func newIncludeCmd() *cobra.Command {
	var fromBranch string
	var toBranch string

	cmd := &cobra.Command{
		Use:   "copy-include [--from <branch>] [--to <branch>]",
		Short: "Copy include-defined files between worktrees",
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

			from := fromBranch
			if strings.TrimSpace(from) == "" {
				from = svc.Ctx.DefaultBranch
			}
			from, err = svc.ResolveBranchShortcut(from)
			if err != nil {
				return err
			}

			to := toBranch
			if strings.TrimSpace(to) == "" {
				to, err = gitx.CurrentBranch(cmd.Context(), "")
				if err != nil {
					return fmt.Errorf("cannot infer destination branch from detached HEAD")
				}
			}
			to, err = svc.ResolveBranchShortcut(to)
			if err != nil {
				return err
			}

			if err := svc.CopyIncludeBetweenBranches(from, to); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Copied include entries from %s to %s\n", from, to); err != nil {
				return fmt.Errorf("write copy-include output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fromBranch, "from", "", "Source branch (default: default branch)")
	cmd.Flags().StringVar(&toBranch, "to", "", "Destination branch (default: current branch)")
	_ = cmd.RegisterFlagCompletionFunc("from", completeLocalBranchesWithShortcuts)
	_ = cmd.RegisterFlagCompletionFunc("to", completeSwitchBranches)
	return cmd
}
