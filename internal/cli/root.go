package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func ExecuteContext(ctx context.Context) error {
	gitx.SetCommandContext(ctx)
	root := newRootCmd()
	return root.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ft",
		Short:         "feature-tree – lightweight git worktree helper",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newSwitchCmd())
	cmd.AddCommand(newIncludeCmd())
	cmd.AddCommand(newSquashCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newInitCmd())

	cmd.SetHelpTemplate(helpTemplate)

	return cmd
}

const helpTemplate = `ft (feature-tree) – lightweight git worktree helper

Usage:
  ft clone <url> [dir]
  ft switch [--create] [--base <branch>] [branch]
  ft create <branch> [--base <branch>]
  ft list
  ft remove [branch] [-f|--force-worktree] [-D|--force-branch] [--no-delete-branch]
  ft squash [--base <branch>]
  ft copy-include [--from <branch>] [--to <branch>]
  ft completion [bash|zsh]
  ft init [bash|zsh]
  ft help

Notes:
  - clone sets up bare-in-.git layout, resolves origin/HEAD, and creates the first worktree.
  - Uses include manifest from default branch worktree (default: .worktreeinclude).
  - list markers: @ is current branch worktree, ^ is default branch worktree.
  - STATE symbols: + staged, ! unstaged, ? untracked; combinations (e.g. +!?) mean multiple states.
  - switch without branch opens fzf picker when running in a TTY.
  - switch/create auto-cd when shell integration is active.
  - Enable integration with: eval "$(ft init zsh)"
  - Tab completion includes branch/worktree names for switch/create and --base.
  - init prints the ft() wrapper for auto-cd; completion prints completion definitions only.
`
