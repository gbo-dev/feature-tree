package cli

import (
	"context"
	"fmt"

	"github.com/gbo-dev/feature-tree/internal/uiansi"
	"github.com/spf13/cobra"
)

const ansiBold = "\x1b[1m"

func ExecuteContext(ctx context.Context) error {
	root := newRootCmd()
	return root.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ft",
		Short:         "feature-tree – lightweight git worktree helper",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return renderRootOverview(cmd)
		},
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newSwitchCmd())
	cmd.AddCommand(newPRCmd())
	cmd.AddCommand(newIncludeCmd())
	cmd.AddCommand(newSquashCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newPickerPreviewCmd())
	cmd.AddCommand(newPickerPreviewStateCmd())
	cmd.AddCommand(newPickerPreviewTabCmd())

	cmd.SetHelpTemplate(helpTemplate)

	return cmd
}

var helpTemplate = fmt.Sprintf(`%s%sft%s (feature-tree) – lightweight git worktree helper

%sUsage:%s
  %sft%s clone <url> [dir]
  %sft%s switch [--create] [--base <branch>] [branch]
  %sft%s create [--all-branches] [--base <branch>] [branch]
  %sft%s pr <num>
  %sft%s list
  %sft%s remove [branch] [-f|--force-worktree] [-D|--force-branch] [--no-delete-branch]
  %sft%s squash [--base <branch>]
  %sft%s copy-include [--from <branch>] [--to <branch>]
  %sft%s completion [bash|zsh]
  %sft%s init [bash|zsh]
  %sft%s help

%sNotes:%s
  - clone sets up bare-in-.git layout, resolves origin/HEAD, and creates the first worktree.
  - Uses include manifest from default branch worktree (default: .worktreeinclude).
  - list markers: @ is current branch worktree, ^ is default branch worktree.
  - STATE shows clean or symbols: + staged, ! unstaged, ? untracked; combinations (e.g. +!?) mean multiple states.
  - switch without branch opens fzf picker when running in a TTY (preview tabs: tab/s-tab).
  - create without branch requires an explicit name unless --all-branches is used.
  - create --all-branches without branch opens fzf picker when running in a TTY.
  - switch/create auto-cd when shell integration is active.
  - Enable integration with: eval "$(ft init zsh)"
  - Tab completion includes branch/worktree names for switch/create and --base.
  - init prints the ft() wrapper for auto-cd; completion prints completion definitions only.
`,
	ansiBold, uiansi.InfoPurple, uiansi.Reset,
	uiansi.InfoPurple, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.Periwinkle, uiansi.Reset,
	uiansi.InfoPurple, uiansi.Reset,
)

func renderRootOverview(cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	if _, err := fmt.Fprintf(w, "%s%sft%s (feature-tree) – lightweight git worktree helper\n\n", ansiBold, uiansi.InfoPurple, uiansi.Reset); err != nil {
		return fmt.Errorf("ft: write overview output: %w", err)
	}
	if _, err := fmt.Fprintf(w, "%sUsage:%s\n", uiansi.InfoPurple, uiansi.Reset); err != nil {
		return fmt.Errorf("ft: write overview output: %w", err)
	}
	if _, err := fmt.Fprintf(w, "  %sft%s <command> [flags]\n\n", uiansi.Periwinkle, uiansi.Reset); err != nil {
		return fmt.Errorf("ft: write overview output: %w", err)
	}

	visibleCommands := make([]*cobra.Command, 0, len(cmd.Commands()))
	maxNameWidth := 0
	for _, sub := range cmd.Commands() {
		if !sub.IsAvailableCommand() || sub.Hidden {
			continue
		}
		visibleCommands = append(visibleCommands, sub)
		if n := len(sub.Name()); n > maxNameWidth {
			maxNameWidth = n
		}
	}

	if _, err := fmt.Fprintf(w, "%sAvailable Commands:%s\n", uiansi.InfoPurple, uiansi.Reset); err != nil {
		return fmt.Errorf("ft: write overview output: %w", err)
	}
	for _, sub := range visibleCommands {
		if _, err := fmt.Fprintf(w, "  %s%-*s%s  %s\n", uiansi.Periwinkle, maxNameWidth, sub.Name(), uiansi.Reset, sub.Short); err != nil {
			return fmt.Errorf("ft: write overview output: %w", err)
		}
	}

	if _, err := fmt.Fprintf(w, "\nRun %sft --help%s for full details.\n", uiansi.Periwinkle, uiansi.Reset); err != nil {
		return fmt.Errorf("ft: write overview output: %w", err)
	}

	return nil
}
