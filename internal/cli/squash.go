package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gbo-dev/feature-tree/internal/core"
	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func newSquashCmd() *cobra.Command {
	var baseBranch string

	cmd := &cobra.Command{
		Use:   "squash [--base <branch>]",
		Short: "Squash current branch commits into one",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("unexpected arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			svc, err := core.NewService(cmd.Context())
			if err != nil {
				return err
			}

			base := baseBranch
			if strings.TrimSpace(base) == "" {
				base = svc.Ctx.DefaultBranch
			}
			base, err = svc.ResolveBranchShortcut(cmd.Context(), base)
			if err != nil {
				return err
			}

			current, err := gitx.CurrentBranch(cmd.Context(), "")
			if err != nil {
				return fmt.Errorf("cannot squash on detached HEAD")
			}
			if current == base {
				return fmt.Errorf("base branch and current branch are the same")
			}

			baseExists, err := gitx.BranchExistsLocal(cmd.Context(), svc.Ctx, base)
			if err != nil {
				return err
			}
			if !baseExists {
				return fmt.Errorf("base branch not found locally: %s", base)
			}

			dirtySymbols, err := gitx.DirtySymbols(cmd.Context(), ".")
			if err != nil {
				return err
			}
			if dirtySymbols != "clean" {
				return fmt.Errorf("working tree must be clean before squash")
			}

			countOut, stderr, exitCode, runErr := gitx.RunGitCommon(cmd.Context(), svc.Ctx, "rev-list", "--count", base+".."+current)
			countOut, err = gitx.ExpectSuccess("count commits for squash", countOut, stderr, exitCode, runErr, "failed to count commits")
			if err != nil {
				return err
			}

			var count int
			if _, err := fmt.Sscanf(strings.TrimSpace(countOut), "%d", &count); err != nil {
				return fmt.Errorf("failed to parse commit count")
			}
			if count < 2 {
				return fmt.Errorf("need at least 2 commits ahead of %s to squash", base)
			}

			mergeBase, stderr, exitCode, runErr := gitx.RunGitCommon(cmd.Context(), svc.Ctx, "merge-base", base, current)
			mergeBase, err = gitx.ExpectSuccess("find merge-base", mergeBase, stderr, exitCode, runErr, "no merge-base found")
			if err != nil {
				return err
			}
			if strings.TrimSpace(mergeBase) == "" {
				return fmt.Errorf("find merge-base: no merge-base found")
			}

			logOut, stderr, exitCode, runErr := gitx.RunGitCommon(cmd.Context(), svc.Ctx, "log", "--format=%s", "--reverse", base+".."+current)
			logOut, err = gitx.ExpectSuccess("list commits for squash", logOut, stderr, exitCode, runErr, "failed to list commits")
			if err != nil {
				return err
			}

			tmpFile, err := os.CreateTemp("", "ft-squash-*.txt")
			if err != nil {
				return fmt.Errorf("create temporary commit message file: %w", err)
			}
			tmpPath := tmpFile.Name()
			defer func() {
				if removeErr := os.Remove(tmpPath); removeErr != nil && err == nil && !os.IsNotExist(removeErr) {
					err = fmt.Errorf("remove temporary commit message file: %w", removeErr)
				}
			}()

			subject := fmt.Sprintf("squash: %s (%d commits)", current, count)
			lines := []string{subject, "", "Squashed commits:"}
			for _, title := range strings.Split(logOut, "\n") {
				title = strings.TrimSpace(title)
				if title == "" {
					continue
				}
				lines = append(lines, "- "+title)
			}
			if _, err = tmpFile.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
				_ = tmpFile.Close()
				return fmt.Errorf("write temporary commit message file: %w", err)
			}
			if err = tmpFile.Close(); err != nil {
				return fmt.Errorf("close temporary commit message file: %w", err)
			}

			_, stderr, exitCode, runErr = gitx.RunGit(cmd.Context(), "", "reset", "--soft", strings.TrimSpace(mergeBase))
			if err := gitx.CommandError("reset branch for squash", stderr, exitCode, runErr, "git reset failed"); err != nil {
				return err
			}

			_, stderr, exitCode, runErr = gitx.RunGit(cmd.Context(), "", "commit", "--file", tmpPath)
			if err := gitx.CommandError("create squashed commit", stderr, exitCode, runErr, "git commit failed"); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Squashed %d commits on %s into one commit\n", count, current); err != nil {
				return fmt.Errorf("write squash output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&baseBranch, "base", "b", "", "Base branch (default: detected default branch)")
	_ = cmd.RegisterFlagCompletionFunc("base", completeLocalBranchesWithShortcuts)
	return cmd
}
