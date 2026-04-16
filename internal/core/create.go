package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func (s *Service) CreateWorktree(branch string, baseBranch string) (*CreateResult, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, fmt.Errorf("branch name is required")
	}
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = s.Ctx.DefaultBranch
	}

	resolvedBaseBranch, err := s.ResolveBranchShortcut(baseBranch)
	if err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(s.CommandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	if existingPath := FindWorktreePath(worktrees, branch); existingPath != "" {
		return &CreateResult{Path: existingPath, Created: false, Branch: branch, FromBase: resolvedBaseBranch}, nil
	}

	branchDirName := SanitizeBranchName(branch)
	worktreePath := filepath.Join(s.Ctx.RepoRoot, branchDirName)

	for _, worktree := range worktrees {
		if worktree.Path == worktreePath && worktree.Branch != branch {
			return nil, fmt.Errorf("path collision: %q maps to %s, already used by %q", branch, worktreePath, worktree.Branch)
		}
	}

	if _, err := os.Stat(worktreePath); err == nil {
		return nil, fmt.Errorf("target path already exists: %s", worktreePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect target path: %w", err)
	}

	branchExists, err := gitx.BranchExistsLocal(s.CommandCtx, s.Ctx, branch)
	if err != nil {
		return nil, err
	}

	if branchExists {
		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "worktree", "add", worktreePath, branch)
		if err := gitx.CommandError("create worktree", stderr, exitCode, runErr, "git worktree add failed"); err != nil {
			return nil, err
		}
	} else {
		baseExists, err := gitx.BranchExistsLocal(s.CommandCtx, s.Ctx, resolvedBaseBranch)
		if err != nil {
			return nil, err
		}
		if !baseExists {
			return nil, fmt.Errorf("base branch not found locally: %s", resolvedBaseBranch)
		}

		_, stderr, exitCode, runErr := gitx.RunGitCommon(s.CommandCtx, s.Ctx, "worktree", "add", "-b", branch, worktreePath, resolvedBaseBranch)
		if err := gitx.CommandError("create worktree", stderr, exitCode, runErr, "git worktree add failed"); err != nil {
			return nil, err
		}
	}

	if branch != s.Ctx.DefaultBranch {
		if err := s.CopyIncludeBetweenBranches(s.Ctx.DefaultBranch, branch); err != nil {
			return nil, err
		}
	}

	return &CreateResult{Path: worktreePath, Created: true, Branch: branch, FromBase: resolvedBaseBranch}, nil
}
