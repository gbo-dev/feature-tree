package gitx

import (
	"context"
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

func DiscoverRepoContext(commandCtx context.Context) (*RepoContext, error) {
	if err := requireCommandContext(commandCtx); err != nil {
		return nil, err
	}

	commonRaw, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "rev-parse", "--git-common-dir")
	commonRaw, err := ExpectSuccess("discover git common dir", commonRaw, stderr, exitCode, runErr, "not inside a git worktree")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(commonRaw) == "" {
		return nil, fmt.Errorf("discover git common dir: empty output")
	}

	commonAbs, err := filepath.Abs(commonRaw)
	if err != nil {
		return nil, fmt.Errorf("resolve git common dir: %w", err)
	}

	commonAbs, err = filepath.EvalSymlinks(commonAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve git common dir symlink: %w", err)
	}

	info, err := os.Stat(commonAbs)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("git common dir not found: %s", commonAbs)
	}

	if filepath.Base(commonAbs) != ".git" {
		return nil, fmt.Errorf("expected .git common dir, found: %s", commonAbs)
	}

	repoRoot := filepath.Dir(commonAbs)

	isBare, err := gitCommon(commandCtx, commonAbs, "rev-parse", "--is-bare-repository")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(isBare) != "true" {
		return nil, fmt.Errorf("only bare-in-.git repositories are supported")
	}

	defaultBranch, err := detectDefaultBranch(commandCtx, commonAbs)
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

func gitCommon(commandCtx context.Context, gitCommonDir string, args ...string) (string, error) {
	fullArgs := append([]string{"--git-dir", gitCommonDir}, args...)
	stdout, stderr, exitCode, err := runCommand(commandCtx, "", "git", fullArgs...)
	return ExpectSuccess("git command failed", stdout, stderr, exitCode, err, "git command failed")
}

func detectDefaultBranch(commandCtx context.Context, gitCommonDir string) (string, error) {
	remoteHead, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitCommonDir, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if runErr != nil {
		return "", CommandError("resolve default branch via origin/HEAD", stderr, exitCode, runErr, "git symbolic-ref failed")
	}
	if exitCode == 0 && remoteHead != "" {
		return strings.TrimPrefix(strings.TrimSpace(remoteHead), "origin/"), nil
	}
	if exitCode != 0 && exitCode != 1 {
		return "", CommandError("resolve default branch via origin/HEAD", stderr, exitCode, nil, "git symbolic-ref failed")
	}

	fallbacks := []string{"main", "master", "trunk"}
	for _, candidate := range fallbacks {
		_, stderr, exitCode, runErr := runCommand(commandCtx, "", "git", "--git-dir", gitCommonDir, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate)
		if exitCode == 0 {
			return candidate, nil
		}
		if exitCode == 1 && runErr == nil {
			continue
		}
		if err := CommandError(fmt.Sprintf("verify fallback branch %q", candidate), stderr, exitCode, runErr, "git show-ref failed"); err != nil {
			return "", err
		}
	}

	return "", fmt.Errorf("could not determine default branch: origin/HEAD is unset and none of main, master, trunk exist locally; run 'git --git-dir=%s remote set-head origin --auto' and retry", gitCommonDir)
}
