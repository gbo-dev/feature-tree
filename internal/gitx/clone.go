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
	if err := requireCommandContext(commandCtx); err != nil {
		return nil, err
	}

	absDir, gitDir, err := cloneResolvePaths(url, dir)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(absDir); err == nil {
		return nil, fmt.Errorf("target directory already exists: %s", absDir)
	}

	if err := cloneBareRepo(commandCtx, url, gitDir, absDir); err != nil {
		return nil, err
	}
	if err := cloneConfigureFetchRefspec(commandCtx, gitDir); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}
	if err := cloneFetchOriginRefs(commandCtx, gitDir); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}
	if err := cloneResolveRemoteHEAD(commandCtx, gitDir); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, fmt.Errorf("%w (ensure the remote default branch/HEAD is configured)", err)
	}

	defaultBranch, err := detectDefaultBranch(commandCtx, gitDir)
	if err != nil {
		_ = os.RemoveAll(absDir)
		return nil, fmt.Errorf("detect default branch: %w", err)
	}

	if err := cloneConfigureTracking(commandCtx, gitDir, defaultBranch); err != nil {
		_ = os.RemoveAll(absDir)
		return nil, err
	}

	worktreePath := filepath.Join(absDir, defaultBranch)
	if err := cloneCreateInitialWorktree(commandCtx, gitDir, worktreePath, defaultBranch); err != nil {
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

func cloneResolvePaths(url string, dir string) (absDir string, gitDir string, err error) {
	if dir == "" {
		dir = repoNameFromURL(url)
	}
	if dir == "" {
		return "", "", fmt.Errorf("could not infer directory name from URL %q; pass an explicit directory", url)
	}

	absDir, err = filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve target directory: %w", err)
	}
	return absDir, filepath.Join(absDir, ".git"), nil
}

func cloneBareRepo(commandCtx context.Context, url string, gitDir string, absDir string) error {
	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "clone", "--bare", url, gitDir)
	if err := CommandError("clone repository", stderr, exitCode, runErr, "git clone failed"); err != nil {
		_ = os.RemoveAll(absDir)
		return err
	}
	return nil
}

func cloneConfigureFetchRefspec(commandCtx context.Context, gitDir string) error {
	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitDir, "config",
		"remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err := CommandError("configure remote fetch refspec", stderr, exitCode, runErr, "git config failed"); err != nil {
		return err
	}
	return nil
}

func cloneFetchOriginRefs(commandCtx context.Context, gitDir string) error {
	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitDir, "fetch", "origin")
	if err := CommandError("fetch origin refs", stderr, exitCode, runErr, "git fetch origin failed"); err != nil {
		return err
	}
	return nil
}

func cloneResolveRemoteHEAD(commandCtx context.Context, gitDir string) error {
	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitDir, "remote", "set-head", "origin", "--auto")
	if err := CommandError("resolve origin/HEAD", stderr, exitCode, runErr, "git remote set-head origin --auto failed"); err != nil {
		return err
	}
	return nil
}

func cloneConfigureTracking(commandCtx context.Context, gitDir string, defaultBranch string) error {
	trackingArgs := [][]string{
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".remote", "origin"},
		{"--git-dir", gitDir, "config", "branch." + defaultBranch + ".merge", "refs/heads/" + defaultBranch},
	}
	for _, args := range trackingArgs {
		_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", args...)
		if err := CommandError("configure default branch tracking", stderr, exitCode, runErr, "git config failed"); err != nil {
			return err
		}
	}
	return nil
}

func cloneCreateInitialWorktree(commandCtx context.Context, gitDir string, worktreePath string, defaultBranch string) error {
	_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitDir, "worktree", "add", worktreePath, defaultBranch)
	if err := CommandError("create initial worktree", stderr, exitCode, runErr, "git worktree add failed"); err != nil {
		return err
	}
	return nil
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
