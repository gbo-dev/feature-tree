package core

import (
	"context"
	"fmt"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

func (s *Service) Switch(commandCtx context.Context, branch string, createIfMissing bool, baseBranch string) (*SwitchResult, error) {
	if commandCtx == nil {
		return nil, fmt.Errorf("missing command context")
	}

	resolvedBranch, err := s.ResolveBranchShortcut(commandCtx, branch)
	if err != nil {
		return nil, err
	}

	worktrees, err := gitx.ListWorktrees(commandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	if path := FindWorktreePath(worktrees, resolvedBranch); path != "" {
		return &SwitchResult{Path: path, Branch: resolvedBranch, DidSwitch: true}, nil
	}

	if !createIfMissing {
		return nil, fmt.Errorf("branch %q has no worktree (use ft create %s or ft switch --create %s)", resolvedBranch, resolvedBranch, resolvedBranch)
	}

	result, err := s.CreateWorktree(commandCtx, resolvedBranch, baseBranch)
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
