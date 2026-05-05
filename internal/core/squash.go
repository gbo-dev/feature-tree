package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type SquashResult struct {
	Count  int
	Branch string
}

func (s *Service) SquashBranch(commandCtx context.Context, baseBranch string) (*SquashResult, error) {
	if commandCtx == nil {
		return nil, fmt.Errorf("missing command context")
	}

	base := baseBranch
	if strings.TrimSpace(base) == "" {
		base = s.Ctx.DefaultBranch
	}
	base, err := s.ResolveBranchShortcut(commandCtx, base)
	if err != nil {
		return nil, err
	}

	current, err := gitx.CurrentBranch(commandCtx, "")
	if err != nil {
		return nil, fmt.Errorf("cannot squash on detached HEAD")
	}
	if current == base {
		return nil, fmt.Errorf("base branch and current branch are the same")
	}

	baseExists, err := gitx.BranchExistsLocal(commandCtx, s.Ctx, base)
	if err != nil {
		return nil, err
	}
	if !baseExists {
		return nil, fmt.Errorf("base branch not found locally: %s", base)
	}

	dirtySymbols, err := gitx.DirtySymbols(commandCtx, ".")
	if err != nil {
		return nil, err
	}
	if dirtySymbols != "clean" {
		return nil, fmt.Errorf("working tree must be clean before squash")
	}

	countOut, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "rev-list", "--count", base+".."+current)
	countOut, err = gitx.ExpectSuccess("count commits for squash", countOut, stderr, exitCode, runErr, "failed to count commits")
	if err != nil {
		return nil, err
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(countOut), "%d", &count); err != nil {
		return nil, fmt.Errorf("failed to parse commit count")
	}
	if count < 2 {
		return nil, fmt.Errorf("need at least 2 commits ahead of %s to squash", base)
	}

	mergeBase, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "merge-base", base, current)
	mergeBase, err = gitx.ExpectSuccess("find merge-base", mergeBase, stderr, exitCode, runErr, "no merge-base found")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(mergeBase) == "" {
		return nil, fmt.Errorf("find merge-base: no merge-base found")
	}

	logOut, stderr, exitCode, runErr := gitx.RunGitCommon(commandCtx, s.Ctx, "log", "--format=%s", "--reverse", base+".."+current)
	logOut, err = gitx.ExpectSuccess("list commits for squash", logOut, stderr, exitCode, runErr, "failed to list commits")
	if err != nil {
		return nil, err
	}

	tmpFile, err := os.CreateTemp("", "ft-squash-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temporary commit message file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, fmt.Errorf("remove temporary commit message file: %w", removeErr))
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
		return nil, fmt.Errorf("write temporary commit message file: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temporary commit message file: %w", err)
	}

	_, stderr, exitCode, runErr = gitx.RunGit(commandCtx, "", "reset", "--soft", strings.TrimSpace(mergeBase))
	if err := gitx.CommandError("reset branch for squash", stderr, exitCode, runErr, "git reset failed"); err != nil {
		return nil, err
	}

	_, stderr, exitCode, runErr = gitx.RunGit(commandCtx, "", "commit", "--file", tmpPath)
	if err := gitx.CommandError("create squashed commit", stderr, exitCode, runErr, "git commit failed"); err != nil {
		return nil, err
	}

	return &SquashResult{Count: count, Branch: current}, nil
}
