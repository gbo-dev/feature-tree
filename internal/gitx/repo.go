package gitx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RepoContext struct {
	RepoRoot      string
	GitCommonDir  string
	DefaultBranch string
	IncludeFile   string
}

func DiscoverRepoContext() (*RepoContext, error) {
	commonRaw, stderr, exitCode, err := runCommand("", "git", "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("ft: not inside a git worktree: %w", err)
	}
	if exitCode != 0 || commonRaw == "" {
		if stderr == "" {
			stderr = "not inside a git worktree"
		}
		return nil, fmt.Errorf("ft: %s", stderr)
	}

	commonAbs, err := filepath.Abs(commonRaw)
	if err != nil {
		return nil, fmt.Errorf("ft: resolve git common dir: %w", err)
	}

	commonAbs, err = filepath.EvalSymlinks(commonAbs)
	if err != nil {
		return nil, fmt.Errorf("ft: resolve git common dir symlink: %w", err)
	}

	info, err := os.Stat(commonAbs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("ft: git common dir not found: %s", commonAbs)
	}

	if filepath.Base(commonAbs) != ".git" {
		return nil, fmt.Errorf("ft: expected .git common dir, found: %s", commonAbs)
	}

	repoRoot := filepath.Dir(commonAbs)

	isBare, err := gitCommon(commonAbs, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(isBare) != "true" {
		return nil, fmt.Errorf("ft: only bare-in-.git repositories are supported")
	}

	defaultBranch, err := detectDefaultBranch(commonAbs)
	if err != nil {
		return nil, err
	}

	return &RepoContext{
		RepoRoot:      repoRoot,
		GitCommonDir:  commonAbs,
		DefaultBranch: defaultBranch,
		IncludeFile:   ".worktreeinclude",
	}, nil
}

func gitCommon(gitCommonDir string, args ...string) (string, error) {
	fullArgs := append([]string{"--git-dir", gitCommonDir}, args...)
	stdout, stderr, exitCode, err := runCommand("", "git", fullArgs...)
	return expectSuccess("ft: git command failed", stdout, stderr, exitCode, err)
}

func detectDefaultBranch(gitCommonDir string) (string, error) {
	remoteHead, _, _, err := runCommand("", "git", "--git-dir", gitCommonDir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil && remoteHead != "" {
		return strings.TrimPrefix(strings.TrimSpace(remoteHead), "origin/"), nil
	}

	fallbacks := []string{"main", "master", "trunk"}
	for _, candidate := range fallbacks {
		_, _, exitCode, runErr := runCommand("", "git", "--git-dir", gitCommonDir, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate)
		if runErr != nil {
			return "", fmt.Errorf("ft: verify branch %s: %w", candidate, runErr)
		}
		if exitCode == 0 {
			return candidate, nil
		}
	}

	return "main", nil
}
