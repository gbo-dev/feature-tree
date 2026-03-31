package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gbo-dev/wt/go-port/internal/core"
	"github.com/gbo-dev/wt/go-port/internal/gitx"
	"github.com/gbo-dev/wt/go-port/internal/shell"
	"github.com/gbo-dev/wt/go-port/internal/tui"
)

func Execute() error {
	root := newRootCmd()
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ft",
		Short:         "featuretree – lightweight git worktree helper",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newSwitchCmd())
	cmd.AddCommand(newCopyIncludeCmd())
	cmd.AddCommand(newSquashCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newCompletionCmd())
	cmd.AddCommand(newInitCmd())

	cmd.SetHelpTemplate(helpTemplate)

	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees with status indicators",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService()
			if err != nil {
				return err
			}

			entries, err := gitx.ListWorktrees(svc.Ctx)
			if err != nil {
				return err
			}

			current, _ := gitx.CurrentBranch("")
			return tui.PrintWorktreeList(entries, current, svc.Ctx, cmd.OutOrStdout())
		},
	}
}

func newCopyIncludeCmd() *cobra.Command {
	var fromBranch string
	var toBranch string

	cmd := &cobra.Command{
		Use:   "copy-include [--from <branch>] [--to <branch>]",
		Short: "Copy include-defined files between worktrees",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("ft: unexpected arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService()
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
				to, err = gitx.CurrentBranch("")
				if err != nil {
					return fmt.Errorf("ft: cannot infer destination branch from detached HEAD")
				}
			}
			to, err = svc.ResolveBranchShortcut(to)
			if err != nil {
				return err
			}

			if err := svc.CopyIncludeBetweenBranches(from, to); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Copied include entries from %s to %s\n", from, to)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromBranch, "from", "", "Source branch (default: default branch)")
	cmd.Flags().StringVar(&toBranch, "to", "", "Destination branch (default: current branch)")
	_ = cmd.RegisterFlagCompletionFunc("from", completeLocalBranchesWithShortcuts)
	_ = cmd.RegisterFlagCompletionFunc("to", completeSwitchBranches)
	return cmd
}

func newSquashCmd() *cobra.Command {
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "squash [--base <branch>]",
		Short: "Squash current branch commits into one",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("ft: unexpected arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := core.NewService()
			if err != nil {
				return err
			}

			base := baseBranch
			if strings.TrimSpace(base) == "" {
				base = svc.Ctx.DefaultBranch
			}
			base, err = svc.ResolveBranchShortcut(base)
			if err != nil {
				return err
			}

			current, err := gitx.CurrentBranch("")
			if err != nil {
				return fmt.Errorf("ft: cannot squash on detached HEAD")
			}
			if current == base {
				return fmt.Errorf("ft: base branch and current branch are the same")
			}

			baseExists, err := gitx.BranchExistsLocal(svc.Ctx, base)
			if err != nil {
				return err
			}
			if !baseExists {
				return fmt.Errorf("ft: base branch not found locally: %s", base)
			}

			dirtySymbols, err := gitx.DirtySymbols(".")
			if err != nil {
				return err
			}
			if dirtySymbols != "clean" {
				return fmt.Errorf("ft: working tree must be clean before squash")
			}

			countOut, stderr, exitCode, runErr := gitx.RunGitCommon(svc.Ctx, "rev-list", "--count", base+".."+current)
			if runErr != nil {
				return runErr
			}
			if exitCode != 0 {
				if stderr == "" {
					stderr = "failed to count commits"
				}
				return fmt.Errorf("ft: %s", stderr)
			}

			var count int
			if _, err := fmt.Sscanf(strings.TrimSpace(countOut), "%d", &count); err != nil {
				return fmt.Errorf("ft: failed to parse commit count")
			}
			if count < 2 {
				return fmt.Errorf("ft: need at least 2 commits ahead of %s to squash", base)
			}

			mergeBase, stderr, exitCode, runErr := gitx.RunGitCommon(svc.Ctx, "merge-base", base, current)
			if runErr != nil {
				return runErr
			}
			if exitCode != 0 || strings.TrimSpace(mergeBase) == "" {
				if stderr == "" {
					stderr = "no merge-base found"
				}
				return fmt.Errorf("ft: %s", stderr)
			}

			logOut, _, _, runErr := gitx.RunGitCommon(svc.Ctx, "log", "--format=%s", "--reverse", base+".."+current)
			if runErr != nil {
				return runErr
			}

			tmpFile, err := os.CreateTemp("", "ft-squash-*.txt")
			if err != nil {
				return fmt.Errorf("ft: create temporary commit message file: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)

			subject := fmt.Sprintf("squash: %s (%d commits)", current, count)
			lines := []string{subject, "", "Squashed commits:"}
			for _, title := range strings.Split(logOut, "\n") {
				title = strings.TrimSpace(title)
				if title == "" {
					continue
				}
				lines = append(lines, "- "+title)
			}
			_, _ = tmpFile.WriteString(strings.Join(lines, "\n") + "\n")
			_ = tmpFile.Close()

			_, stderr, exitCode, runErr = gitx.RunGit("", "reset", "--soft", strings.TrimSpace(mergeBase))
			if runErr != nil {
				return runErr
			}
			if exitCode != 0 {
				if stderr == "" {
					stderr = "git reset failed"
				}
				return fmt.Errorf("ft: %s", stderr)
			}

			_, stderr, exitCode, runErr = gitx.RunGit("", "commit", "--file", tmpPath)
			if runErr != nil {
				return runErr
			}
			if exitCode != 0 {
				if stderr == "" {
					stderr = "git commit failed"
				}
				return fmt.Errorf("ft: %s", stderr)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Squashed %d commits on %s into one commit\n", count, current)
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: detected default branch)")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}

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
			svc, err := core.NewService()
			if err != nil {
				return err
			}

			branch := ""
			if len(args) == 1 {
				branch = args[0]
			} else {
				current, currentErr := gitx.CurrentBranch("")
				if currentErr != nil {
					return fmt.Errorf("ft: cannot infer branch from detached HEAD")
				}

				if current != svc.Ctx.DefaultBranch {
					branch = current
				} else if term.IsTerminal(int(os.Stdin.Fd())) {
					entries, err := gitx.ListWorktrees(svc.Ctx)
					if err != nil {
						return err
					}
					picked, pickErr := tui.PickRemoveBranch(entries, current, svc.Ctx)
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

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <url> [dir]",
		Short: "Clone a repo into a bare-in-.git layout with an initial worktree",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 {
				return fmt.Errorf("ft: usage: ft clone <url> [dir]")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			dir := ""
			if len(args) == 2 {
				dir = args[1]
			}

			result, err := gitx.CloneRepo(url, dir)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cloned into %s\n", result.RepoRoot)
			fmt.Fprintf(cmd.OutOrStdout(), "Default branch: %s\n", result.DefaultBranch)
			fmt.Fprintf(cmd.OutOrStdout(), "Worktree: %s\n", result.WorktreePath)
			fmt.Fprintf(cmd.OutOrStdout(), "\ncd %s/%s\n", result.RepoRoot, result.DefaultBranch)
			return nil
		},
	}
}

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
			svc, err := core.NewService()
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

func newSwitchCmd() *cobra.Command {
	var createIfMissing bool
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "switch [branch]",
		Short: "Switch to an existing worktree branch",
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
			svc, err := core.NewService()
			if err != nil {
				return err
			}

			branch := ""
			if len(args) == 1 {
				branch = args[0]
			} else {
				entries, err := gitx.ListWorktrees(svc.Ctx)
				if err != nil {
					return err
				}

				current, _ := gitx.CurrentBranch("")
				if term.IsTerminal(int(os.Stdin.Fd())) {
					picked, pickErr := tui.PickSwitchBranch(entries, current, svc.Ctx)
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

			if result.Created {
				fmt.Fprintf(cmd.OutOrStdout(), "Created worktree: %s -> %s\n", result.Branch, result.Path)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to %s (%s)\n", result.Branch, result.Path)
			shell.EmitCDOrWarning(result.Path, cmd.OutOrStdout(), cmd.ErrOrStderr())

			return nil
		},
	}

	cmd.Flags().BoolVarP(&createIfMissing, "create", "c", false, "Create worktree if branch is missing")
	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch used with --create")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh]",
		Short: "Generate shell completion scripts",
		Long:  "Generate shell completion (only) scripts for your shell.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("ft: specify a shell: bash|zsh")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			default:
				return fmt.Errorf("ft: unsupported shell %q (supported: bash, zsh)", args[0])
			}
		},
	}

	return cmd
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [bash|zsh]",
		Short: "Print shell integration snippet for auto-cd",
		Long: `Print the shell function required for automatic directory switching.

A Go binary cannot change its caller's working directory — this is an OS
constraint that applies in any language. The shell function wraps the ft
binary, reads the __FT_CD__ marker on stdout, and calls cd on your behalf.

Supported shells: bash, zsh.
Without an argument ft auto-detects from $SHELL.

Source once in your shell config:
  eval "$(ft init)"      # auto-detect from $SHELL
  eval "$(ft init zsh)"  # explicit`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("ft: expected at most one shell argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			targetShell := inferShell()
			if len(args) == 1 {
				targetShell = args[0]
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "ft init: generating integration for %q (auto-detected from $SHELL; use 'ft init bash' or 'ft init zsh' to be explicit)\n", targetShell)
			}

			script, err := shell.InitScript(targetShell)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), script)
			return nil
		},
	}

	return cmd
}

func inferShell() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return "zsh"
	}
	base := filepath.Base(s)
	if base == "bash" || base == "zsh" {
		return base
	}
	return "zsh"
}

const helpTemplate = `ft (featuretree) – lightweight git worktree helper

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
