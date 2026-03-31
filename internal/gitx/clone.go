package gitx

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CloneResult struct {
	RepoRoot      string
	GitCommonDir  string
	DefaultBranch string
	WorktreePath  string
}

// CloneRepo sets up a bare-in-.git repository from a remote URL and creates
// the initial worktree for the detected default branch.
func CloneRepo(commandCtx context.Context, url string, dir string) (*CloneResult, error) {
	commandCtx = normalizeCommandContext(commandCtx)

	if dir == "" {
		dir = repoNameFromURL(url)
	}
	if dir == "" {
		return nil, fmt.Errorf("ft: could not infer directory name from URL %q; pass an explicit directory", url)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("ft: resolve target directory: %w", err)
	}

	if _, err := os.Stat(absDir); err == nil {
		return nil, fmt.Errorf("ft: target directory already exists: %s", absDir)
	}

	gitDir := filepath.Join(absDir, ".git")

	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "clone", "--bare", url, gitDir)
	if err := CommandError("clone repository", stderr, exitCode, runErr, "git clone failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}

	// Bare clones omit this refspec; without it, remote-tracking refs stay stale
	// and origin/HEAD cannot be resolved reliably.
	_, stderr, exitCode, runErr = runCommand(commandCtx, "", "git", "--git-dir", gitDir, "config",
		"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err := CommandError("configure remote fetch refspec", stderr, exitCode, runErr, "git config failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}

	_, stderr, exitCode, runErr = runCommand(commandCtx, "", "git", "--git-dir", gitDir, "fetch", "origin")
	if err := CommandError("fetch origin refs", stderr, exitCode, runErr, "git fetch origin failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}

	_, stderr, exitCode, runErr = runCommand(commandCtx, "", "git", "--git-dir", gitDir, "remote", "set-head", "origin", "--auto")
	if err := CommandError("resolve origin/HEAD", stderr, exitCode, runErr, "git remote set-head origin --auto failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, fmt.Errorf("%w (ensure the remote default branch/HEAD is configured)", err)
	}

	defaultBranch, err := detectDefaultBranch(commandCtx, gitDir)
	if err != nil {
		_ = os.RemoveAll(absDir)
		return nil, fmt.Errorf("ft: detect default branch: %w", err)
	}

	// Bare clones also omit branch tracking config needed for git pull in worktrees.
	trackingArgs := [][]string{
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".remote", "origin"},
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".merge", "refs/heads/" + defaultBranch},
	}
	for _, args := range trackingArgs {
		_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", args...)
		if err := CommandError("configure default branch tracking", stderr, exitCode, runErr, "git config failed"); err != nil {
			_ = os.RemoveAll(absDir)
			return nil, err
		}
	}

	worktreePath := filepath.Join(absDir, defaultBranch)
	_, stderr, exitCode, runErr = runCommand(commandCtx, "", "git", "--git-dir", gitDir, "worktree", "add", worktreePath, defaultBranch)
	if err := CommandError("create initial worktree", stderr, exitCode, runErr, "git worktree add failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}

	return &CloneResult{
		RepoRoot:      absDir,
		GitCommonDir:  gitDir,
		DefaultBranch: defaultBranch,
		WorktreePath:  worktreePath,
	}, nil
}

// repoNameFromURL derives a directory name from a git URL, stripping the
// trailing .git suffix and taking only the last path segment.
func repoNameFromURL(url string) string {
	url = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(url), "/"), ".git")
	if idx := strings.LastIndexAny(url, "/:"); idx >= 0 {
		url = url[idx+1:]
	}
	return url
}
