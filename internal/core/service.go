package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type Service struct {
	Ctx        *gitx.RepoContext
	CommandCtx context.Context
}

type CreateResult struct {
	Path     string
	Created  bool
	Branch   string
	FromBase string
}

type SwitchResult struct {
	Path      string
	Branch    string
	Created   bool
	FromBase  string
	DidSwitch bool
}

type RemoveResult struct {
	Branch            string
	Path              string
	FallbackPath      string
	TargetRef         string
	DeletedMerged     bool
	DeletedIdentical  bool
	DeletedEquivalent bool
	DeletedForced     bool
	KeptBranch        bool
	NoDeleteBranch    bool
}

func NewService(commandCtx context.Context) (*Service, error) {
	repoCtx, err := gitx.DiscoverRepoContext(commandCtx)
	if err != nil {
		return nil, err
	}
	if commandCtx == nil {
		commandCtx = context.Background()
	}
	return &Service{Ctx: repoCtx, CommandCtx: commandCtx}, nil
}

func (s *Service) ResolveBranchShortcut(input string) (string, error) {
	switch input {
	case "^":
		return s.Ctx.DefaultBranch, nil
	case "@":
		current, err := gitx.CurrentBranch(s.CommandCtx, "")
		if err != nil {
			return "", fmt.Errorf("ft: HEAD is detached; @ is unavailable")
		}
		return current, nil
	default:
		return input, nil
	}
}

func SanitizeBranchName(branch string) string {
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = strings.ReplaceAll(branch, "\\", "-")
	return branch
}

func FindWorktreePath(worktrees []gitx.Worktree, branch string) string {
	for _, worktree := range worktrees {
		if worktree.Branch == branch {
			return worktree.Path
		}
	}
	return ""
}
