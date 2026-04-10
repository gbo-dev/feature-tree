package gitx

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const maxConcurrentBranchRelations = 8

type Worktree struct {
	Path         string
	Branch       string
	LockedReason string
}

// RelativePath returns "." for current worktree and "../<name>" for siblings;
// if fromPath is empty (detached HEAD), it returns wtPath unchanged.
func RelativePath(wtPath, fromPath string) string {
	if fromPath == "" {
		return wtPath
	}
	if wtPath == fromPath {
		return "."
	}
	return "../" + filepath.Base(wtPath)
}

func ListWorktrees(commandCtx context.Context, ctx *RepoContext) ([]Worktree, error) {
	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "worktree", "list", "--porcelain")
	if err := CommandError("list worktrees", stderr, exitCode, runErr, "git worktree list failed"); err != nil {
		return nil, err
	}

	var (
		entries     []Worktree
		entryPath   string
		entryBranch string
		entryLock   string
	)

	flush := func() {
		if entryPath != "" && entryBranch != "" {
			entries = append(entries, Worktree{Path: entryPath, Branch: entryBranch, LockedReason: entryLock})
		}
		entryPath = ""
		entryBranch = ""
		entryLock = ""
	}

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			entryPath = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"):
			entryBranch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "locked":
			entryLock = "locked"
		case strings.HasPrefix(line, "locked "):
			entryLock = strings.TrimPrefix(line, "locked ")
		}
	}
	flush()

	return entries, nil
}

func CurrentBranch(commandCtx context.Context, dir string) (string, error) {
	stdout, stderr, exitCode, runErr := RunGit(commandCtx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err := CommandError("resolve current branch", stderr, exitCode, runErr, "HEAD is detached"); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func BranchExistsLocal(commandCtx context.Context, ctx *RepoContext, branch string) (bool, error) {
	_, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if exitCode == 0 {
		return true, nil
	}
	if exitCode == 1 && runErr == nil {
		return false, nil
	}
	if err := CommandError(fmt.Sprintf("verify local branch %q", branch), stderr, exitCode, runErr, "git show-ref failed"); err != nil {
		return false, err
	}
	return false, nil
}

func DirtySymbols(commandCtx context.Context, path string) (string, error) {
	stdout, stderr, exitCode, runErr := RunGit(commandCtx, path, "status", "--porcelain", "--untracked-files=normal")
	if err := CommandError("inspect dirty state", stderr, exitCode, runErr, "git status failed"); err != nil {
		return "", err
	}

	staged := false
	unstaged := false
	untracked := false

	for _, line := range strings.Split(stdout, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "?? ") {
			untracked = true
			continue
		}
		if len(line) >= 2 {
			if line[0] != ' ' {
				staged = true
			}
			if line[1] != ' ' {
				unstaged = true
			}
		}
	}

	result := ""
	if staged {
		result += "+"
	}
	if unstaged {
		result += "!"
	}
	if untracked {
		result += "?"
	}
	if result == "" {
		return "clean", nil
	}
	return result, nil
}

func DirtyState(symbols string) string {
	symbols = strings.TrimSpace(symbols)
	if symbols == "" || symbols == "clean" {
		return "clean"
	}
	return symbols
}

func BranchRelation(commandCtx context.Context, ctx *RepoContext, branch string) (string, error) {
	if branch == ctx.DefaultBranch {
		return "-", nil
	}

	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "rev-list", "--left-right", "--count", ctx.DefaultBranch+"..."+branch)
	if runErr != nil {
		return "", CommandError(fmt.Sprintf("compute branch relation for %q", branch), stderr, exitCode, runErr, "git rev-list failed")
	}
	if exitCode != 0 {
		if stderr == "" {
			return "?", nil
		}
		return "", CommandError(fmt.Sprintf("compute branch relation for %q", branch), stderr, exitCode, nil, "git rev-list failed")
	}

	parts := strings.Fields(stdout)
	if len(parts) != 2 {
		return "?", nil
	}

	behind, err := strconv.Atoi(parts[0])
	if err != nil {
		return "?", nil
	}
	ahead, err := strconv.Atoi(parts[1])
	if err != nil {
		return "?", nil
	}

	return fmt.Sprintf("A: %d B: %d", ahead, behind), nil
}

func FetchBranchRelationsParallel(commandCtx context.Context, ctx *RepoContext, branches []string) []string {
	results := make([]string, len(branches))
	if len(branches) == 0 {
		return results
	}

	limit := maxConcurrentBranchRelations
	if len(branches) < limit {
		limit = len(branches)
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, branch := range branches {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, branchName string) {
			defer wg.Done()
			defer func() { <-sem }()

			relation, err := BranchRelation(commandCtx, ctx, branchName)
			if err != nil || strings.TrimSpace(relation) == "" {
				results[idx] = "?"
				return
			}
			results[idx] = relation
		}(i, branch)
	}
	wg.Wait()
	return results
}
