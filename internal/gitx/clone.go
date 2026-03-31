package gitx

import (
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

// CloneRepo sets up a bare-in-.git repository from a remote URL:
//
//  1. git clone --bare <url> <dir>/.git
//  2. write the fetch refspec (bare clones omit it, breaking git fetch)
//  3. git fetch origin  (populate refs/remotes/origin/*)
//  4. git remote set-head origin --auto  (resolve origin/HEAD)
//  5. write branch tracking config so git pull works from worktrees
//  6. create the initial worktree for the default branch
func CloneRepo(url string, dir string) (*CloneResult, error) {
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

	// Step 1: bare clone.
	if _, _, exitCode, err := runCommand("", "git", "clone", "--bare", url, gitDir); err != nil || exitCode != 0 {
		_ = os.RemoveAll(absDir)
		if err != nil {
			return nil, fmt.Errorf("ft: git clone: %w", err)
		}
		return nil, fmt.Errorf("ft: git clone failed")
	}

	// Step 2: write the fetch refspec that bare clones omit.
	// Without it, git fetch never populates refs/remotes/origin/*, so remote-tracking
	// branches don't update and origin/HEAD cannot be resolved.
	if _, _, exitCode, err := runCommand("", "git", "--git-dir", gitDir, "config",
		"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil || exitCode != 0 {
		_ = os.RemoveAll(absDir)
		if err != nil {
			return nil, fmt.Errorf("ft: write fetch refspec: %w", err)
		}
		return nil, fmt.Errorf("ft: write fetch refspec failed")
	}

	// Step 3: fetch so refs/remotes/origin/* are populated, then resolve origin/HEAD.
	if _, _, _, err := runCommand("", "git", "--git-dir", gitDir, "fetch", "origin"); err == nil {
		// Resolve origin/HEAD now that remote-tracking refs exist.
		_, _, _, _ = runCommand("", "git", "--git-dir", gitDir, "remote", "set-head", "origin", "--auto")
	}

	defaultBranch, err := detectDefaultBranch(gitDir)
	if err != nil {
		_ = os.RemoveAll(absDir)
		return nil, fmt.Errorf("ft: detect default branch: %w", err)
	}

	// Step 4: write branch tracking entries.
	// git clone --bare omits these, so git pull from a worktree fails without them.
	trackingArgs := [][]string{
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".remote", "origin"},
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".merge", "refs/heads/" + defaultBranch},
	}
	for _, args := range trackingArgs {
		if _, _, exitCode, err := runCommand("", "git", args...); err != nil || exitCode != 0 {
			_ = os.RemoveAll(absDir)
			if err != nil {
				return nil, fmt.Errorf("ft: write branch config: %w", err)
			}
			return nil, fmt.Errorf("ft: write branch config failed")
		}
	}

	// Step 5: create the initial worktree for the default branch.
	worktreePath := filepath.Join(absDir, defaultBranch)
	if _, _, exitCode, err := runCommand("", "git", "--git-dir", gitDir, "worktree", "add", worktreePath, defaultBranch); err != nil || exitCode != 0 {
		_ = os.RemoveAll(absDir)
		if err != nil {
			return nil, fmt.Errorf("ft: create initial worktree: %w", err)
		}
		return nil, fmt.Errorf("ft: create initial worktree failed")
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
