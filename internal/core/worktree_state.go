package core

import (
	"context"
	"fmt"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type WorktreeState struct {
	Entries       []gitx.Worktree
	CurrentBranch string
}

func (s *Service) WorktreeState(commandCtx context.Context) (*WorktreeState, error) {
	if commandCtx == nil {
		return nil, fmt.Errorf("missing command context")
	}

	entries, err := gitx.ListWorktrees(commandCtx, s.Ctx)
	if err != nil {
		return nil, err
	}

	current, err := gitx.CurrentBranch(commandCtx, "")
	if err != nil {
		return nil, err
	}

	return &WorktreeState{
		Entries:       entries,
		CurrentBranch: current,
	}, nil
}

func (s *Service) CurrentBranch(commandCtx context.Context) (string, error) {
	if commandCtx == nil {
		return "", fmt.Errorf("missing command context")
	}
	return gitx.CurrentBranch(commandCtx, "")
}
