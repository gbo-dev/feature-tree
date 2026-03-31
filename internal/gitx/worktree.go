package gitx

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

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

func ListWorktrees(ctx *RepoContext) ([]Worktree, error) {
	stdout, stderr, exitCode, err := RunGitCommon(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("ft: list worktrees: %w", err)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "git worktree list failed"
		}
		return nil, fmt.Errorf("ft: %s", stderr)
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

func CurrentBranch(dir string) (string, error) {
	stdout, stderr, exitCode, err := RunGit(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("ft: resolve current branch: %w", err)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "HEAD is detached"
		}
		return "", fmt.Errorf("ft: %s", stderr)
	}
	return strings.TrimSpace(stdout), nil
}

func BranchExistsLocal(ctx *RepoContext, branch string) (bool, error) {
	_, stderr, exitCode, err := RunGitCommon(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		return false, fmt.Errorf("ft: verify local branch %q: %w", branch, err)
	}
	if exitCode == 0 {
		return true, nil
	}
	if exitCode == 1 {
		return false, nil
	}
	if stderr != "" {
		return false, fmt.Errorf("ft: %s", stderr)
	}
	return false, fmt.Errorf("ft: git show-ref failed for branch %q", branch)
}

func DirtySymbols(path string) (string, error) {
	stdout, stderr, exitCode, err := RunGit(path, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return "", fmt.Errorf("ft: inspect dirty state: %w", err)
	}
	if exitCode != 0 {
		if stderr == "" {
			stderr = "git status failed"
		}
		return "", fmt.Errorf("ft: %s", stderr)
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
	if symbols == "" || symbols == "?" {
		return "dirty"
	}
	if symbols == "clean" {
		return "clean"
	}
	return fmt.Sprintf("dirty (%s)", symbols)
}

func BranchRelation(ctx *RepoContext, branch string) (string, error) {
	if branch == ctx.DefaultBranch {
		return "-", nil
	}

	stdout, stderr, exitCode, err := RunGitCommon(ctx, "rev-list", "--left-right", "--count", ctx.DefaultBranch+"..."+branch)
	if err != nil {
		return "", fmt.Errorf("ft: compute branch relation: %w", err)
	}
	if exitCode != 0 {
		if stderr == "" {
			return "?", nil
		}
		return "", fmt.Errorf("ft: %s", stderr)
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
