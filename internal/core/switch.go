package core

import (
	"fmt"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func (s *Service) Switch(branch string, createIfMissing bool, baseBranch string) (*SwitchResult, error) {
	resolvedBranch, err := s.ResolveBranchShortcut(branch)
	if err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(s.CommandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	if path := FindWorktreePath(worktrees, resolvedBranch); path != "" {
		return &SwitchResult{Path: path, Branch: resolvedBranch, DidSwitch: true}, nil
	}

	if !createIfMissing {
		return nil, fmt.Errorf("ft: branch %q has no worktree (use ft create %s or ft switch --create %s)", resolvedBranch, resolvedBranch, resolvedBranch)
	}

	result, err := s.CreateWorktree(resolvedBranch, baseBranch)
	if err != nil {
		return nil, err
	}

	return &SwitchResult{
		Path:      result.Path,
		Branch:    result.Branch,
		Created:   result.Created,
		FromBase:  result.FromBase,
		DidSwitch: true,
	}, nil
}
