package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gbo-dev/feature-tree/internal/gitx"
)

type Service struct {
	Ctx *gitx.RepoContext

	// prMetadataResolver is an optional test hook; nil uses gh pr view.
	prMetadataResolver func(context.Context, int) (PRMetadata, error)
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

type PRResult struct {
	Number   int
	Path     string
	Branch   string
	Created  bool
	Warnings []string
	Hints    []string
}

type RemoveResult struct {
	Branch            string
	Path              string
	FallbackPath      string
	TargetRef         string
	FetchWarning      string
	DeletedMerged     bool
	DeletedIdentical  bool
	DeletedEquivalent bool
	DeletedForced     bool
	KeptBranch        bool
	NoDeleteBranch    bool
}

func NewService(commandCtx context.Context) (*Service, error) {
	if commandCtx == nil {
		return nil, fmt.Errorf("missing command context")
	}

	repoCtx, err := gitx.DiscoverRepoContext(commandCtx)
	if err != nil {
		return nil, err
	}
	return &Service{Ctx: repoCtx}, nil
}

func (s *Service) ResolveBranchShortcut(commandCtx context.Context, input string) (string, error) {
	if commandCtx == nil {
		return "", fmt.Errorf("missing command context")
	}

	switch input {
	case "^":
		return s.Ctx.DefaultBranch, nil
	case "@":
		current, err := gitx.CurrentBranch(commandCtx, "")
		if err != nil {
			if errors.Is(err, gitx.ErrDetachedHead) {
				return "", fmt.Errorf("HEAD is detached; @ is unavailable")
			}
			return "", fmt.Errorf("resolve current branch: %w", err)
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

// WorktreeBranchPathMismatch reports when an ft-layout worktree directory no
// longer matches its checked-out branch (direct child of repoRoot only).
func WorktreeBranchPathMismatch(repoRoot string, wt gitx.Worktree) bool {
	if repoRoot == "" || wt.Path == "" || wt.Branch == "" {
		return false
	}
	if filepath.Dir(wt.Path) != repoRoot {
		return false
	}
	return SanitizeBranchName(wt.Branch) != filepath.Base(wt.Path)
}

func FindWorktreePath(worktrees []gitx.Worktree, branch string) string {
	for _, worktree := range worktrees {
		if worktree.Branch == branch {
			return worktree.Path
		}
	}
	return ""
}
