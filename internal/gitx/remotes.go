package gitx

import (
	"context"
	"fmt"
	"strings"
)

func RemoteURL(commandCtx context.Context, ctx *RepoContext, remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", fmt.Errorf("remote name is required")
	}
	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "remote", "get-url", remote)
	if err := CommandError(fmt.Sprintf("read remote URL for %q", remote), stderr, exitCode, runErr, "git remote get-url failed"); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout), nil
}

func FindRemoteByURL(commandCtx context.Context, ctx *RepoContext, wantURL string) (string, error) {
	wantURL = strings.TrimSpace(wantURL)
	if wantURL == "" {
		return "", nil
	}

	stdout, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "remote")
	if err := CommandError("list remotes", stderr, exitCode, runErr, "git remote failed"); err != nil {
		return "", err
	}

	for _, remote := range strings.Fields(stdout) {
		url, err := RemoteURL(commandCtx, ctx, remote)
		if err != nil {
			continue
		}
		if normalizeRemoteURL(url) == normalizeRemoteURL(wantURL) {
			return remote, nil
		}
	}
	return "", nil
}

func normalizeRemoteURL(url string) string {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	return url
}

func AddRemote(commandCtx context.Context, ctx *RepoContext, name string, url string) error {
	name = strings.TrimSpace(name)
	url = strings.TrimSpace(url)
	if name == "" || url == "" {
		return fmt.Errorf("remote name and URL are required")
	}

	_, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "remote", "add", name, url)
	if err := CommandError(fmt.Sprintf("add remote %q", name), stderr, exitCode, runErr, "git remote add failed"); err != nil {
		return err
	}
	return nil
}

func RemoteBranchExists(commandCtx context.Context, ctx *RepoContext, remote string, branch string) (bool, error) {
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return false, nil
	}

	_, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "show-ref", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch)
	if exitCode == 0 && runErr == nil {
		return true, nil
	}
	if exitCode == 1 && runErr == nil {
		return false, nil
	}
	if err := CommandError(fmt.Sprintf("verify remote branch %q/%q", remote, branch), stderr, exitCode, runErr, "git show-ref failed"); err != nil {
		return false, err
	}
	return false, nil
}

func FetchRemoteBranch(commandCtx context.Context, ctx *RepoContext, remote string, branch string) error {
	remote = strings.TrimSpace(remote)
	branch = strings.TrimSpace(branch)
	if remote == "" || branch == "" {
		return fmt.Errorf("remote and branch are required")
	}

	refspec := fmt.Sprintf("+refs/heads/%s:refs/remotes/%s/%s", branch, remote, branch)
	_, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "fetch", remote, refspec)
	if err := CommandError(fmt.Sprintf("fetch %s/%s", remote, branch), stderr, exitCode, runErr, "git fetch failed"); err != nil {
		return err
	}
	return nil
}

func SetBranchUpstream(commandCtx context.Context, ctx *RepoContext, localBranch string, upstream string) error {
	localBranch = strings.TrimSpace(localBranch)
	upstream = strings.TrimSpace(upstream)
	if localBranch == "" || upstream == "" {
		return fmt.Errorf("local branch and upstream are required")
	}

	_, stderr, exitCode, runErr := RunGitCommon(commandCtx, ctx, "branch", "--set-upstream-to", upstream, localBranch)
	if err := CommandError(fmt.Sprintf("set upstream for branch %q", localBranch), stderr, exitCode, runErr, "git branch failed"); err != nil {
		return err
	}
	return nil
}
